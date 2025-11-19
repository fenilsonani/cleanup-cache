package scanner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fenilsonani/cleanup-cache/internal/config"
	"github.com/fenilsonani/cleanup-cache/internal/platform"
	"github.com/fenilsonani/cleanup-cache/internal/progress"
	"github.com/fenilsonani/cleanup-cache/internal/security"
)

// FileInfo represents information about a file found during scanning
type FileInfo struct {
	Path         string
	Size         int64
	ModTime      time.Time
	Category     string
	Reason       string // Why this file was flagged for cleanup
	Hash         string // For duplicate detection
}

// ScanResult represents the result of a scan operation
type ScanResult struct {
	Files      []FileInfo
	TotalSize  int64
	TotalCount int
	Category   string
	Errors     []error
}

// Scanner is the main scanner that coordinates all scan operations
type Scanner struct {
	config           *config.Config
	platformInfo     *platform.Info
	pathValidator    *security.PathValidator
	progressReporter *progress.ProgressReporter
}

// New creates a new Scanner
func New(cfg *config.Config, platformInfo *platform.Info) *Scanner {
	return &Scanner{
		config:           cfg,
		platformInfo:     platformInfo,
		pathValidator:    security.NewPathValidator(),
		progressReporter: progress.NewProgressReporter(),
	}
}

// SetProgressReporter sets a custom progress reporter
func (s *Scanner) SetProgressReporter(pr *progress.ProgressReporter) {
	s.progressReporter = pr
}

// GetProgressReporter returns the scanner's progress reporter
func (s *Scanner) GetProgressReporter() *progress.ProgressReporter {
	return s.progressReporter
}

// ScanAll scans all enabled categories in parallel for better performance
func (s *Scanner) ScanAll() (*ScanResult, error) {
	result := &ScanResult{
		Files:  []FileInfo{},
		Errors: []error{},
	}

	startTime := time.Now()

	// Count enabled categories
	categoriesTotal := 0
	if s.config.Categories.Cache {
		categoriesTotal++
	}
	if s.config.Categories.Temp {
		categoriesTotal++
	}
	if s.config.Categories.Logs {
		categoriesTotal++
	}
	if s.config.Categories.Duplicates {
		categoriesTotal++
	}
	if s.config.Categories.Downloads {
		categoriesTotal++
	}
	if s.config.Categories.PackageManagers {
		categoriesTotal++
	}

	categoriesDone := 0

	// Channel to collect scan results
	resultChan := make(chan *ScanResult, 6) // Max 6 categories
	var wg sync.WaitGroup
	var mu sync.Mutex // Protect result and categoriesDone

	// Helper to report and run a category scan
	runCategoryScan := func(category string, scanFunc func() *ScanResult) {
		defer wg.Done()

		// Report starting this category
		mu.Lock()
		s.reportScanProgress(progress.PhaseScanning, category, "", result.TotalCount, result.TotalSize, categoriesTotal, categoriesDone, startTime)
		mu.Unlock()

		// Run the scan
		scanResult := scanFunc()

		// Send result and update progress
		resultChan <- scanResult

		mu.Lock()
		categoriesDone++
		s.reportScanProgress(progress.PhaseScanning, category, "", result.TotalCount, result.TotalSize, categoriesTotal, categoriesDone, startTime)
		mu.Unlock()
	}

	// Scan categories in parallel
	if s.config.Categories.Cache {
		wg.Add(1)
		go runCategoryScan("cache", s.ScanCache)
	}

	if s.config.Categories.Temp {
		wg.Add(1)
		go runCategoryScan("temp", s.ScanTemp)
	}

	if s.config.Categories.Logs {
		wg.Add(1)
		go runCategoryScan("logs", s.ScanLogs)
	}

	if s.config.Categories.Duplicates {
		wg.Add(1)
		go runCategoryScan("duplicates", s.ScanDuplicates)
	}

	if s.config.Categories.Downloads {
		wg.Add(1)
		go runCategoryScan("downloads", s.ScanDownloads)
	}

	if s.config.Categories.PackageManagers {
		wg.Add(1)
		go runCategoryScan("package_managers", s.ScanPackageManagers)
	}

	// Close channel when all scans complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for scanResult := range resultChan {
		mu.Lock()
		result.merge(scanResult)
		mu.Unlock()
	}

	// Report completion
	s.reportScanProgress(progress.PhaseComplete, "", "", result.TotalCount, result.TotalSize, categoriesTotal, categoriesDone, startTime)

	return result, nil
}

// ScanAllStreaming scans all enabled categories with streaming for memory efficiency
// This prevents OOM issues when scanning millions of files
func (s *Scanner) ScanAllStreaming(ctx context.Context) <-chan *ScanBatch {
	batchChan := make(chan *ScanBatch, ChannelBufferSize)

	go func() {
		defer close(batchChan)

		startTime := time.Now()
		totalFiles := 0
		totalSize := int64(0)

		// Count enabled categories
		categoriesTotal := 0
		if s.config.Categories.Cache {
			categoriesTotal++
		}
		if s.config.Categories.Temp {
			categoriesTotal++
		}
		if s.config.Categories.Logs {
			categoriesTotal++
		}
		if s.config.Categories.Duplicates {
			categoriesTotal++
		}
		if s.config.Categories.Downloads {
			categoriesTotal++
		}
		if s.config.Categories.PackageManagers {
			categoriesTotal++
		}

		categoriesDone := 0

		// Scan each category and stream results
		if s.config.Categories.Cache {
			s.reportScanProgress(progress.PhaseScanning, "cache", "", totalFiles, totalSize, categoriesTotal, categoriesDone, startTime)
			s.scanCategoryStreaming(ctx, "cache", s.ScanCache, batchChan, &totalFiles, &totalSize)
			categoriesDone++
		}

		if s.config.Categories.Temp {
			s.reportScanProgress(progress.PhaseScanning, "temp", "", totalFiles, totalSize, categoriesTotal, categoriesDone, startTime)
			s.scanCategoryStreaming(ctx, "temp", s.ScanTemp, batchChan, &totalFiles, &totalSize)
			categoriesDone++
		}

		if s.config.Categories.Logs {
			s.reportScanProgress(progress.PhaseScanning, "logs", "", totalFiles, totalSize, categoriesTotal, categoriesDone, startTime)
			s.scanCategoryStreaming(ctx, "logs", s.ScanLogs, batchChan, &totalFiles, &totalSize)
			categoriesDone++
		}

		if s.config.Categories.Duplicates {
			s.reportScanProgress(progress.PhaseScanning, "duplicates", "", totalFiles, totalSize, categoriesTotal, categoriesDone, startTime)
			s.scanCategoryStreaming(ctx, "duplicates", s.ScanDuplicates, batchChan, &totalFiles, &totalSize)
			categoriesDone++
		}

		if s.config.Categories.Downloads {
			s.reportScanProgress(progress.PhaseScanning, "downloads", "", totalFiles, totalSize, categoriesTotal, categoriesDone, startTime)
			s.scanCategoryStreaming(ctx, "downloads", s.ScanDownloads, batchChan, &totalFiles, &totalSize)
			categoriesDone++
		}

		if s.config.Categories.PackageManagers {
			s.reportScanProgress(progress.PhaseScanning, "packagemanagers", "", totalFiles, totalSize, categoriesTotal, categoriesDone, startTime)
			s.scanCategoryStreaming(ctx, "packagemanagers", s.ScanPackageManagers, batchChan, &totalFiles, &totalSize)
			categoriesDone++
		}

		// Report completion
		s.reportScanProgress(progress.PhaseComplete, "", "", totalFiles, totalSize, categoriesTotal, categoriesDone, startTime)
	}()

	return batchChan
}

// scanCategoryStreaming scans a category and converts to streaming batches
func (s *Scanner) scanCategoryStreaming(ctx context.Context, category string, scanFunc func() *ScanResult, batchChan chan<- *ScanBatch, totalFiles *int, totalSize *int64) {
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Run the scan
	result := scanFunc()

	// Update totals
	*totalFiles += result.TotalCount
	*totalSize += result.TotalSize

	// Convert to batches
	collector := NewBatchCollector(category, DefaultBatchSize, batchChan)
	for _, file := range result.Files {
		// Check for cancellation
		select {
		case <-ctx.Done():
			collector.SendError(ctx.Err())
			return
		default:
		}

		collector.Add(file)
	}

	// Send any errors
	for _, err := range result.Errors {
		collector.SendError(err)
	}

	collector.Finalize()
}

// reportScanProgress reports scan progress to listeners
func (s *Scanner) reportScanProgress(phase progress.Phase, category, currentPath string, filesFound int, totalSize int64, categoriesTotal, categoriesDone int, startTime time.Time) {
	if s.progressReporter == nil {
		return
	}

	s.progressReporter.UpdateScanProgress(&progress.ScanProgress{
		Phase:           phase,
		Category:        category,
		CurrentPath:     currentPath,
		FilesFound:      filesFound,
		TotalSize:       totalSize,
		CategoriesTotal: categoriesTotal,
		CategoriesDone:  categoriesDone,
		StartTime:       startTime,
	})
}

// merge combines two ScanResults
func (r *ScanResult) merge(other *ScanResult) {
	r.Files = append(r.Files, other.Files...)
	r.TotalSize += other.TotalSize
	r.TotalCount += other.TotalCount
	r.Errors = append(r.Errors, other.Errors...)
}

// shouldSkipFile checks if a file should be skipped based on configuration
func (s *Scanner) shouldSkipFile(path string, info os.FileInfo) bool {
	// Check if path is protected
	if s.isProtectedPath(path) {
		return true
	}

	// Check if path matches exclude patterns
	for _, pattern := range s.config.ExcludePattern {
		matched, _ := filepath.Match(pattern, path)
		if matched {
			return true
		}
		// Also check if any parent directory matches
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Check if path is in whitelist
	for _, whitelistPath := range s.config.WhitelistPaths {
		if strings.HasPrefix(path, whitelistPath) {
			return true
		}
	}

	// Check file age
	minAge := time.Duration(s.config.MinFileAge) * time.Hour
	if time.Since(info.ModTime()) < minAge {
		return true
	}

	return false
}

// isProtectedPath checks if a path is protected
func (s *Scanner) isProtectedPath(path string) bool {
	// Use centralized path validator
	if s.pathValidator.IsProtectedPath(path) {
		return true
	}

	// Check config protected paths
	for _, protected := range s.config.ProtectedPaths {
		if path == protected || strings.HasPrefix(path, protected+"/") {
			return true
		}
	}

	return false
}

// walkDirectory walks a directory and collects files matching criteria
func (s *Scanner) walkDirectory(dir string, category string, ageThreshold time.Duration) *ScanResult {
	result := &ScanResult{
		Files:    []FileInfo{},
		Category: category,
		Errors:   []error{},
	}

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return result
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Permission denied or other errors - skip and continue
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file should be skipped
		if s.shouldSkipFile(path, info) {
			return nil
		}

		// Check age threshold
		if ageThreshold > 0 && time.Since(info.ModTime()) < ageThreshold {
			return nil
		}

		// Add file to result
		fileInfo := FileInfo{
			Path:     path,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Category: category,
			Reason:   "Matches cleanup criteria",
		}

		result.Files = append(result.Files, fileInfo)
		result.TotalSize += info.Size()
		result.TotalCount++

		return nil
	})

	if err != nil {
		result.Errors = append(result.Errors, err)
	}

	return result
}

// GroupByCategory groups scan results by category
func (r *ScanResult) GroupByCategory() map[string]*ScanResult {
	grouped := make(map[string]*ScanResult)

	for _, file := range r.Files {
		if _, exists := grouped[file.Category]; !exists {
			grouped[file.Category] = &ScanResult{
				Files:    []FileInfo{},
				Category: file.Category,
			}
		}

		cat := grouped[file.Category]
		cat.Files = append(cat.Files, file)
		cat.TotalSize += file.Size
		cat.TotalCount++
	}

	return grouped
}
