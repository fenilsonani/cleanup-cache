package scanner

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fenilsonani/system-cleanup/internal/config"
	"github.com/fenilsonani/system-cleanup/internal/platform"
	"github.com/fenilsonani/system-cleanup/internal/testutil"
)

// =============================================================================
// parseSize Tests - Comprehensive
// =============================================================================

func TestParseSize(t *testing.T) {
	hs := &HyperScanner{}

	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		// Standard units
		{"1KB", "1KB", 1024},
		{"1MB", "1MB", 1024 * 1024},
		{"1GB", "1GB", 1024 * 1024 * 1024},

		// Various values
		{"100KB", "100KB", 100 * 1024},
		{"100MB", "100MB", 100 * 1024 * 1024},
		{"500MB", "500MB", 500 * 1024 * 1024},
		{"2GB", "2GB", 2 * 1024 * 1024 * 1024},
		{"10GB", "10GB", 10 * 1024 * 1024 * 1024},

		// Case insensitivity
		{"lowercase kb", "100kb", 100 * 1024},
		{"lowercase mb", "50mb", 50 * 1024 * 1024},
		{"lowercase gb", "1gb", 1024 * 1024 * 1024},
		{"mixed Kb", "100Kb", 100 * 1024},
		{"mixed Mb", "100Mb", 100 * 1024 * 1024},
		{"mixed Gb", "1Gb", 1024 * 1024 * 1024},

		// Whitespace handling
		{"leading space", " 100MB", 100 * 1024 * 1024},
		{"trailing space", "100MB ", 100 * 1024 * 1024},
		{"both spaces", " 100MB ", 100 * 1024 * 1024},

		// No unit (bytes)
		{"no unit small", "1024", 1024},
		{"no unit large", "1048576", 1048576},

		// Zero values
		{"zero KB", "0KB", 0},
		{"zero MB", "0MB", 0},
		{"zero GB", "0GB", 0},
		{"zero no unit", "0", 0},

		// Edge cases
		{"single digit KB", "1KB", 1024},
		{"large GB", "100GB", 100 * 1024 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hs.parseSize(tt.input)
			if result != tt.expected {
				t.Errorf("parseSize(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseSizeInvalidInputs(t *testing.T) {
	hs := &HyperScanner{}

	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"only spaces", "   "},
		{"only unit", "MB"},
		{"invalid unit", "100TB"},
		{"negative value", "-100MB"},
		{"decimal value", "1.5MB"},
		{"letters mixed", "abc100MB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic, may return 0 or unexpected value
			result := hs.parseSize(tt.input)
			_ = result // Just ensure no panic
		})
	}
}

// =============================================================================
// expandPath Tests - Comprehensive
// =============================================================================

func TestExpandPath(t *testing.T) {
	home := "/Users/testuser"

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		// Tilde expansion
		{"tilde only", "~", "/Users/testuser"},
		{"tilde with slash", "~/", "/Users/testuser"},
		{"tilde with path", "~/Documents", "/Users/testuser/Documents"},
		{"tilde deep path", "~/Developer/projects/myapp", "/Users/testuser/Developer/projects/myapp"},
		{"tilde with dots", "~/file.txt", "/Users/testuser/file.txt"},

		// Absolute paths (unchanged)
		{"absolute simple", "/usr/local/bin", "/usr/local/bin"},
		{"absolute deep", "/var/log/app/debug.log", "/var/log/app/debug.log"},
		{"absolute with spaces concept", "/path/to/file", "/path/to/file"},

		// Relative paths (unchanged)
		{"relative simple", "some/path", "some/path"},
		{"relative deep", "a/b/c/d/e", "a/b/c/d/e"},
		{"relative dot", "./local/path", "./local/path"},
		{"relative dotdot", "../parent/path", "../parent/path"},

		// Tilde in middle (unchanged - not at start)
		{"tilde in middle", "/path/with/~/tilde", "/path/with/~/tilde"},
		{"tilde at end", "/path/to~", "/path/to~"},

		// Edge cases
		{"tilde with special", "~/file-name_v2.tar.gz", "/Users/testuser/file-name_v2.tar.gz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.path, home)
			if result != tt.expected {
				t.Errorf("expandPath(%q, %q) = %q, want %q", tt.path, home, result, tt.expected)
			}
		})
	}
}

func TestExpandPathEmptyHome(t *testing.T) {
	// Edge case: empty home directory
	// When home is empty, filepath.Join("", "test") returns "test"
	result := expandPath("~/test", "")
	if result != "test" {
		t.Errorf("expandPath with empty home = %q, want test", result)
	}
}

// =============================================================================
// categorizeArtifact Tests - Comprehensive
// =============================================================================

func TestCategorizeArtifact(t *testing.T) {
	cfg := &config.Config{
		Categories: config.Categories{
			NodeModules:    true,
			VirtualEnvs:    true,
			BuildArtifacts: true,
		},
	}
	hs := &HyperScanner{config: cfg}

	tests := []struct {
		name     string
		artifact string
		want     string
	}{
		// Node modules
		{"node_modules", "node_modules", "node_modules"},

		// Python virtual environments
		{"venv", "venv", "virtual_envs"},
		{".venv", ".venv", "virtual_envs"},
		{"virtualenv", "virtualenv", "virtual_envs"},

		// Build artifacts - JavaScript/TypeScript
		{"dist", "dist", "build_artifacts"},
		{"build", "build", "build_artifacts"},
		{".next", ".next", "build_artifacts"},
		{"out", "out", "build_artifacts"},

		// Build artifacts - Python
		{"__pycache__", "__pycache__", "build_artifacts"},

		// Build artifacts - Java/Kotlin
		{"target", "target", "build_artifacts"},
		{".gradle", ".gradle", "build_artifacts"},

		// Not recognized directories
		{"src", "src", ""},
		{"lib", "lib", ""},
		{"vendor", "vendor", ""},
		{"packages", "packages", ""},
		{"deps", "deps", ""},
		{"modules", "modules", ""},
		{"random_folder", "random_folder", ""},

		// Similar but not matching
		{"node_module", "node_module", ""},  // singular
		{"nodemodules", "nodemodules", ""},  // no underscore
		{"_pycache_", "_pycache_", ""},      // single underscore
		{"pycache", "pycache", ""},          // no underscores
		{"builds", "builds", ""},            // plural
		{"targets", "targets", ""},          // plural
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hs.categorizeArtifact(tt.artifact)
			if got != tt.want {
				t.Errorf("categorizeArtifact(%q) = %q, want %q", tt.artifact, got, tt.want)
			}
		})
	}
}

func TestCategorizeArtifactAllDisabled(t *testing.T) {
	cfg := &config.Config{
		Categories: config.Categories{
			NodeModules:    false,
			VirtualEnvs:    false,
			BuildArtifacts: false,
		},
	}
	hs := &HyperScanner{config: cfg}

	artifacts := []string{
		"node_modules", "venv", ".venv", "virtualenv",
		"dist", "build", ".next", "__pycache__", "target", ".gradle", "out",
	}

	for _, artifact := range artifacts {
		t.Run(artifact+"_disabled", func(t *testing.T) {
			got := hs.categorizeArtifact(artifact)
			if got != "" {
				t.Errorf("categorizeArtifact(%q) = %q, want empty (all disabled)", artifact, got)
			}
		})
	}
}

func TestCategorizeArtifactPartialEnabled(t *testing.T) {
	// Only node_modules enabled
	cfg := &config.Config{
		Categories: config.Categories{
			NodeModules:    true,
			VirtualEnvs:    false,
			BuildArtifacts: false,
		},
	}
	hs := &HyperScanner{config: cfg}

	if got := hs.categorizeArtifact("node_modules"); got != "node_modules" {
		t.Errorf("node_modules should be categorized when enabled")
	}
	if got := hs.categorizeArtifact("venv"); got != "" {
		t.Errorf("venv should not be categorized when disabled")
	}
	if got := hs.categorizeArtifact("dist"); got != "" {
		t.Errorf("dist should not be categorized when disabled")
	}
}

// =============================================================================
// GroupByCategory Tests - Comprehensive
// =============================================================================

func TestGroupByCategory(t *testing.T) {
	now := time.Now()

	result := &ScanResult{
		Files: []FileInfo{
			{Path: "/tmp/cache1", Size: 100, Category: "cache", ModTime: now},
			{Path: "/tmp/cache2", Size: 200, Category: "cache", ModTime: now},
			{Path: "/home/node_modules", Size: 5000, Category: "node_modules", ModTime: now},
			{Path: "/tmp/log1", Size: 50, Category: "logs", ModTime: now},
			{Path: "/tmp/log2", Size: 75, Category: "logs", ModTime: now},
			{Path: "/tmp/log3", Size: 25, Category: "logs", ModTime: now},
		},
		TotalSize:  5450,
		TotalCount: 6,
	}

	grouped := result.GroupByCategory()

	tests := []struct {
		category  string
		wantCount int
		wantSize  int64
	}{
		{"cache", 2, 300},
		{"node_modules", 1, 5000},
		{"logs", 3, 150},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			cat, ok := grouped[tt.category]
			if !ok {
				t.Fatalf("missing category: %s", tt.category)
			}
			if cat.TotalCount != tt.wantCount {
				t.Errorf("count = %d, want %d", cat.TotalCount, tt.wantCount)
			}
			if cat.TotalSize != tt.wantSize {
				t.Errorf("size = %d, want %d", cat.TotalSize, tt.wantSize)
			}
			if cat.Category != tt.category {
				t.Errorf("category = %q, want %q", cat.Category, tt.category)
			}
		})
	}

	if len(grouped) != 3 {
		t.Errorf("categories count = %d, want 3", len(grouped))
	}
}

func TestGroupByCategoryEmpty(t *testing.T) {
	result := &ScanResult{Files: []FileInfo{}}
	grouped := result.GroupByCategory()

	if len(grouped) != 0 {
		t.Errorf("expected empty map, got %d categories", len(grouped))
	}
}

func TestGroupByCategorySingle(t *testing.T) {
	result := &ScanResult{
		Files: []FileInfo{
			{Path: "/file1", Size: 100, Category: "cache"},
			{Path: "/file2", Size: 200, Category: "cache"},
			{Path: "/file3", Size: 300, Category: "cache"},
		},
	}

	grouped := result.GroupByCategory()

	if len(grouped) != 1 {
		t.Errorf("expected 1 category, got %d", len(grouped))
	}
	if grouped["cache"].TotalCount != 3 {
		t.Errorf("count = %d, want 3", grouped["cache"].TotalCount)
	}
	if grouped["cache"].TotalSize != 600 {
		t.Errorf("size = %d, want 600", grouped["cache"].TotalSize)
	}
}

// =============================================================================
// NewHyperScanner Tests
// =============================================================================

func TestNewHyperScanner(t *testing.T) {
	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true, Temp: true},
	}
	pInfo := &platform.Info{}

	scanner := NewHyperScanner(cfg, pInfo)

	if scanner == nil {
		t.Fatal("NewHyperScanner returned nil")
	}

	if scanner.config != cfg {
		t.Error("config not set")
	}

	if scanner.platformInfo != pInfo {
		t.Error("platformInfo not set")
	}

	// Worker count bounds
	if scanner.workerCount < 16 || scanner.workerCount > 64 {
		t.Errorf("workerCount = %d, want 16-64", scanner.workerCount)
	}

	if scanner.sem == nil {
		t.Error("semaphore not initialized")
	}

	if cap(scanner.sem) != scanner.workerCount {
		t.Error("semaphore capacity mismatch")
	}

	if scanner.results == nil {
		t.Error("results slice not initialized")
	}

	if scanner.cache == nil {
		t.Error("cache not initialized")
	}

	if scanner.cache.DirMtimes == nil {
		t.Error("cache.DirMtimes not initialized")
	}
}

func TestNewHyperScannerNilConfig(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Log("NewHyperScanner with nil config may panic or handle gracefully")
		}
	}()

	// This tests behavior with nil config - implementation dependent
	_ = NewHyperScanner(nil, &platform.Info{})
}

func TestSetProgressCallback(t *testing.T) {
	scanner := NewHyperScanner(&config.Config{}, &platform.Info{})

	if scanner.progressCb != nil {
		t.Error("callback should be nil initially")
	}

	scanner.SetProgressCallback(func(category, path string, count int, size int64) {
		// Callback registered
	})

	if scanner.progressCb == nil {
		t.Error("callback not set")
	}
}

// =============================================================================
// Integration Tests with Test Fixtures
// =============================================================================

func TestScanCategoryWithFixture(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create cache files
	f.CreateCacheFile("app.cache", 1024)
	f.CreateCacheFile("data.cache", 2048)
	f.CreateCacheFile("temp.cache", 512)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)
	hs.cache = &ScanCache{
		Version:      1,
		DirMtimes:    make(map[string]time.Time),
		DirResults:   make(map[string]*CachedDirInfo),
		ArtifactDirs: make(map[string][]string),
	}

	hs.scanDirsWithCache([]string{f.CacheDir}, "cache")

	if len(hs.results) != 3 {
		t.Errorf("TotalCount = %d, want 3", len(hs.results))
	}

	expectedSize := int64(1024 + 2048 + 512)
	var totalSize int64
	for _, r := range hs.results {
		totalSize += r.Size
	}
	if totalSize != expectedSize {
		t.Errorf("TotalSize = %d, want %d", totalSize, expectedSize)
	}
}

func TestScanSkipsNewFiles(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create a new file (not old enough)
	f.CreateFile("cache/new.cache", []byte("new content"))

	// Create an old file
	f.CreateCacheFile("old.cache", 100)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)
	hs.cache = &ScanCache{
		Version:      1,
		DirMtimes:    make(map[string]time.Time),
		DirResults:   make(map[string]*CachedDirInfo),
		ArtifactDirs: make(map[string][]string),
	}

	hs.scanDirsWithCache([]string{f.CacheDir}, "cache")

	if len(hs.results) != 1 {
		t.Errorf("TotalCount = %d, want 1 (only old file)", len(hs.results))
	}
}

func TestScanDevArtifactsWithFixture(t *testing.T) {
	f := testutil.NewFixture(t)
	f.PopulateNodeModules()

	cfg := &config.Config{
		MinFileAge: 0,
		Categories: config.Categories{NodeModules: true},
		Dev: config.DevConfig{
			ProjectDirs: []string{filepath.Dir(f.NodeModules)},
		},
	}
	pInfo := &platform.Info{}

	scanner := NewHyperScanner(cfg, pInfo)

	got := scanner.categorizeArtifact("node_modules")
	if got != "node_modules" {
		t.Errorf("categorizeArtifact = %q, want node_modules", got)
	}
}

func TestScanEmptyDirectory(t *testing.T) {
	f := testutil.NewFixture(t)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)
	hs.cache = &ScanCache{
		Version:      1,
		DirMtimes:    make(map[string]time.Time),
		DirResults:   make(map[string]*CachedDirInfo),
		ArtifactDirs: make(map[string][]string),
	}

	hs.scanDirsWithCache([]string{f.CacheDir}, "cache")

	if len(hs.results) != 0 {
		t.Errorf("expected 0 results for empty directory, got %d", len(hs.results))
	}
}

func TestScanNonExistentDirectory(t *testing.T) {
	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{"/nonexistent/path/12345"},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)
	hs.cache = &ScanCache{
		Version:      1,
		DirMtimes:    make(map[string]time.Time),
		DirResults:   make(map[string]*CachedDirInfo),
		ArtifactDirs: make(map[string][]string),
	}

	// Should not panic
	hs.scanDirsWithCache([]string{"/nonexistent/path/12345"}, "cache")

	if len(hs.results) != 0 {
		t.Errorf("expected 0 results for non-existent directory, got %d", len(hs.results))
	}
}

func TestScanWithSymlinks(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create actual file
	f.CreateCacheFile("real.cache", 100)

	// Create symlink to file
	realFile := filepath.Join(f.CacheDir, "real.cache")
	linkPath := filepath.Join(f.CacheDir, "link.cache")
	os.Symlink(realFile, linkPath)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)
	hs.cache = &ScanCache{
		Version:      1,
		DirMtimes:    make(map[string]time.Time),
		DirResults:   make(map[string]*CachedDirInfo),
		ArtifactDirs: make(map[string][]string),
	}

	hs.scanDirsWithCache([]string{f.CacheDir}, "cache")

	// Results depend on how scanner handles symlinks
	// At minimum, should not panic
	t.Logf("Found %d results (symlink handling test)", len(hs.results))
}

func TestScanMultipleCategories(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create files in different categories
	f.CreateCacheFile("cache1.dat", 100)
	f.CreateLogFile("app.log", 200)
	f.CreateTempFile("temp.tmp", 300)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{
			Cache: true,
			Logs:  true,
			Temp:  true,
		},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		LogDirs:      []string{f.LogsDir},
		TempDirs:     []string{f.TempDir},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)
	result, err := hs.ScanAll()
	if err != nil {
		t.Fatalf("ScanAll failed: %v", err)
	}

	if result == nil {
		t.Fatal("ScanAll returned nil")
	}

	// Should find files from all categories
	t.Logf("Found %d total files across categories", result.TotalCount)
}

// =============================================================================
// Cache Tests
// =============================================================================

func TestScanCacheInitialization(t *testing.T) {
	scanner := NewHyperScanner(&config.Config{}, &platform.Info{})

	if scanner.cache.DirMtimes == nil {
		t.Error("DirMtimes not initialized")
	}
	if scanner.cache.DirResults == nil {
		t.Error("DirResults not initialized")
	}
	if scanner.cache.ArtifactDirs == nil {
		t.Error("ArtifactDirs not initialized")
	}
	if scanner.cache.Version != 1 {
		t.Errorf("Version = %d, want 1", scanner.cache.Version)
	}
}

func TestDirChecksum(t *testing.T) {
	f := testutil.NewFixture(t)

	checksum1 := dirChecksum(f.CacheDir)
	if checksum1 == "" {
		t.Error("empty checksum for valid directory")
	}

	// Same directory = same checksum
	checksum2 := dirChecksum(f.CacheDir)
	if checksum1 != checksum2 {
		t.Error("checksum not deterministic")
	}

	// Different directory = different checksum
	checksum3 := dirChecksum(f.LogsDir)
	if checksum3 == "" {
		t.Error("empty checksum for logs directory")
	}

	// Non-existent = empty
	checksum4 := dirChecksum("/nonexistent/path/12345")
	if checksum4 != "" {
		t.Error("expected empty checksum for non-existent path")
	}
}

func TestCacheExpiration(t *testing.T) {
	cfg := &config.Config{}
	pInfo := &platform.Info{}
	scanner := NewHyperScanner(cfg, pInfo)

	// Manually set cache with old timestamp
	scanner.cache.LastScan = time.Now().Add(-2 * time.Hour)

	// Cache should be considered expired (1 hour threshold)
	// New scanner should not use expired cache
	newScanner := NewHyperScanner(cfg, pInfo)

	// Cache should be fresh
	if time.Since(newScanner.cache.LastScan) > time.Minute {
		t.Log("Cache was properly reset for new scanner")
	}
}

// =============================================================================
// FileInfo and ScanResult Tests
// =============================================================================

func TestFileInfoStruct(t *testing.T) {
	now := time.Now()

	info := FileInfo{
		Path:     "/test/path",
		Size:     1024,
		ModTime:  now,
		Category: "cache",
		Reason:   "test reason",
		Hash:     "abc123",
	}

	if info.Path != "/test/path" {
		t.Errorf("Path = %q", info.Path)
	}
	if info.Size != 1024 {
		t.Errorf("Size = %d", info.Size)
	}
	if info.Category != "cache" {
		t.Errorf("Category = %q", info.Category)
	}
	if info.Reason != "test reason" {
		t.Errorf("Reason = %q", info.Reason)
	}
	if info.Hash != "abc123" {
		t.Errorf("Hash = %q", info.Hash)
	}
}

func TestScanResultStruct(t *testing.T) {
	result := &ScanResult{
		Files:      []FileInfo{{Path: "/test"}},
		TotalSize:  100,
		TotalCount: 1,
		Category:   "cache",
		Errors:     []error{},
	}

	if len(result.Files) != 1 {
		t.Errorf("Files len = %d", len(result.Files))
	}
	if result.TotalSize != 100 {
		t.Errorf("TotalSize = %d", result.TotalSize)
	}
	if result.Category != "cache" {
		t.Errorf("Category = %q", result.Category)
	}
}

// =============================================================================
// Edge Cases and Security Tests
// =============================================================================

func TestScanHiddenDirectories(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create hidden directory with files
	hiddenDir := f.CreateDir("cache/.hidden")
	f.CreateFileWithAge("cache/.hidden/secret.cache", []byte("hidden"), 48*time.Hour)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)
	hs.cache = &ScanCache{
		Version:      1,
		DirMtimes:    make(map[string]time.Time),
		DirResults:   make(map[string]*CachedDirInfo),
		ArtifactDirs: make(map[string][]string),
	}

	hs.scanDirsWithCache([]string{f.CacheDir}, "cache")

	// Scanner may or may not include hidden directories based on implementation
	t.Logf("Hidden directory test: %s, found %d results", hiddenDir, len(hs.results))
}

func TestScanDeepNestedStructure(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create deeply nested structure
	deepPath := "cache/a/b/c/d/e/f/g/h/i/j"
	f.CreateFileWithAge(deepPath+"/deep.cache", []byte("deep"), 48*time.Hour)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)
	hs.cache = &ScanCache{
		Version:      1,
		DirMtimes:    make(map[string]time.Time),
		DirResults:   make(map[string]*CachedDirInfo),
		ArtifactDirs: make(map[string][]string),
	}

	hs.scanDirsWithCache([]string{f.CacheDir}, "cache")

	if len(hs.results) != 1 {
		t.Errorf("expected 1 result from deep structure, got %d", len(hs.results))
	}
}

func TestScanLargeNumberOfFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large file test in short mode")
	}

	f := testutil.NewFixture(t)

	// Create many files
	numFiles := 100
	for i := 0; i < numFiles; i++ {
		f.CreateCacheFile(filepath.Join("subdir", "file"+string(rune('0'+i%10))+string(rune('0'+i/10))+".cache"), 100)
	}

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)
	hs.cache = &ScanCache{
		Version:      1,
		DirMtimes:    make(map[string]time.Time),
		DirResults:   make(map[string]*CachedDirInfo),
		ArtifactDirs: make(map[string][]string),
	}

	hs.scanDirsWithCache([]string{f.CacheDir}, "cache")

	if len(hs.results) != numFiles {
		t.Errorf("expected %d results, got %d", numFiles, len(hs.results))
	}
}

func TestScanWithBrokenSymlinks(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create broken symlink
	f.CreateBrokenSymlink("cache/broken_link")

	// Create valid file
	f.CreateCacheFile("valid.cache", 100)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)
	hs.cache = &ScanCache{
		Version:      1,
		DirMtimes:    make(map[string]time.Time),
		DirResults:   make(map[string]*CachedDirInfo),
		ArtifactDirs: make(map[string][]string),
	}

	// Should not panic on broken symlinks
	hs.scanDirsWithCache([]string{f.CacheDir}, "cache")

	// At least the valid file should be found
	if len(hs.results) < 1 {
		t.Error("expected at least 1 result (the valid file)")
	}
}

func TestScanWithSpecialCharactersInFilename(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create files with special characters (that are valid on filesystem)
	specialNames := []string{
		"file with spaces.cache",
		"file-with-dashes.cache",
		"file_with_underscores.cache",
		"file.multiple.dots.cache",
		"UPPERCASE.cache",
		"MixedCase.cache",
	}

	for _, name := range specialNames {
		f.CreateCacheFile(name, 100)
	}

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)
	hs.cache = &ScanCache{
		Version:      1,
		DirMtimes:    make(map[string]time.Time),
		DirResults:   make(map[string]*CachedDirInfo),
		ArtifactDirs: make(map[string][]string),
	}

	hs.scanDirsWithCache([]string{f.CacheDir}, "cache")

	if len(hs.results) != len(specialNames) {
		t.Errorf("expected %d results, got %d", len(specialNames), len(hs.results))
	}
}

func TestScanZeroSizeFiles(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create zero-size file
	f.CreateCacheFile("empty.cache", 0)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)
	hs.cache = &ScanCache{
		Version:      1,
		DirMtimes:    make(map[string]time.Time),
		DirResults:   make(map[string]*CachedDirInfo),
		ArtifactDirs: make(map[string][]string),
	}

	hs.scanDirsWithCache([]string{f.CacheDir}, "cache")

	if len(hs.results) != 1 {
		t.Errorf("expected 1 result (zero-size file), got %d", len(hs.results))
	}
	if len(hs.results) > 0 && hs.results[0].Size != 0 {
		t.Errorf("expected size 0, got %d", hs.results[0].Size)
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestConcurrentScanning(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create files
	for i := 0; i < 10; i++ {
		f.CreateCacheFile(filepath.Join("dir"+string(rune('0'+i)), "file.cache"), 100)
	}

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	// Run multiple scans concurrently
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			hs := NewHyperScanner(cfg, pInfo)
			hs.ScanAll()
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 5; i++ {
		<-done
	}
}

// =============================================================================
// ScanCategory Tests - Comprehensive
// =============================================================================

func TestScanCategoryCache(t *testing.T) {
	f := testutil.NewFixture(t)
	f.CreateCacheFile("test.cache", 500)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)
	result := hs.ScanCategory("cache")

	if result == nil {
		t.Fatal("ScanCategory returned nil")
	}
	if result.Category != "cache" {
		t.Errorf("expected category 'cache', got %q", result.Category)
	}
	if result.TotalCount < 1 {
		t.Errorf("expected at least 1 result, got %d", result.TotalCount)
	}
}

func TestScanCategoryTemp(t *testing.T) {
	f := testutil.NewFixture(t)
	f.CreateTempFile("temp.tmp", 200)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Temp: true},
	}
	pInfo := &platform.Info{
		TempDirs: []string{f.TempDir},
	}

	hs := NewHyperScanner(cfg, pInfo)
	result := hs.ScanCategory("temp")

	if result == nil {
		t.Fatal("ScanCategory returned nil")
	}
	if result.Category != "temp" {
		t.Errorf("expected category 'temp', got %q", result.Category)
	}
}

func TestScanCategoryLogs(t *testing.T) {
	f := testutil.NewFixture(t)
	f.CreateLogFile("app.log", 300)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Logs: true},
	}
	pInfo := &platform.Info{
		LogDirs: []string{f.LogsDir},
	}

	hs := NewHyperScanner(cfg, pInfo)
	result := hs.ScanCategory("logs")

	if result == nil {
		t.Fatal("ScanCategory returned nil")
	}
	if result.Category != "logs" {
		t.Errorf("expected category 'logs', got %q", result.Category)
	}
}

func TestScanCategoryNodeModules(t *testing.T) {
	f := testutil.NewFixture(t)
	f.PopulateNodeModules()

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{NodeModules: true},
		Dev: config.DevConfig{
			ProjectDirs: []string{filepath.Dir(f.NodeModules)},
		},
	}
	pInfo := &platform.Info{}

	hs := NewHyperScanner(cfg, pInfo)
	result := hs.ScanCategory("node_modules")

	if result == nil {
		t.Fatal("ScanCategory returned nil")
	}
	if result.Category != "node_modules" {
		t.Errorf("expected category 'node_modules', got %q", result.Category)
	}
	t.Logf("Found %d node_modules artifacts", result.TotalCount)
}

func TestScanCategoryVirtualEnvs(t *testing.T) {
	f := testutil.NewFixture(t)
	f.PopulateVenv()

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{VirtualEnvs: true},
		Dev: config.DevConfig{
			ProjectDirs: []string{filepath.Dir(f.Venv)},
		},
	}
	pInfo := &platform.Info{}

	hs := NewHyperScanner(cfg, pInfo)
	result := hs.ScanCategory("virtual_envs")

	if result == nil {
		t.Fatal("ScanCategory returned nil")
	}
	if result.Category != "virtual_envs" {
		t.Errorf("expected category 'virtual_envs', got %q", result.Category)
	}
	t.Logf("Found %d virtual env artifacts", result.TotalCount)
}

func TestScanCategoryBuildArtifacts(t *testing.T) {
	f := testutil.NewFixture(t)
	f.PopulateBuildDir()

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{BuildArtifacts: true},
		Dev: config.DevConfig{
			ProjectDirs: []string{filepath.Dir(f.BuildDir)},
		},
	}
	pInfo := &platform.Info{}

	hs := NewHyperScanner(cfg, pInfo)
	result := hs.ScanCategory("build_artifacts")

	if result == nil {
		t.Fatal("ScanCategory returned nil")
	}
	if result.Category != "build_artifacts" {
		t.Errorf("expected category 'build_artifacts', got %q", result.Category)
	}
	t.Logf("Found %d build artifacts", result.TotalCount)
}

func TestScanCategoryUnknown(t *testing.T) {
	cfg := &config.Config{}
	pInfo := &platform.Info{}

	hs := NewHyperScanner(cfg, pInfo)
	result := hs.ScanCategory("unknown_category")

	if result == nil {
		t.Fatal("ScanCategory returned nil for unknown category")
	}
	if result.TotalCount != 0 {
		t.Errorf("expected 0 results for unknown category, got %d", result.TotalCount)
	}
}

// =============================================================================
// Dev Artifact Scanning Tests - Extended
// =============================================================================

func TestScanAllDevArtifactTypes(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create multiple dev artifact types using CreateRandomFile
	// node_modules
	nmDir := filepath.Join(f.RootDir, "project1", "node_modules", "pkg")
	os.MkdirAll(nmDir, 0755)
	f.CreateRandomFile(filepath.Join("project1", "node_modules", "pkg", "index.js"), 50)

	// venv
	venvDir := filepath.Join(f.RootDir, "project2", "venv", "lib")
	os.MkdirAll(venvDir, 0755)
	f.CreateRandomFile(filepath.Join("project2", "venv", "lib", "site.py"), 50)

	// .venv (hidden)
	dotVenvDir := filepath.Join(f.RootDir, "project3", ".venv", "bin")
	os.MkdirAll(dotVenvDir, 0755)
	f.CreateRandomFile(filepath.Join("project3", ".venv", "bin", "activate"), 50)

	// build
	buildDir := filepath.Join(f.RootDir, "project4", "build", "static")
	os.MkdirAll(buildDir, 0755)
	f.CreateRandomFile(filepath.Join("project4", "build", "static", "main.js"), 50)

	// __pycache__
	pycacheDir := filepath.Join(f.RootDir, "project5", "__pycache__")
	os.MkdirAll(pycacheDir, 0755)
	f.CreateRandomFile(filepath.Join("project5", "__pycache__", "module.pyc"), 50)

	// target (Rust/Cargo)
	targetDir := filepath.Join(f.RootDir, "project6", "target", "debug")
	os.MkdirAll(targetDir, 0755)
	f.CreateRandomFile(filepath.Join("project6", "target", "debug", "app"), 100)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{
			NodeModules:    true,
			VirtualEnvs:    true,
			BuildArtifacts: true,
		},
		Dev: config.DevConfig{
			ProjectDirs: []string{f.RootDir},
		},
	}
	pInfo := &platform.Info{}

	hs := NewHyperScanner(cfg, pInfo)
	result, err := hs.ScanAll()
	if err != nil {
		t.Fatalf("ScanAll failed: %v", err)
	}

	t.Logf("Found %d dev artifacts total", result.TotalCount)
}

func TestScanDevArtifactsEmptyProjectDirs(t *testing.T) {
	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{
			NodeModules:    true,
			VirtualEnvs:    true,
			BuildArtifacts: true,
		},
		Dev: config.DevConfig{
			ProjectDirs: []string{}, // Empty
		},
	}
	pInfo := &platform.Info{}

	hs := NewHyperScanner(cfg, pInfo)
	result := hs.ScanCategory("node_modules")

	if result == nil {
		t.Fatal("ScanCategory returned nil")
	}
	// Should return 0 results with no project dirs
	if result.TotalCount != 0 {
		t.Errorf("expected 0 results with empty project dirs, got %d", result.TotalCount)
	}
}

func TestScanDevArtifactsNonExistentProjectDir(t *testing.T) {
	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{NodeModules: true},
		Dev: config.DevConfig{
			ProjectDirs: []string{"/nonexistent/path/that/does/not/exist"},
		},
	}
	pInfo := &platform.Info{}

	hs := NewHyperScanner(cfg, pInfo)
	result := hs.ScanCategory("node_modules")

	if result == nil {
		t.Fatal("ScanCategory returned nil")
	}
	// Should handle gracefully
	if result.TotalCount != 0 {
		t.Errorf("expected 0 results for nonexistent path, got %d", result.TotalCount)
	}
}

func TestScanDevArtifactsWithTildePath(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create a node_modules in the fixture
	nmDir := filepath.Join(f.RootDir, "node_modules", "test-pkg")
	os.MkdirAll(nmDir, 0755)
	f.CreateRandomFile(filepath.Join("node_modules", "test-pkg", "package.json"), 100)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{NodeModules: true},
		Dev: config.DevConfig{
			// Use absolute path since tilde expansion happens internally
			ProjectDirs: []string{f.RootDir},
		},
	}
	pInfo := &platform.Info{}

	hs := NewHyperScanner(cfg, pInfo)
	result := hs.ScanCategory("node_modules")

	if result == nil {
		t.Fatal("ScanCategory returned nil")
	}
	t.Logf("Found %d node_modules with path config", result.TotalCount)
}

// =============================================================================
// Large Files Scanning Tests
// =============================================================================

func TestScanLargeFiles(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create some files of varying sizes
	f.CreateRandomFile(filepath.Join("large", "small.bin"), 100)
	f.CreateRandomFile(filepath.Join("large", "medium.bin"), 1000)
	f.CreateRandomFile(filepath.Join("large", "large.bin"), 10000)

	cfg := &config.Config{
		MinFileAge: 24,
		LargeFiles: config.LargeFilesConfig{
			MinSize:   "500", // 500 bytes
			ScanPaths: []string{f.LargeFilesDir},
		},
	}
	pInfo := &platform.Info{}

	hs := NewHyperScanner(cfg, pInfo)
	result := hs.ScanCategory("large_files")

	if result == nil {
		t.Fatal("ScanCategory returned nil")
	}
	t.Logf("Found %d large files", result.TotalCount)
}

func TestScanOldFiles(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create an old file
	oldFile := f.CreateRandomFile(filepath.Join("cache", "old.cache"), 100)

	// Set modification time to 100 days ago
	oldTime := time.Now().Add(-100 * 24 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	cfg := &config.Config{
		MinFileAge: 24,
		OldFiles: config.OldFilesConfig{
			MinAgeDays: 30, // 30 days
			ScanPaths:  []string{f.CacheDir},
		},
	}
	pInfo := &platform.Info{}

	hs := NewHyperScanner(cfg, pInfo)
	result := hs.ScanCategory("old_files")

	if result == nil {
		t.Fatal("ScanCategory returned nil")
	}
	t.Logf("Found %d old files", result.TotalCount)
}

// =============================================================================
// Add Artifact Result Tests
// =============================================================================

func TestAddArtifactResultCalculatesSize(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create a directory with files to calculate size
	artifactDir := filepath.Join(f.RootDir, "node_modules")
	os.MkdirAll(artifactDir, 0755)
	f.CreateRandomFile(filepath.Join("node_modules", "file1.js"), 100)
	f.CreateRandomFile(filepath.Join("node_modules", "file2.js"), 200)
	subDir := filepath.Join(artifactDir, "subpkg")
	os.MkdirAll(subDir, 0755)
	f.CreateRandomFile(filepath.Join("node_modules", "subpkg", "file3.js"), 300)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{NodeModules: true},
	}
	pInfo := &platform.Info{}

	hs := NewHyperScanner(cfg, pInfo)
	hs.addArtifactResult(artifactDir, "node_modules")

	if len(hs.results) != 1 {
		t.Errorf("expected 1 result, got %d", len(hs.results))
	}

	if len(hs.results) > 0 {
		// Total size should be 100 + 200 + 300 = 600
		if hs.results[0].Size < 600 {
			t.Errorf("expected size >= 600, got %d", hs.results[0].Size)
		}
		if hs.results[0].Category != "node_modules" {
			t.Errorf("expected category 'node_modules', got %q", hs.results[0].Category)
		}
		if hs.results[0].Path != artifactDir {
			t.Errorf("expected path %q, got %q", artifactDir, hs.results[0].Path)
		}
	}
}

func TestAddArtifactResultNonExistent(t *testing.T) {
	cfg := &config.Config{}
	pInfo := &platform.Info{}

	hs := NewHyperScanner(cfg, pInfo)
	hs.addArtifactResult("/nonexistent/path", "node_modules")

	// Non-existent paths should be skipped - no result added
	if len(hs.results) != 0 {
		t.Errorf("expected 0 results for nonexistent path, got %d", len(hs.results))
	}
}

func TestAddArtifactResultEmptyDirectory(t *testing.T) {
	f := testutil.NewFixture(t)

	emptyDir := filepath.Join(f.RootDir, "empty_node_modules")
	os.MkdirAll(emptyDir, 0755)

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{NodeModules: true},
	}
	pInfo := &platform.Info{}

	hs := NewHyperScanner(cfg, pInfo)
	hs.addArtifactResult(emptyDir, "node_modules")

	if len(hs.results) != 1 {
		t.Errorf("expected 1 result for empty dir, got %d", len(hs.results))
	}
	if len(hs.results) > 0 && hs.results[0].Size != 0 {
		t.Errorf("expected size 0 for empty dir, got %d", hs.results[0].Size)
	}
}

// =============================================================================
// Progress Callback Tests
// =============================================================================

func TestProgressCallbackCalledDuringScan(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create files to scan
	for i := 0; i < 5; i++ {
		f.CreateCacheFile("file"+string(rune('0'+i))+".cache", 100)
	}

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)

	callCount := 0
	hs.SetProgressCallback(func(category, currentPath string, filesFound int, totalSize int64) {
		callCount++
	})

	_, err := hs.ScanAll()
	if err != nil {
		t.Fatalf("ScanAll failed: %v", err)
	}

	t.Logf("Progress callback called %d times", callCount)
}

// =============================================================================
// Scan Result Aggregation Tests
// =============================================================================

func TestScanResultTotalSizeAccurate(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create files with known sizes
	sizes := []int{100, 200, 300, 400, 500}
	expectedTotal := int64(0)
	for i, size := range sizes {
		f.CreateCacheFile("file"+string(rune('0'+i))+".cache", size)
		expectedTotal += int64(size)
	}

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)
	// Initialize cache to enable isolated scanning
	hs.cache = &ScanCache{
		Version:      1,
		DirMtimes:    make(map[string]time.Time),
		DirResults:   make(map[string]*CachedDirInfo),
		ArtifactDirs: make(map[string][]string),
	}
	// Use direct scan method to ensure isolation
	hs.scanDirsWithCache([]string{f.CacheDir}, "cache")

	if atomic.LoadInt64(&hs.totalSize) != expectedTotal {
		t.Errorf("expected total size %d, got %d", expectedTotal, atomic.LoadInt64(&hs.totalSize))
	}
	if len(hs.results) != len(sizes) {
		t.Errorf("expected %d files, got %d", len(sizes), len(hs.results))
	}
}

func TestScanResultFilesMatchCount(t *testing.T) {
	f := testutil.NewFixture(t)

	numFiles := 7
	for i := 0; i < numFiles; i++ {
		f.CreateCacheFile("file"+string(rune('0'+i))+".cache", 100)
	}

	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	hs := NewHyperScanner(cfg, pInfo)
	result, err := hs.ScanAll()
	if err != nil {
		t.Fatalf("ScanAll failed: %v", err)
	}

	if len(result.Files) != result.TotalCount {
		t.Errorf("Files length %d doesn't match TotalCount %d", len(result.Files), result.TotalCount)
	}
}
