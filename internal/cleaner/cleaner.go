package cleaner

import (
	"fmt"
	"os"
	"time"

	"github.com/fenilsonani/system-cleanup/internal/config"
	"github.com/fenilsonani/system-cleanup/internal/progress"
	"github.com/fenilsonani/system-cleanup/internal/scanner"
)

// CleanResult represents the result of a clean operation
type CleanResult struct {
	DeletedFiles  []string
	DeletedSize   int64
	SkippedFiles  []string
	SkippedReason map[string]string
	Errors        []*DeletionError
	DryRun        bool
	UsedSudo      bool
	SudoSucceeded int
	SudoFailed    int
}

// Cleaner handles file deletion with safeguards
type Cleaner struct {
	config            *config.Config
	permissionManager *PermissionManager
	sudoManager       *SudoManager
	manifest          *DeletionManifest
	askSudo           bool // Whether to prompt for sudo if needed
	progressReporter  *progress.ProgressReporter
}

// New creates a new Cleaner
func New(cfg *config.Config) *Cleaner {
	return &Cleaner{
		config:            cfg,
		permissionManager: NewPermissionManager(),
		sudoManager:       NewSudoManager(),
		manifest:          NewDeletionManifest(),
		askSudo:           true, // Default to asking for sudo
		progressReporter:  progress.NewProgressReporter(),
	}
}

// SetAskSudo sets whether to prompt for sudo
func (c *Cleaner) SetAskSudo(ask bool) {
	c.askSudo = ask
}

// SetProgressReporter sets a custom progress reporter
func (c *Cleaner) SetProgressReporter(pr *progress.ProgressReporter) {
	c.progressReporter = pr
}

// GetProgressReporter returns the cleaner's progress reporter
func (c *Cleaner) GetProgressReporter() *progress.ProgressReporter {
	return c.progressReporter
}

// Clean performs the cleanup operation with smart sudo handling
func (c *Cleaner) Clean(scanResult *scanner.ScanResult) (cleanResult *CleanResult, cleanErr error) {
	result := &CleanResult{
		DeletedFiles:  []string{},
		SkippedFiles:  []string{},
		SkippedReason: make(map[string]string),
		Errors:        []*DeletionError{},
		DryRun:        c.config.DryRun,
	}

	// SECURITY: Ensure sudo password is ALWAYS cleared, even on panic
	sudoWasUsed := false
	defer func() {
		if sudoWasUsed {
			c.sudoManager.Clear()
		}
		// Recover from panics and ensure password is cleared
		if r := recover(); r != nil {
			cleanErr = fmt.Errorf("cleanup panicked: %v", r)
		}
	}()

	// If dry-run, just simulate
	if c.config.DryRun {
		for _, file := range scanResult.Files {
			result.DeletedFiles = append(result.DeletedFiles, file.Path)
			result.DeletedSize += file.Size
		}
		return result, nil
	}

	// Pre-flight: Analyze permissions
	startTime := time.Now()
	totalFiles := len(scanResult.Files)
	totalSize := scanResult.TotalSize

	filePaths := make([]string, len(scanResult.Files))
	fileMap := make(map[string]scanner.FileInfo)
	for i, file := range scanResult.Files {
		filePaths[i] = file.Path
		fileMap[file.Path] = file
	}

	permReport := c.permissionManager.AnalyzePermissions(filePaths, func(path string) int64 {
		if file, ok := fileMap[path]; ok {
			return file.Size
		}
		return 0
	})

	// Report start of cleanup
	c.reportCleanProgress(progress.PhaseCleaning, "", 0, totalFiles, 0, totalSize, false, startTime)

	// First, delete files that don't need sudo
	for _, path := range permReport.NormalFiles {
		file := fileMap[path]

		// Report current file
		c.reportCleanProgress(progress.PhaseCleaning, file.Path, len(result.DeletedFiles), totalFiles, result.DeletedSize, totalSize, false, startTime)

		if err := c.deleteFileNormalWithRetry(file, result); err != nil {
			result.Errors = append(result.Errors, err)
		}
	}

	// Handle files requiring sudo
	if len(permReport.RequiresSudo) > 0 {
		if c.askSudo && c.sudoManager.IsAvailable() {
			// Ask user for sudo password
			if err := c.sudoManager.PromptForPassword(); err != nil {
				// User declined or password wrong, skip sudo files
				for _, path := range permReport.RequiresSudo {
					result.SkippedFiles = append(result.SkippedFiles, path)
					result.SkippedReason[path] = "Requires elevated permissions (sudo declined)"
				}
			} else {
				// Mark that sudo is being used (for cleanup in defer)
				sudoWasUsed = true

				// Start keep-alive goroutine to maintain sudo session
				stopKeepAlive := make(chan struct{})
				defer close(stopKeepAlive)

				go func() {
					ticker := time.NewTicker(4 * time.Minute)
					defer ticker.Stop()
					for {
						select {
						case <-ticker.C:
							// Refresh sudo session
							if err := c.sudoManager.KeepAlive(); err != nil {
								// Session expired, can't continue
								return
							}
						case <-stopKeepAlive:
							return
						}
					}
				}()

				// Delete files with sudo using batch operations for better performance
				result.UsedSudo = true

				// Use batch deletion (100 files per sudo command)
				succeeded, failed := c.sudoManager.DeleteFiles(permReport.RequiresSudo)

				// Update results and manifest
				for _, path := range succeeded {
					file := fileMap[path]

					// Add to manifest
					c.manifest.Add(file.Path, file.Size, file.Category)

					result.DeletedFiles = append(result.DeletedFiles, file.Path)
					result.DeletedSize += file.Size
					result.SudoSucceeded++

					// Report progress
					c.reportCleanProgress(progress.PhaseCleaning, file.Path, len(result.DeletedFiles), totalFiles, result.DeletedSize, totalSize, true, startTime)
				}

				for path, err := range failed {
					delErr := CategorizeError(path, err)
					result.Errors = append(result.Errors, delErr)
					result.SkippedFiles = append(result.SkippedFiles, path)
					result.SkippedReason[path] = delErr.UserMessage()
					result.SudoFailed++
				}
			}
		} else {
			// Sudo not available or not asking, skip these files
			for _, path := range permReport.RequiresSudo {
				result.SkippedFiles = append(result.SkippedFiles, path)
				result.SkippedReason[path] = "Requires elevated permissions"
			}
		}
	}

	// Handle inaccessible files
	for path, err := range permReport.InaccessibleFiles {
		result.SkippedFiles = append(result.SkippedFiles, path)
		result.SkippedReason[path] = fmt.Sprintf("Inaccessible: %v", err)
	}

	// Report completion
	c.reportCleanProgress(progress.PhaseComplete, "", len(result.DeletedFiles), totalFiles, result.DeletedSize, totalSize, result.UsedSudo, startTime)

	return result, nil
}

// deleteFileNormalWithRetry attempts to delete a file with retries for transient errors
func (c *Cleaner) deleteFileNormalWithRetry(file scanner.FileInfo, result *CleanResult) *DeletionError {
	const maxRetries = 3
	retryDelays := []time.Duration{
		100 * time.Millisecond,
		500 * time.Millisecond,
		2 * time.Second,
	}

	var lastErr *DeletionError

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Try to delete (result is only modified on success)
		lastErr = c.deleteFileNormal(file, result)

		if lastErr == nil {
			// Success
			return nil
		}

		if !lastErr.Retryable {
			// Not retryable (permission denied, path invalid, etc.), give up
			return lastErr
		}

		// File is in use or temporarily unavailable
		// On last attempt, don't sleep
		if attempt < maxRetries-1 {
			time.Sleep(retryDelays[attempt])
			// Clear any partial state from failed attempt
			// (deleteFileNormal only adds to result.SkippedFiles on error, which is ok)
		}
	}

	// All retries exhausted - return last error
	return lastErr
}

// deleteFileNormal deletes a file with normal permissions
func (c *Cleaner) deleteFileNormal(file scanner.FileInfo, result *CleanResult) *DeletionError {
	// Safety check: verify it's safe to delete (not a special file)
	if err := IsSafeToDelete(file.Path); err != nil {
		result.SkippedFiles = append(result.SkippedFiles, file.Path)
		result.SkippedReason[file.Path] = fmt.Sprintf("Safety check failed: %v", err)
		return &DeletionError{
			Path:     file.Path,
			Reason:   ErrorInvalidPath,
			Original: err,
		}
	}

	// Safety check: verify file age again
	// Use Lstat to not follow symlinks (prevents TOCTOU attacks)
	info, err := os.Lstat(file.Path)
	if err != nil {
		if os.IsNotExist(err) {
			// File already deleted, that's fine
			return nil
		}
		return CategorizeError(file.Path, err)
	}

	// SECURITY: Ensure it's not a symlink (prevents following symlinks to delete unintended targets)
	if info.Mode()&os.ModeSymlink != 0 {
		result.SkippedFiles = append(result.SkippedFiles, file.Path)
		result.SkippedReason[file.Path] = "File changed to symlink (security check)"
		return &DeletionError{
			Path:     file.Path,
			Reason:   ErrorInvalidPath,
			Original: fmt.Errorf("path is a symlink"),
		}
	}

	minAge := time.Duration(c.config.MinFileAge) * time.Hour
	if time.Since(info.ModTime()) < minAge {
		result.SkippedFiles = append(result.SkippedFiles, file.Path)
		result.SkippedReason[file.Path] = "File too new (safety check)"
		return nil
	}

	// Add to manifest before deleting
	c.manifest.Add(file.Path, file.Size, file.Category)

	// Attempt deletion - use RemoveAll for directories (e.g., node_modules, venv)
	var deleteErr error
	if info.IsDir() {
		deleteErr = os.RemoveAll(file.Path)
	} else {
		deleteErr = os.Remove(file.Path)
	}
	if deleteErr != nil {
		delErr := CategorizeError(file.Path, deleteErr)
		result.SkippedFiles = append(result.SkippedFiles, file.Path)
		result.SkippedReason[file.Path] = delErr.UserMessage()
		return delErr
	}

	result.DeletedFiles = append(result.DeletedFiles, file.Path)
	result.DeletedSize += file.Size

	return nil
}

// deleteFileSudo deletes a file using sudo
func (c *Cleaner) deleteFileSudo(file scanner.FileInfo, result *CleanResult) *DeletionError {
	// Safety check: verify it's safe to delete (not a special file)
	if err := IsSafeToDelete(file.Path); err != nil {
		result.SkippedFiles = append(result.SkippedFiles, file.Path)
		result.SkippedReason[file.Path] = fmt.Sprintf("Safety check failed: %v", err)
		return &DeletionError{
			Path:     file.Path,
			Reason:   ErrorInvalidPath,
			Original: err,
		}
	}

	// Safety check: verify file age
	// Use Lstat to not follow symlinks (prevents TOCTOU attacks)
	info, err := os.Lstat(file.Path)
	if err != nil {
		if os.IsNotExist(err) {
			// File already deleted
			return nil
		}
		return CategorizeError(file.Path, err)
	}

	// SECURITY: Ensure it's still a regular file, not a symlink
	if info.Mode()&os.ModeSymlink != 0 {
		result.SkippedFiles = append(result.SkippedFiles, file.Path)
		result.SkippedReason[file.Path] = "File changed to symlink (security check)"
		return &DeletionError{
			Path:     file.Path,
			Reason:   ErrorInvalidPath,
			Original: fmt.Errorf("path is a symlink"),
		}
	}

	minAge := time.Duration(c.config.MinFileAge) * time.Hour
	if time.Since(info.ModTime()) < minAge {
		result.SkippedFiles = append(result.SkippedFiles, file.Path)
		result.SkippedReason[file.Path] = "File too new (safety check)"
		return nil
	}

	// Add to manifest
	c.manifest.Add(file.Path, file.Size, file.Category)

	// Delete with sudo
	if err := c.sudoManager.DeleteFile(file.Path); err != nil {
		delErr := CategorizeError(file.Path, err)
		result.SkippedFiles = append(result.SkippedFiles, file.Path)
		result.SkippedReason[file.Path] = delErr.UserMessage()
		return delErr
	}

	result.DeletedFiles = append(result.DeletedFiles, file.Path)
	result.DeletedSize += file.Size

	return nil
}

// CleanCategory cleans files from a specific category
func (c *Cleaner) CleanCategory(scanResult *scanner.ScanResult, category string) (*CleanResult, error) {
	// Filter files by category
	filteredResult := &scanner.ScanResult{
		Files:  []scanner.FileInfo{},
		Errors: []error{},
	}

	for _, file := range scanResult.Files {
		if file.Category == category {
			filteredResult.Files = append(filteredResult.Files, file)
			filteredResult.TotalSize += file.Size
			filteredResult.TotalCount++
		}
	}

	return c.Clean(filteredResult)
}

// GetManifest returns the deletion manifest
func (c *Cleaner) GetManifest() *DeletionManifest {
	return c.manifest
}

// SaveManifest saves the deletion manifest to a file
func (c *Cleaner) SaveManifest(path string) error {
	return c.manifest.Save(path)
}

// GetPermissionReport analyzes files and returns a permission report
func (c *Cleaner) GetPermissionReport(scanResult *scanner.ScanResult) *PermissionReport {
	filePaths := make([]string, len(scanResult.Files))
	fileMap := make(map[string]scanner.FileInfo)
	for i, file := range scanResult.Files {
		filePaths[i] = file.Path
		fileMap[file.Path] = file
	}

	return c.permissionManager.AnalyzePermissions(filePaths, func(path string) int64 {
		if file, ok := fileMap[path]; ok {
			return file.Size
		}
		return 0
	})
}

// reportCleanProgress reports clean progress to listeners
func (c *Cleaner) reportCleanProgress(phase progress.Phase, currentFile string, deletedFiles, totalFiles int, deletedSize, totalSize int64, usingSudo bool, startTime time.Time) {
	if c.progressReporter == nil {
		return
	}

	c.progressReporter.UpdateCleanProgress(&progress.CleanProgress{
		Phase:        phase,
		CurrentFile:  currentFile,
		DeletedFiles: deletedFiles,
		TotalFiles:   totalFiles,
		DeletedSize:  deletedSize,
		TotalSize:    totalSize,
		UsingSudo:    usingSudo,
		StartTime:    startTime,
	})
}

// DeletionManifest keeps track of deleted files
type DeletionManifest struct {
	Files     []DeletedFileInfo
	Timestamp time.Time
	TotalSize int64
}

// DeletedFileInfo represents information about a deleted file
type DeletedFileInfo struct {
	Path      string
	Size      int64
	Category  string
	DeletedAt time.Time
}

// NewDeletionManifest creates a new DeletionManifest
func NewDeletionManifest() *DeletionManifest {
	return &DeletionManifest{
		Files:     []DeletedFileInfo{},
		Timestamp: time.Now(),
	}
}

// Add adds a file to the manifest
func (m *DeletionManifest) Add(path string, size int64, category string) {
	m.Files = append(m.Files, DeletedFileInfo{
		Path:      path,
		Size:      size,
		Category:  category,
		DeletedAt: time.Now(),
	})
	m.TotalSize += size
}

// Save saves the manifest to a file
func (m *DeletionManifest) Save(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "Deletion Manifest\n")
	fmt.Fprintf(file, "Created: %s\n", m.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(file, "Total Size: %d bytes\n", m.TotalSize)
	fmt.Fprintf(file, "Total Files: %d\n\n", len(m.Files))

	for _, f := range m.Files {
		fmt.Fprintf(file, "%s | %d bytes | %s | %s\n",
			f.Path, f.Size, f.Category, f.DeletedAt.Format(time.RFC3339))
	}

	return nil
}
