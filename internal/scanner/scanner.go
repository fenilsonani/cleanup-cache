package scanner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fenilsonani/cleanup-cache/internal/config"
	"github.com/fenilsonani/cleanup-cache/internal/platform"
	"github.com/fenilsonani/cleanup-cache/internal/progress"
	"github.com/fenilsonani/cleanup-cache/internal/security"
)

// FileInfo represents information about a file found during scanning
type FileInfo struct {
	Path     string
	Size     int64
	ModTime  time.Time
	Category string
	Reason   string // Why this file was flagged for cleanup
	Hash     string // For duplicate detection
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

// ScanWorker represents a worker for parallel directory scanning
type ScanWorker struct {
	id        int
	jobs      <-chan ScanJob
	results   chan<- ScanResult
	wg        *sync.WaitGroup
	semaphore chan struct{} // Limit concurrent I/O
}

// ScanJob represents a directory scanning job
type ScanJob struct {
	Path           string
	Category       string
	AgeThreshold   time.Duration
	ShouldSkipFunc func(string, os.FileInfo) bool
}

// OptimizedScanner adds performance optimizations to the base Scanner
type OptimizedScanner struct {
	*Scanner
	workerCount int
	fileInfoPool sync.Pool
}

// NewOptimized creates a new optimized scanner
func NewOptimized(cfg *config.Config, platformInfo *platform.Info) *OptimizedScanner {
	base := New(cfg, platformInfo)

	// Determine optimal worker count based on system resources
	workerCount := runtime.NumCPU()
	if workerCount < 4 {
		workerCount = 4 // Minimum 4 workers for I/O parallelism
	}
	if workerCount > 16 {
		workerCount = 16 // Cap at 16 to avoid excessive context switching
	}

	return &OptimizedScanner{
		Scanner:     base,
		workerCount: workerCount,
		fileInfoPool: sync.Pool{
			New: func() interface{} {
				return &FileInfo{}
			},
		},
	}
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
	if s.config.Categories.Downloads {
		categoriesTotal++
	}
	if s.config.Categories.PackageManagers {
		categoriesTotal++
	}
	if s.config.Categories.Docker {
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

	if s.config.Categories.Downloads {
		wg.Add(1)
		go runCategoryScan("downloads", s.ScanDownloads)
	}

	if s.config.Categories.PackageManagers {
		wg.Add(1)
		go runCategoryScan("package_managers", s.ScanPackageManagers)
	}

	if s.config.Categories.Docker {
		wg.Add(1)
		go runCategoryScan("docker", s.ScanDocker)
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
	// Fast pre-checks before expensive validation
	if s.pathValidator.SkipFileFast(path, info) {
		return true
	}

	// Check if path is protected using cached validation
	if err := s.pathValidator.ValidateCached(path); err != nil {
		return true // Skip if validation fails
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

// walkDirectoryOptimized walks a directory using WalkDir with optimizations
func (s *OptimizedScanner) walkDirectoryOptimized(dir string, category string, ageThreshold time.Duration) *ScanResult {
	// Pre-allocate slice with estimated capacity to reduce reallocations
	result := &ScanResult{
		Files:    make([]FileInfo, 0, 1000), // Start with capacity for 1000 files
		Category: category,
		Errors:   []error{},
	}

	// Check if directory exists
	if _, err := os.Stat(dir); err != nil && os.IsNotExist(err) {
		return result
	}

	// Use filepath.WalkDir for better performance (Go 1.16+)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Permission denied or other errors - skip and continue
			return nil
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get file info more efficiently
		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Check age threshold
		if ageThreshold > 0 && time.Since(info.ModTime()) < ageThreshold {
			return nil
		}

		// Quick path-based checks before expensive operations
		if s.quickSkipFile(path, info) {
			return nil
		}

		// Get FileInfo from pool
		fileInfo := s.fileInfoPool.Get().(*FileInfo)

		// Reset fields
		*fileInfo = FileInfo{
			Path:     path,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Category: category,
			Reason:   "Matches cleanup criteria",
		}

		// Add to result
		result.Files = append(result.Files, *fileInfo)
		result.TotalSize += info.Size()
		result.TotalCount++

		// Return to pool
		s.fileInfoPool.Put(fileInfo)

		return nil
	})

	if err != nil {
		result.Errors = append(result.Errors, err)
	}

	return result
}

// quickSkipFile performs fast checks to skip files without expensive operations
func (s *OptimizedScanner) quickSkipFile(path string, info os.FileInfo) bool {
	// Skip hidden files quickly
	if strings.HasPrefix(filepath.Base(path), ".") {
		return true
	}

	// Skip very large files in cache/temp dirs quickly
	if info.Size() > 100*1024*1024 { // > 100MB
		dir := filepath.Dir(path)
		if strings.Contains(dir, "cache") || strings.Contains(dir, "tmp") {
			return true
		}
	}

	// Fast path validation using cache
	if s.pathValidator.SkipFileFast(path, info) {
		return true
	}

	return false
}

// startWorkerPool initializes and starts the worker pool for directory scanning
func (s *OptimizedScanner) startWorkerPool(jobs chan ScanJob, results chan ScanResult) {
	var wg sync.WaitGroup

	// Create semaphore to limit concurrent I/O operations
	semaphore := make(chan struct{}, s.workerCount)

	// Start workers
	for i := 0; i < s.workerCount; i++ {
		wg.Add(1)
		go s.worker(i, jobs, results, &wg, semaphore)
	}

	// Wait for all workers to finish when jobs channel closes
	go func() {
		wg.Wait()
		close(results)
	}()
}

// worker processes directory scanning jobs
func (s *OptimizedScanner) worker(id int, jobs <-chan ScanJob, results chan<- ScanResult, wg *sync.WaitGroup, semaphore chan struct{}) {
	defer wg.Done()

	for job := range jobs {
		// Acquire semaphore to limit concurrent I/O
		semaphore <- struct{}{}

		func() {
			defer func() { <-semaphore }()

			// Process the job
			result := s.walkDirectoryOptimized(job.Path, job.Category, job.AgeThreshold)

			// Apply custom skip function if provided
			if job.ShouldSkipFunc != nil {
				filtered := make([]FileInfo, 0, len(result.Files))
				for _, file := range result.Files {
					if info, err := os.Stat(file.Path); err == nil {
						if !job.ShouldSkipFunc(file.Path, info) {
							filtered = append(filtered, file)
						}
					}
				}
				result.Files = filtered
				// Recalculate totals
				result.TotalCount = len(filtered)
				result.TotalSize = 0
				for _, file := range filtered {
					result.TotalSize += file.Size
				}
			}

			results <- *result
		}()
	}
}

// ScanAllOptimized scans all categories using optimized parallel processing
func (s *OptimizedScanner) ScanAllOptimized() (*ScanResult, error) {
	result := &ScanResult{
		Files:  []FileInfo{},
		Errors: []error{},
	}

	startTime := time.Now()

	// Create channels for jobs and results with larger buffers
	jobs := make(chan ScanJob, 50) // Buffer for pending jobs
	results := make(chan ScanResult, 10) // Buffer for results

	// Start worker pool
	s.startWorkerPool(jobs, results)

	// Count enabled categories and submit jobs
	var categoriesTotal int32
	var categoriesDone int32

	// Submit jobs in parallel
	go func() {
		defer close(jobs)

		if s.config.Categories.Cache {
			atomic.AddInt32(&categoriesTotal, 1)
			// For each cache directory, submit a job
			for _, cacheDir := range s.platformInfo.CacheDirs {
				jobs <- ScanJob{
					Path:         cacheDir,
					Category:     "cache",
					AgeThreshold: time.Duration(s.config.MinFileAge) * time.Hour,
				}
			}
			for _, systemCache := range s.platformInfo.SystemCaches {
				jobs <- ScanJob{
					Path:         systemCache,
					Category:     "cache",
					AgeThreshold: time.Duration(s.config.MinFileAge) * time.Hour,
				}
			}
		}

		if s.config.Categories.Temp {
			atomic.AddInt32(&categoriesTotal, 1)
			// For each temp directory, submit a job
			for _, tempDir := range s.platformInfo.TempDirs {
				jobs <- ScanJob{
					Path:         tempDir,
					Category:     "temp",
					AgeThreshold: time.Duration(s.config.MinFileAge) * time.Hour,
				}
			}
		}

		if s.config.Categories.Logs {
			atomic.AddInt32(&categoriesTotal, 1)
			// For each log directory, submit a job
			for _, logDir := range s.platformInfo.LogDirs {
				jobs <- ScanJob{
					Path:         logDir,
					Category:     "logs",
					AgeThreshold: time.Duration(s.config.MinFileAge) * time.Hour,
				}
			}
		}

		if s.config.Categories.Downloads {
			atomic.AddInt32(&categoriesTotal, 1)
			if s.platformInfo.DownloadsDir != "" {
				jobs <- ScanJob{
					Path:         s.platformInfo.DownloadsDir,
					Category:     "downloads",
					AgeThreshold: 30 * 24 * time.Hour, // 30 days
				}
			}
		}

		// Add more category jobs as needed...
	}()

	// Collect results with improved progress tracking
	var wg sync.WaitGroup
	resultChan := make(chan *ScanResult, int(categoriesTotal))

	wg.Add(1)
	go func() {
		defer wg.Done()
		for scanResult := range results {
			// Merge result atomically
			s.mergeResultAtomic(result, &scanResult)

			// Update progress
			done := atomic.AddInt32(&categoriesDone, 1)
			s.reportScanProgress(progress.PhaseScanning, scanResult.Category, "",
				result.TotalCount, result.TotalSize,
				int(categoriesTotal), int(done), startTime)
		}
	}()

	wg.Wait()
	close(resultChan)

	// Collect final merged results
	for partialResult := range resultChan {
		s.mergeResultAtomic(result, partialResult)
	}

	// Final progress report
	s.reportScanProgress(progress.PhaseComplete, "", "",
		result.TotalCount, result.TotalSize,
		int(categoriesTotal), int(categoriesTotal), startTime)

	return result, nil
}

// ScanAllStreamingOptimized scans using optimized streaming for large datasets
func (s *OptimizedScanner) ScanAllStreamingOptimized(ctx context.Context, batchSize int) (<-chan *ScanBatch, error) {
	// Use larger batch sizes for optimized scanning
	if batchSize <= 0 {
		batchSize = 50000 // Increased from 10000 for better throughput
	}

	batchChan := make(chan *ScanBatch, 100) // Larger buffer
	var totalFiles int64
	var totalSize int64
	var wg sync.WaitGroup

	// Create optimized job channels
	jobs := make(chan ScanJob, 100)
	results := make(chan ScanResult, 20)

	// Start worker pool
	s.startWorkerPool(jobs, results)

	// Submit all directory jobs
	var totalJobs int32
	go func() {
		defer close(jobs)

		if s.config.Categories.Cache {
			for _, cacheDir := range s.platformInfo.CacheDirs {
				atomic.AddInt32(&totalJobs, 1)
				jobs <- ScanJob{
					Path:         cacheDir,
					Category:     "cache",
					AgeThreshold: time.Duration(s.config.MinFileAge) * time.Hour,
				}
			}
			for _, systemCache := range s.platformInfo.SystemCaches {
				atomic.AddInt32(&totalJobs, 1)
				jobs <- ScanJob{
					Path:         systemCache,
					Category:     "cache",
					AgeThreshold: time.Duration(s.config.MinFileAge) * time.Hour,
				}
			}
		}

		if s.config.Categories.Temp {
			for _, tempDir := range s.platformInfo.TempDirs {
				atomic.AddInt32(&totalJobs, 1)
				jobs <- ScanJob{
					Path:         tempDir,
					Category:     "temp",
					AgeThreshold: time.Duration(s.config.MinFileAge) * time.Hour,
				}
			}
		}

		// ... add other categories ...
	}()

	// Use a map to track batch collectors per category
	batchCollectors := make(map[string]*BatchCollector)
	var mu sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(batchChan)

		var jobsDone int32

		for result := range results {
			// Get or create batch collector for this category
			mu.Lock()
			collector, exists := batchCollectors[result.Category]
			if !exists {
				collector = NewBatchCollector(result.Category, batchSize, batchChan)
				batchCollectors[result.Category] = collector
			}
			mu.Unlock()

			// Add files to collector
			for _, file := range result.Files {
				atomic.AddInt64(&totalFiles, 1)
				atomic.AddInt64(&totalSize, file.Size)
				collector.Add(file)
			}

			// Check if all jobs are done
			done := atomic.AddInt32(&jobsDone, 1)
			if int(done) == int(totalJobs) {
				// Flush remaining batches for all categories
				mu.Lock()
				for _, collector := range batchCollectors {
					collector.Flush()
				}
				mu.Unlock()
			}
		}
	}()

	return batchChan, nil
}

// mergeResultAtomically merges scan results safely
func (s *OptimizedScanner) mergeResultAtomic(target, source *ScanResult) {
	// Pre-allocate with combined capacity to avoid reallocations
	if cap(target.Files) < len(target.Files)+len(source.Files) {
		newFiles := make([]FileInfo, len(target.Files), len(target.Files)+len(source.Files))
		copy(newFiles, target.Files)
		target.Files = newFiles
	}

	target.Files = append(target.Files, source.Files...)
	target.TotalSize += source.TotalSize
	target.TotalCount += source.TotalCount
	target.Errors = append(target.Errors, source.Errors...)
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
