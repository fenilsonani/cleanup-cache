package scanner

import (
	"os"
	"path/filepath"

	"github.com/fenilsonani/cleanup-cache/internal/platform"
)

// ScanPackageManagers scans package manager caches
func (s *Scanner) ScanPackageManagers() *ScanResult {
	result := &ScanResult{
		Files:    []FileInfo{},
		Category: "package_managers",
		Errors:   []error{},
	}

	switch s.platformInfo.OS {
	case platform.MacOS:
		s.scanMacOSPackageManagers(result)
	case platform.Linux:
		s.scanLinuxPackageManagers(result)
	}

	// Scan common language package managers (cross-platform)
	s.scanLanguagePackageManagers(result)

	return result
}

// scanMacOSPackageManagers scans macOS-specific package managers
func (s *Scanner) scanMacOSPackageManagers(result *ScanResult) {
	// Homebrew cache
	brewCachePaths := []string{
		"/Library/Caches/Homebrew",
		s.platformInfo.HomeDir + "/Library/Caches/Homebrew",
	}

	for _, cachePath := range brewCachePaths {
		dirResult := s.walkDirectory(cachePath, "homebrew_cache", 0)
		result.merge(dirResult)
	}
}

// scanLinuxPackageManagers scans Linux-specific package managers
func (s *Scanner) scanLinuxPackageManagers(result *ScanResult) {
	packageManagerCaches := []string{
		"/var/cache/apt/archives",
		"/var/cache/yum",
		"/var/cache/dnf",
		"/var/cache/pacman/pkg",
		"/var/lib/snapd/cache",
	}

	for _, cachePath := range packageManagerCaches {
		if _, err := os.Stat(cachePath); os.IsNotExist(err) {
			continue
		}

		dirResult := s.walkDirectory(cachePath, "system_package_cache", 0)
		result.merge(dirResult)
	}
}

// scanLanguagePackageManagers scans language-specific package manager caches
func (s *Scanner) scanLanguagePackageManagers(result *ScanResult) {
	homeDir := s.platformInfo.HomeDir

	languageCaches := []struct {
		path     string
		category string
	}{
		// Node.js / npm / yarn
		{filepath.Join(homeDir, ".npm"), "npm_cache"},
		{filepath.Join(homeDir, ".yarn/cache"), "yarn_cache"},
		{filepath.Join(homeDir, ".cache/yarn"), "yarn_cache"},
		{filepath.Join(homeDir, ".pnpm-store"), "pnpm_cache"},

		// Python / pip
		{filepath.Join(homeDir, ".cache/pip"), "pip_cache"},
		{filepath.Join(homeDir, "Library/Caches/pip"), "pip_cache"}, // macOS

		// Ruby / gem
		{filepath.Join(homeDir, ".gem/cache"), "gem_cache"},

		// Rust / cargo
		{filepath.Join(homeDir, ".cargo/registry/cache"), "cargo_cache"},

		// Go
		{filepath.Join(homeDir, ".cache/go-build"), "go_cache"},
		{filepath.Join(homeDir, "Library/Caches/go-build"), "go_cache"}, // macOS

		// Java / Maven / Gradle
		{filepath.Join(homeDir, ".m2/repository"), "maven_cache"},
		{filepath.Join(homeDir, ".gradle/caches"), "gradle_cache"},

		// PHP / Composer
		{filepath.Join(homeDir, ".composer/cache"), "composer_cache"},
		{filepath.Join(homeDir, ".cache/composer"), "composer_cache"},

		// CocoaPods (macOS)
		{filepath.Join(homeDir, "Library/Caches/CocoaPods"), "cocoapods_cache"},
	}

	for _, cache := range languageCaches {
		if _, err := os.Stat(cache.path); os.IsNotExist(err) {
			continue
		}

		dirResult := s.walkDirectory(cache.path, cache.category, 0)
		result.merge(dirResult)
	}
}
