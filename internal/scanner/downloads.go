package scanner

import (
	"time"
)

// ScanDownloads scans the Downloads folder for old files
func (s *Scanner) ScanDownloads() *ScanResult {
	result := &ScanResult{
		Files:    []FileInfo{},
		Category: "downloads",
		Errors:   []error{},
	}

	// Get age threshold from config
	ageThreshold := time.Duration(s.config.AgeThresholds.Downloads) * 24 * time.Hour

	// Scan downloads directory
	dirResult := s.walkDirectory(s.platformInfo.DownloadsDir, "downloads", ageThreshold)
	result.merge(dirResult)

	return result
}
