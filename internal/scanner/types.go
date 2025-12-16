package scanner

import "time"

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

// ProgressCallback is called during scanning to report progress
type ProgressCallback func(category, currentPath string, filesFound int, totalSize int64)

// GroupByCategory groups results by their category
func (r *ScanResult) GroupByCategory() map[string]*ScanResult {
	grouped := make(map[string]*ScanResult)

	for _, file := range r.Files {
		if _, ok := grouped[file.Category]; !ok {
			grouped[file.Category] = &ScanResult{
				Category: file.Category,
				Files:    make([]FileInfo, 0),
			}
		}
		grouped[file.Category].Files = append(grouped[file.Category].Files, file)
		grouped[file.Category].TotalSize += file.Size
		grouped[file.Category].TotalCount++
	}

	return grouped
}
