package scanner

import (
	"bytes"
	"crypto/md5"
	"encoding/gob"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fenilsonani/system-cleanup/internal/config"
	"github.com/fenilsonani/system-cleanup/internal/platform"
)

// HyperScanner uses advanced techniques for blazing fast scanning
// - Directory mtime caching (skip unchanged directories)
// - macOS Spotlight integration (mdfind for indexed searches)
// - Persistent scan cache for instant re-scans
// - Parallel syscalls with optimal batching
type HyperScanner struct {
	config       *config.Config
	platformInfo *platform.Info
	progressCb   ProgressCallback

	// Scan cache - persisted between runs
	cache     *ScanCache
	cachePath string
	cacheMu   sync.RWMutex // Protects cache map access

	// Runtime state
	filesFound int64
	totalSize  int64

	// Worker pool
	workerCount int
	sem         chan struct{}

	// Results
	resultMu sync.Mutex
	results  []FileInfo
}

// ScanCache stores scan results for fast re-scanning
type ScanCache struct {
	Version      int                       `json:"version"`
	LastScan     time.Time                 `json:"last_scan"`
	DirMtimes    map[string]time.Time      `json:"dir_mtimes"`    // Directory -> last modified
	DirResults   map[string]*CachedDirInfo `json:"dir_results"`   // Directory -> cached scan results
	ArtifactDirs map[string][]string       `json:"artifact_dirs"` // DevDir -> list of artifact paths
}

// CachedDirInfo stores cached info about a directory
type CachedDirInfo struct {
	Path      string    `json:"path"`
	TotalSize int64     `json:"total_size"`
	FileCount int       `json:"file_count"`
	Category  string    `json:"category"`
	ScannedAt time.Time `json:"scanned_at"`
	Checksum  string    `json:"checksum"` // For validation
}

// NewHyperScanner creates a new hyper-optimized scanner
func NewHyperScanner(cfg *config.Config, platformInfo *platform.Info) *HyperScanner {
	workers := runtime.NumCPU() * 4
	if workers < 16 {
		workers = 16
	}
	if workers > 64 {
		workers = 64
	}

	home, _ := os.UserHomeDir()
	cachePath := filepath.Join(home, ".cache", "tidyup", "scan_cache.gob")

	hs := &HyperScanner{
		config:       cfg,
		platformInfo: platformInfo,
		workerCount:  workers,
		sem:          make(chan struct{}, workers),
		cachePath:    cachePath,
		results:      make([]FileInfo, 0, 10000),
	}

	// Load existing cache
	hs.loadCache()

	return hs
}

// SetProgressCallback sets the progress callback
func (hs *HyperScanner) SetProgressCallback(cb ProgressCallback) {
	hs.progressCb = cb
}

// loadCache loads the scan cache from disk
func (hs *HyperScanner) loadCache() {
	hs.cache = &ScanCache{
		Version:      1,
		DirMtimes:    make(map[string]time.Time),
		DirResults:   make(map[string]*CachedDirInfo),
		ArtifactDirs: make(map[string][]string),
	}

	f, err := os.Open(hs.cachePath)
	if err != nil {
		return // No cache yet
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	var cache ScanCache
	if err := dec.Decode(&cache); err == nil {
		// Only use cache if it's recent (within 1 hour)
		if time.Since(cache.LastScan) < time.Hour {
			// Ensure maps are initialized
			if cache.ArtifactDirs == nil {
				cache.ArtifactDirs = make(map[string][]string)
			}
			hs.cache = &cache
		}
	}
}

// saveCache saves the scan cache to disk
func (hs *HyperScanner) saveCache() {
	if err := os.MkdirAll(filepath.Dir(hs.cachePath), 0755); err != nil {
		return
	}

	f, err := os.Create(hs.cachePath)
	if err != nil {
		return
	}
	defer f.Close()

	hs.cacheMu.RLock()
	hs.cache.LastScan = time.Now()
	enc := gob.NewEncoder(f)
	if err := enc.Encode(hs.cache); err != nil {
		hs.cacheMu.RUnlock()
		return
	}
	hs.cacheMu.RUnlock()
}

// ScanAll performs a hyper-fast scan of all enabled categories
func (hs *HyperScanner) ScanAll() (*ScanResult, error) {
	atomic.StoreInt64(&hs.filesFound, 0)
	atomic.StoreInt64(&hs.totalSize, 0)
	hs.results = make([]FileInfo, 0, 10000)

	var wg sync.WaitGroup

	// Scan categories in parallel using optimal strategies
	if hs.config.Categories.Cache {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hs.scanCacheCategory()
		}()
	}

	if hs.config.Categories.Temp {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hs.scanTempCategory()
		}()
	}

	if hs.config.Categories.Logs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hs.scanLogsCategory()
		}()
	}

	// Dev artifacts - use smart directory detection
	if hs.config.Categories.NodeModules || hs.config.Categories.VirtualEnvs || hs.config.Categories.BuildArtifacts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hs.scanDevArtifacts()
		}()
	}

	// Large files - use Spotlight on macOS
	if hs.config.Categories.LargeFiles {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hs.scanLargeFilesSpotlight()
		}()
	}

	// Old files - use Spotlight for access time
	if hs.config.Categories.OldFiles {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hs.scanOldFilesSpotlight()
		}()
	}

	// Docker - scan Docker artifacts
	if hs.config.Categories.Docker {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hs.scanDockerCategory()
		}()
	}

	wg.Wait()

	// Save cache for next run
	hs.saveCache()

	return &ScanResult{
		Files:      hs.results,
		TotalSize:  atomic.LoadInt64(&hs.totalSize),
		TotalCount: len(hs.results),
	}, nil
}

// ScanCategory scans only one category
func (hs *HyperScanner) ScanCategory(category string) *ScanResult {
	atomic.StoreInt64(&hs.filesFound, 0)
	atomic.StoreInt64(&hs.totalSize, 0)
	hs.results = make([]FileInfo, 0, 5000)

	switch category {
	case "cache":
		hs.scanCacheCategory()
	case "temp":
		hs.scanTempCategory()
	case "logs":
		hs.scanLogsCategory()
	case "node_modules":
		hs.scanDevArtifactsType("node_modules")
	case "virtual_envs":
		hs.scanDevArtifactsType("venv")
	case "build_artifacts":
		hs.scanDevArtifactsType("build")
	case "large_files":
		hs.scanLargeFilesSpotlight()
	case "old_files":
		hs.scanOldFilesSpotlight()
	case "docker":
		hs.scanDockerCategory()
	}

	return &ScanResult{
		Files:      hs.results,
		TotalSize:  atomic.LoadInt64(&hs.totalSize),
		TotalCount: len(hs.results),
		Category:   category,
	}
}

// scanCacheCategory scans cache directories with mtime optimization
func (hs *HyperScanner) scanCacheCategory() {
	dirs := hs.getCacheDirs()
	hs.scanDirsWithCache(dirs, "cache")
}

// scanTempCategory scans temp directories
func (hs *HyperScanner) scanTempCategory() {
	dirs := hs.platformInfo.TempDirs
	hs.scanDirsWithCache(dirs, "temp")
}

// scanLogsCategory scans log directories
func (hs *HyperScanner) scanLogsCategory() {
	dirs := hs.platformInfo.LogDirs
	hs.scanDirsWithCache(dirs, "logs")
}

// scanDockerCategory scans Docker artifacts and unused containers/images
func (hs *HyperScanner) scanDockerCategory() {
	if !hs.config.Docker.Enabled {
		return
	}

	home, _ := os.UserHomeDir()

	// Get Docker artifact directories based on platform
	var dockerDirs []string

	if runtime.GOOS == "darwin" {
		// macOS Docker Desktop - scan specific subdirectories
		basePath := filepath.Join(home, "Library/Containers/com.docker.docker/Data")
		dockerDirs = []string{
			filepath.Join(basePath, "vms"),
			filepath.Join(basePath, "log"),
		}
	} else if runtime.GOOS == "linux" {
		// Linux Docker
		dockerDirs = []string{
			filepath.Join(home, ".docker"),
		}
		// Also try system Docker if not running as root
		if os.Getuid() != 0 {
			dockerDirs = append(dockerDirs, "/var/lib/docker")
		}
	}

	// Scan Docker artifact directories and get total size
	for _, dir := range dockerDirs {
		if info, err := os.Stat(dir); err == nil {
			// For Docker directories, treat as a single item with the directory size
			totalSize := hs.getDirSize(dir)
			if totalSize > 0 {
				// Add as a single "file" item for deletion
				hs.addResult(dir, "docker", totalSize, info.ModTime())
			}
		}
	}

	// Try to clean up using Docker CLI if daemon is running
	if hs.config.Docker.CleanBuildCache {
		hs.scanDockerCLI()
	}
}

// getDirSize recursively calculates directory size
func (hs *HyperScanner) getDirSize(path string) int64 {
	var size int64
	filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return size
}

// scanDockerCLI scans Docker artifacts using the Docker CLI
func (hs *HyperScanner) scanDockerCLI() {
	// Check if docker command is available
	_, err := exec.LookPath("docker")
	if err != nil {
		return // Docker not installed
	}

	// Try to get Docker system info - if this fails, daemon isn't running
	cmd := exec.Command("docker", "system", "df", "--format", "json")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return // Docker daemon not running
	}

	// Parse docker system df output to estimate cleanup size
	// For now, just track that we attempted to scan
	// Actual cleanup will be handled by the cleaner
}

// scanDirsWithCache scans directories using mtime caching
func (hs *HyperScanner) scanDirsWithCache(dirs []string, category string) {
	var wg sync.WaitGroup

	for _, dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			hs.scanDirOptimized(d, category)
		}(dir)
	}

	wg.Wait()
}

// scanDirOptimized scans a directory with mtime-based caching
func (hs *HyperScanner) scanDirOptimized(dir, category string) {
	// Check if directory has changed since last scan
	info, err := os.Stat(dir)
	if err != nil {
		return
	}

	dirMtime := info.ModTime()
	cacheKey := fmt.Sprintf("%s:%s", dir, category)

	// Check cache (read lock)
	hs.cacheMu.RLock()
	cachedMtime, hasMtime := hs.cache.DirMtimes[cacheKey]
	cached, hasCached := hs.cache.DirResults[cacheKey]
	hs.cacheMu.RUnlock()

	if hasMtime && !dirMtime.After(cachedMtime) && hasCached {
		// Directory unchanged, use cached results
		hs.addCachedResult(cached)
		return
	}

	// Directory changed or not in cache - do full scan
	var totalSize int64
	var fileCount int

	hs.sem <- struct{}{}
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if len(name) > 0 && name[0] == '.' && name != ".cache" && name != ".npm" {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Age check
		ageThreshold := time.Duration(hs.config.MinFileAge) * time.Hour
		if time.Since(info.ModTime()) < ageThreshold {
			return nil
		}

		totalSize += info.Size()
		fileCount++

		hs.addResult(path, category, info.Size(), info.ModTime())
		return nil
	})
	<-hs.sem

	// Update cache (write lock)
	hs.cacheMu.Lock()
	hs.cache.DirMtimes[cacheKey] = dirMtime
	hs.cache.DirResults[cacheKey] = &CachedDirInfo{
		Path:      dir,
		TotalSize: totalSize,
		FileCount: fileCount,
		Category:  category,
		ScannedAt: time.Now(),
	}
	hs.cacheMu.Unlock()
}

// scanDevArtifacts uses smart detection for dev artifacts
func (hs *HyperScanner) scanDevArtifacts() {
	home, _ := os.UserHomeDir()
	devDirs := make([]string, 0)

	for _, d := range hs.config.Dev.ProjectDirs {
		d = expandPath(d, home)
		if _, err := os.Stat(d); err == nil {
			devDirs = append(devDirs, d)
		}
	}

	var wg sync.WaitGroup
	for _, dir := range devDirs {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			hs.findDevArtifactsInDir(d)
		}(dir)
	}
	wg.Wait()
}

// scanDevArtifactsType scans for a specific type of dev artifact
func (hs *HyperScanner) scanDevArtifactsType(artifactType string) {
	home, _ := os.UserHomeDir()
	devDirs := make([]string, 0)

	for _, d := range hs.config.Dev.ProjectDirs {
		d = expandPath(d, home)
		if _, err := os.Stat(d); err == nil {
			devDirs = append(devDirs, d)
		}
	}

	var wg sync.WaitGroup
	for _, dir := range devDirs {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			hs.findDevArtifactsOfType(d, artifactType)
		}(dir)
	}
	wg.Wait()
}

// findDevArtifactsInDir finds all dev artifacts in a directory using find command
func (hs *HyperScanner) findDevArtifactsInDir(dir string) {
	// Check cache first - skip find command if dev dir hasn't changed
	cacheKey := fmt.Sprintf("devdir:%s", dir)

	info, err := os.Stat(dir)
	if err != nil {
		return
	}
	dirMtime := info.ModTime()

	// Check cache (read lock)
	hs.cacheMu.RLock()
	cachedMtime, hasMtime := hs.cache.DirMtimes[cacheKey]
	cachedPaths, hasPaths := hs.cache.ArtifactDirs[cacheKey]
	hs.cacheMu.RUnlock()

	// If we have cached artifacts and directory hasn't changed, use cache
	if hasMtime && !dirMtime.After(cachedMtime) && hasPaths {
		// Use cached artifact paths - super fast!
		var artifactWg sync.WaitGroup
		for _, path := range cachedPaths {
			// Verify path still exists
			if _, err := os.Stat(path); err == nil {
				category := hs.categorizeArtifact(filepath.Base(path))
				if category != "" {
					artifactWg.Add(1)
					go func(p, cat string) {
						defer artifactWg.Done()
						hs.addArtifactResult(p, cat)
					}(path, category)
				}
			}
		}
		artifactWg.Wait()
		return
	}

	// Directory changed or not cached - run find
	patterns := []string{}

	if hs.config.Categories.NodeModules {
		patterns = append(patterns, "-name", "node_modules", "-type", "d", "-o")
	}
	if hs.config.Categories.VirtualEnvs {
		patterns = append(patterns, "-name", "venv", "-type", "d", "-o", "-name", ".venv", "-type", "d", "-o",
			"-name", "virtualenv", "-type", "d", "-o")
	}
	if hs.config.Categories.BuildArtifacts {
		patterns = append(patterns, "-name", "dist", "-type", "d", "-o", "-name", "build", "-type", "d", "-o",
			"-name", ".next", "-type", "d", "-o", "-name", "__pycache__", "-type", "d", "-o",
			"-name", "target", "-type", "d", "-o", "-name", ".gradle", "-type", "d", "-o",
			"-name", "out", "-type", "d", "-o")
	}

	if len(patterns) == 0 {
		return
	}

	// Remove trailing -o
	patterns = patterns[:len(patterns)-1]

	// Build find command with pruning for speed
	args := []string{dir, "-maxdepth", "6", "("}
	args = append(args, patterns...)
	args = append(args, ")", "-prune", "-print")

	cmd := exec.Command("find", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		// Fallback to manual scan
		hs.findDevArtifactsManual(dir)
		return
	}

	// Collect found paths for caching
	foundPaths := make([]string, 0)

	// Process results - collect all paths first
	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			foundPaths = append(foundPaths, line)
		}
	}

	// Process artifacts in parallel for faster du calls
	var artifactWg sync.WaitGroup
	for _, line := range foundPaths {
		category := hs.categorizeArtifact(filepath.Base(line))
		if category != "" {
			artifactWg.Add(1)
			go func(p, cat string) {
				defer artifactWg.Done()
				hs.addArtifactResult(p, cat)
			}(line, category)
		}
	}
	artifactWg.Wait()

	// Update cache with found artifact paths (write lock)
	hs.cacheMu.Lock()
	hs.cache.DirMtimes[cacheKey] = dirMtime
	hs.cache.ArtifactDirs[cacheKey] = foundPaths
	hs.cacheMu.Unlock()
}

// findDevArtifactsOfType finds artifacts of a specific type
func (hs *HyperScanner) findDevArtifactsOfType(dir, artifactType string) {
	var names []string
	var category string

	switch artifactType {
	case "node_modules":
		names = []string{"node_modules"}
		category = "node_modules"
	case "venv":
		names = []string{"venv", ".venv", "virtualenv"}
		category = "virtual_envs"
	case "build":
		names = []string{"dist", "build", ".next", "__pycache__", "target", ".gradle", "out"}
		category = "build_artifacts"
	}

	// Check cache first
	cacheKey := fmt.Sprintf("devdir:%s:%s", dir, artifactType)

	info, err := os.Stat(dir)
	if err != nil {
		return
	}
	dirMtime := info.ModTime()

	// Use cached results if dev dir hasn't changed (read lock)
	hs.cacheMu.RLock()
	cachedMtime, hasMtime := hs.cache.DirMtimes[cacheKey]
	cachedPaths, hasPaths := hs.cache.ArtifactDirs[cacheKey]
	hs.cacheMu.RUnlock()

	if hasMtime && !dirMtime.After(cachedMtime) && hasPaths {
		var artifactWg sync.WaitGroup
		for _, path := range cachedPaths {
			if _, err := os.Stat(path); err == nil {
				artifactWg.Add(1)
				go func(p string) {
					defer artifactWg.Done()
					hs.addArtifactResult(p, category)
				}(path)
			}
		}
		artifactWg.Wait()
		return
	}

	// Run find command
	args := []string{dir, "-maxdepth", "6", "("}
	for i, name := range names {
		if i > 0 {
			args = append(args, "-o")
		}
		args = append(args, "-name", name, "-type", "d")
	}
	args = append(args, ")", "-prune", "-print")

	cmd := exec.Command("find", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return
	}

	foundPaths := make([]string, 0)
	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			foundPaths = append(foundPaths, line)
		}
	}

	// Process artifacts in parallel for faster du calls
	var artifactWg sync.WaitGroup
	for _, line := range foundPaths {
		artifactWg.Add(1)
		go func(p string) {
			defer artifactWg.Done()
			hs.addArtifactResult(p, category)
		}(line)
	}
	artifactWg.Wait()

	// Update cache (write lock)
	hs.cacheMu.Lock()
	hs.cache.DirMtimes[cacheKey] = dirMtime
	hs.cache.ArtifactDirs[cacheKey] = foundPaths
	hs.cacheMu.Unlock()
}

// findDevArtifactsManual fallback manual scan
func (hs *HyperScanner) findDevArtifactsManual(dir string) {
	var wg sync.WaitGroup
	var depth int32

	var scanDir func(path string)
	scanDir = func(path string) {
		defer wg.Done()

		currentDepth := atomic.AddInt32(&depth, 1)
		defer atomic.AddInt32(&depth, -1)

		if currentDepth > 6 {
			return
		}

		hs.sem <- struct{}{}
		entries, err := os.ReadDir(path)
		<-hs.sem

		if err != nil {
			return
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			if len(name) > 0 && name[0] == '.' && name != ".venv" && name != ".next" {
				continue
			}

			fullPath := filepath.Join(path, name)
			category := hs.categorizeArtifact(name)

			if category != "" {
				hs.addArtifactResult(fullPath, category)
			} else {
				wg.Add(1)
				go scanDir(fullPath)
			}
		}
	}

	wg.Add(1)
	go scanDir(dir)
	wg.Wait()
}

// categorizeArtifact returns the category for a dev artifact directory
func (hs *HyperScanner) categorizeArtifact(name string) string {
	switch name {
	case "node_modules":
		if hs.config.Categories.NodeModules {
			return "node_modules"
		}
	case "venv", ".venv", "virtualenv":
		if hs.config.Categories.VirtualEnvs {
			return "virtual_envs"
		}
	case "dist", "build", ".next", "__pycache__", "target", ".gradle", "out":
		if hs.config.Categories.BuildArtifacts {
			return "build_artifacts"
		}
	}
	return ""
}

// scanLargeFilesSpotlight uses Spotlight for fast large file discovery on macOS
func (hs *HyperScanner) scanLargeFilesSpotlight() {
	minSize := hs.parseSize(hs.config.LargeFiles.MinSize)
	home, _ := os.UserHomeDir()

	// Use mdfind (Spotlight) on macOS for instant results
	// Query: files larger than minSize in home directory
	query := fmt.Sprintf("kMDItemFSSize > %d", minSize)

	cmd := exec.Command("mdfind", "-onlyin", home, query)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		// Fallback to manual scan
		hs.scanLargeFilesManual()
		return
	}

	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip excluded paths
		skip := false
		for _, excl := range hs.config.LargeFiles.ExcludePaths {
			excl = expandPath(excl, home)
			if strings.HasPrefix(line, excl) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		info, err := os.Stat(line)
		if err != nil || info.IsDir() {
			continue
		}

		hs.addResult(line, "large_files", info.Size(), info.ModTime())
	}
}

// scanLargeFilesManual fallback for non-macOS or when Spotlight fails
func (hs *HyperScanner) scanLargeFilesManual() {
	home, _ := os.UserHomeDir()
	minSize := hs.parseSize(hs.config.LargeFiles.MinSize)

	for _, scanPath := range hs.config.LargeFiles.ScanPaths {
		scanPath = expandPath(scanPath, home)

		filepath.WalkDir(scanPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			// Skip excluded paths
			for _, excl := range hs.config.LargeFiles.ExcludePaths {
				excl = expandPath(excl, home)
				if strings.HasPrefix(path, excl) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			if d.IsDir() {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return nil
			}

			if info.Size() >= minSize {
				hs.addResult(path, "large_files", info.Size(), info.ModTime())
			}

			return nil
		})
	}
}

// scanOldFilesSpotlight uses Spotlight for fast old file discovery
func (hs *HyperScanner) scanOldFilesSpotlight() {
	minAgeDays := hs.config.OldFiles.MinAgeDays
	cutoff := time.Now().AddDate(0, 0, -minAgeDays)
	home, _ := os.UserHomeDir()

	// Use mdfind for files not accessed since cutoff
	// kMDItemLastUsedDate < cutoff
	query := fmt.Sprintf("kMDItemLastUsedDate < $time.iso(%s)", cutoff.Format("2006-01-02"))

	for _, scanPath := range hs.config.OldFiles.ScanPaths {
		scanPath = expandPath(scanPath, home)

		cmd := exec.Command("mdfind", "-onlyin", scanPath, query)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = nil

		if err := cmd.Run(); err != nil {
			// Fallback to manual scan for this path
			hs.scanOldFilesManual(scanPath)
			continue
		}

		for _, line := range strings.Split(out.String(), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			info, err := os.Stat(line)
			if err != nil || info.IsDir() {
				continue
			}

			hs.addResult(line, "old_files", info.Size(), info.ModTime())
		}
	}
}

// scanOldFilesManual fallback for old files scanning
func (hs *HyperScanner) scanOldFilesManual(dir string) {
	minAgeDays := hs.config.OldFiles.MinAgeDays
	cutoff := time.Now().AddDate(0, 0, -minAgeDays)

	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		if info.ModTime().Before(cutoff) {
			hs.addResult(path, "old_files", info.Size(), info.ModTime())
		}

		return nil
	})
}

// addResult adds a file result
func (hs *HyperScanner) addResult(path, category string, size int64, modTime time.Time) {
	hs.resultMu.Lock()
	hs.results = append(hs.results, FileInfo{
		Path:     path,
		Size:     size,
		ModTime:  modTime,
		Category: category,
		Reason:   "Matches cleanup criteria",
	})
	hs.resultMu.Unlock()

	atomic.AddInt64(&hs.filesFound, 1)
	atomic.AddInt64(&hs.totalSize, size)

	if hs.progressCb != nil {
		hs.progressCb(category, path, int(atomic.LoadInt64(&hs.filesFound)), atomic.LoadInt64(&hs.totalSize))
	}
}

// addArtifactResult adds a dev artifact directory result with caching
func (hs *HyperScanner) addArtifactResult(path, category string) {
	// First verify the path exists
	if _, err := os.Stat(path); err != nil {
		return // Skip non-existent paths
	}

	cacheKey := fmt.Sprintf("artifact:%s", path)

	// Check cache first (read lock)
	hs.cacheMu.RLock()
	cached, hasCached := hs.cache.DirResults[cacheKey]
	cachedMtime, hasMtime := hs.cache.DirMtimes[cacheKey]
	hs.cacheMu.RUnlock()

	if hasCached {
		// Verify directory hasn't changed
		info, err := os.Stat(path)
		if err == nil && hasMtime && !info.ModTime().After(cachedMtime) {
			// Use cached result
			hs.resultMu.Lock()
			hs.results = append(hs.results, FileInfo{
				Path:     cached.Path,
				Size:     cached.TotalSize,
				Category: category,
				Reason:   fmt.Sprintf("Dev artifact: ~%d files (cached)", cached.FileCount),
			})
			hs.resultMu.Unlock()
			atomic.AddInt64(&hs.filesFound, 1)
			atomic.AddInt64(&hs.totalSize, cached.TotalSize)
			return
		}
	}

	// Quick size calculation using du - run with semaphore for parallelism
	hs.sem <- struct{}{}

	var size int64
	var fileCount int

	cmd := exec.Command("du", "-sk", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil

	if err := cmd.Run(); err == nil {
		parts := strings.Fields(out.String())
		if len(parts) > 0 {
			fmt.Sscanf(parts[0], "%d", &size)
			size *= 1024 // du -sk returns KB
		}
	}

	<-hs.sem

	// Estimate file count from size (avg 10KB per file)
	fileCount = int(size / (10 * 1024))
	if fileCount < 1 {
		fileCount = 1
	}

	// Update cache (write lock)
	if info, err := os.Stat(path); err == nil {
		hs.cacheMu.Lock()
		hs.cache.DirMtimes[cacheKey] = info.ModTime()
		hs.cache.DirResults[cacheKey] = &CachedDirInfo{
			Path:      path,
			TotalSize: size,
			FileCount: fileCount,
			Category:  category,
			ScannedAt: time.Now(),
		}
		hs.cacheMu.Unlock()
	}

	hs.resultMu.Lock()
	hs.results = append(hs.results, FileInfo{
		Path:     path,
		Size:     size,
		Category: category,
		Reason:   fmt.Sprintf("Dev artifact: ~%d files", fileCount),
	})
	hs.resultMu.Unlock()

	atomic.AddInt64(&hs.filesFound, 1)
	atomic.AddInt64(&hs.totalSize, size)
}

// addCachedResult adds results from cache
func (hs *HyperScanner) addCachedResult(cached *CachedDirInfo) {
	hs.resultMu.Lock()
	hs.results = append(hs.results, FileInfo{
		Path:     cached.Path,
		Size:     cached.TotalSize,
		Category: cached.Category,
		Reason:   fmt.Sprintf("Cached: %d files", cached.FileCount),
	})
	hs.resultMu.Unlock()

	atomic.AddInt64(&hs.filesFound, int64(cached.FileCount))
	atomic.AddInt64(&hs.totalSize, cached.TotalSize)
}

// getCacheDirs returns cache directories
func (hs *HyperScanner) getCacheDirs() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{
		filepath.Join(home, "Library", "Caches"),
		filepath.Join(home, ".cache"),
		filepath.Join(home, ".npm", "_cacache"),
		filepath.Join(home, "go", "pkg", "mod", "cache"),
	}
	dirs = append(dirs, hs.platformInfo.CacheDirs...)
	dirs = append(dirs, hs.platformInfo.SystemCaches...)

	result := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if _, err := os.Stat(d); err == nil {
			result = append(result, d)
		}
	}
	return result
}

// parseSize parses a size string
func (hs *HyperScanner) parseSize(s string) int64 {
	s = strings.ToUpper(strings.TrimSpace(s))
	multiplier := int64(1)

	if strings.HasSuffix(s, "GB") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	} else if strings.HasSuffix(s, "MB") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	} else if strings.HasSuffix(s, "KB") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "KB")
	}

	var val int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			val = val*10 + int64(c-'0')
		}
	}
	return val * multiplier
}

// dirChecksum creates a quick checksum for directory validation
func dirChecksum(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	data := fmt.Sprintf("%s:%d:%d", path, info.ModTime().Unix(), info.Size())
	return fmt.Sprintf("%x", md5.Sum([]byte(data)))
}

// expandPath expands ~ to home directory
func expandPath(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
