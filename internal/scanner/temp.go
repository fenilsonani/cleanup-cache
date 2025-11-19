package scanner

import (
	"time"
)

// ScanTemp scans for temporary files
func (s *Scanner) ScanTemp() *ScanResult {
	result := &ScanResult{
		Files:    []FileInfo{},
		Category: "temp",
		Errors:   []error{},
	}

	// Get age threshold from config
	ageThreshold := time.Duration(s.config.AgeThresholds.Temp) * 24 * time.Hour

	// Scan temp directories
	for _, tempDir := range s.platformInfo.TempDirs {
		dirResult := s.walkDirectory(tempDir, "temp", ageThreshold)
		result.merge(dirResult)
	}

	return result
}
