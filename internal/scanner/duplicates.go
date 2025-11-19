package scanner

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/fenilsonani/cleanup-cache/pkg/utils"
)

// ScanDuplicates finds duplicate files
func (s *Scanner) ScanDuplicates() *ScanResult {
	result := &ScanResult{
		Files:    []FileInfo{},
		Category: "duplicates",
		Errors:   []error{},
	}

	// Map of hash to list of files with that hash
	hashMap := make(map[string][]FileInfo)

	// Directories to scan for duplicates (user directories only for safety)
	scanDirs := []string{
		s.platformInfo.HomeDir + "/Downloads",
		s.platformInfo.HomeDir + "/Documents",
		s.platformInfo.HomeDir + "/Desktop",
	}

	// Scan directories and build hash map
	for _, dir := range scanDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if info.IsDir() {
				return nil
			}

			// Skip if file should be skipped
			if s.shouldSkipFile(path, info) {
				return nil
			}

			// Skip small files (likely not worth finding duplicates)
			if info.Size() < 1024 { // 1KB
				return nil
			}

			// Compute hash (use quick hash for large files)
			var hash string
			var hashErr error
			if info.Size() > 10*1024*1024 { // 10MB
				hash, hashErr = utils.HashFileQuick(path, 1024*1024) // 1MB chunks
			} else {
				hash, hashErr = utils.HashFile(path)
			}

			if hashErr != nil {
				return nil // Skip on error
			}

			fileInfo := FileInfo{
				Path:     path,
				Size:     info.Size(),
				ModTime:  info.ModTime(),
				Category: "duplicates",
				Hash:     hash,
			}

			hashMap[hash] = append(hashMap[hash], fileInfo)

			return nil
		})

		if err != nil {
			result.Errors = append(result.Errors, err)
		}
	}

	// Find duplicates (files with same hash, keeping the newest)
	for hash, files := range hashMap {
		if len(files) <= 1 {
			continue // Not a duplicate
		}

		// Sort by modification time, newest first (O(n log n) instead of O(n²))
		sort.Slice(files, func(i, j int) bool {
			return files[i].ModTime.After(files[j].ModTime)
		})

		// Add all but the newest to result (keep the first one)
		for i := 1; i < len(files); i++ {
			file := files[i]
			file.Reason = "Duplicate of " + files[0].Path
			file.Hash = hash
			result.Files = append(result.Files, file)
			result.TotalSize += file.Size
			result.TotalCount++
		}
	}

	return result
}
