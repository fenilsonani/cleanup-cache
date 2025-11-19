package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ScanLogs scans for log files
func (s *Scanner) ScanLogs() *ScanResult {
	result := &ScanResult{
		Files:    []FileInfo{},
		Category: "logs",
		Errors:   []error{},
	}

	// Get age threshold from config
	ageThreshold := time.Duration(s.config.AgeThresholds.Logs) * 24 * time.Hour

	// Scan log directories
	for _, logDir := range s.platformInfo.LogDirs {
		dirResult := s.scanLogDirectory(logDir, ageThreshold)
		result.merge(dirResult)
	}

	return result
}

// scanLogDirectory scans a directory for log files specifically
func (s *Scanner) scanLogDirectory(dir string, ageThreshold time.Duration) *ScanResult {
	result := &ScanResult{
		Files:    []FileInfo{},
		Category: "logs",
		Errors:   []error{},
	}

	// Common log file extensions
	logExtensions := []string{".log", ".log.gz", ".log.bz2", ".log.xz", ".log.1", ".log.2"}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip on error
		}

		if info.IsDir() {
			return nil
		}

		// Check if it's a log file
		isLogFile := false
		for _, ext := range logExtensions {
			if strings.HasSuffix(path, ext) {
				isLogFile = true
				break
			}
		}

		// Also check for rotated logs (e.g., file.log.1, file.log.2.gz)
		if strings.Contains(filepath.Base(path), ".log.") {
			isLogFile = true
		}

		if !isLogFile {
			return nil
		}

		// Skip if file should be skipped
		if s.shouldSkipFile(path, info) {
			return nil
		}

		// Check age threshold
		if ageThreshold > 0 && time.Since(info.ModTime()) < ageThreshold {
			return nil
		}

		fileInfo := FileInfo{
			Path:     path,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Category: "logs",
			Reason:   "Old log file",
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
