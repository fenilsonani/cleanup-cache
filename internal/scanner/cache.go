package scanner

// ScanCache scans for cache files
func (s *Scanner) ScanCache() *ScanResult {
	result := &ScanResult{
		Files:    []FileInfo{},
		Category: "cache",
		Errors:   []error{},
	}

	// Scan user cache directories
	for _, cacheDir := range s.platformInfo.CacheDirs {
		dirResult := s.walkDirectory(cacheDir, "cache", 0)
		result.merge(dirResult)
	}

	// Scan system caches (may require permissions)
	for _, systemCache := range s.platformInfo.SystemCaches {
		dirResult := s.walkDirectory(systemCache, "cache", 0)
		result.merge(dirResult)
	}

	return result
}

// ScanBrowserCaches specifically scans browser cache directories
func (s *Scanner) ScanBrowserCaches() *ScanResult {
	result := &ScanResult{
		Files:    []FileInfo{},
		Category: "browser_cache",
		Errors:   []error{},
	}

	browserCachePaths := s.getBrowserCachePaths()

	for _, cachePath := range browserCachePaths {
		dirResult := s.walkDirectory(cachePath, "browser_cache", 0)
		result.merge(dirResult)
	}

	return result
}

// getBrowserCachePaths returns browser-specific cache paths based on platform
func (s *Scanner) getBrowserCachePaths() []string {
	homeDir := s.platformInfo.HomeDir
	var paths []string

	switch s.platformInfo.OS {
	case "darwin":
		paths = []string{
			homeDir + "/Library/Caches/Google/Chrome",
			homeDir + "/Library/Caches/Firefox",
			homeDir + "/Library/Caches/com.apple.Safari",
			homeDir + "/Library/Caches/Microsoft Edge",
			homeDir + "/Library/Caches/Brave",
		}
	case "linux":
		paths = []string{
			homeDir + "/.cache/google-chrome",
			homeDir + "/.cache/chromium",
			homeDir + "/.cache/mozilla/firefox",
			homeDir + "/.cache/microsoft-edge",
			homeDir + "/.cache/BraveSoftware",
		}
	}

	return paths
}
