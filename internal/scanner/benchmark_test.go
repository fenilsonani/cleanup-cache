package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fenilsonani/system-cleanup/internal/config"
	"github.com/fenilsonani/system-cleanup/internal/platform"
	"github.com/fenilsonani/system-cleanup/internal/testutil"
)

// =============================================================================
// Scanner Benchmarks
// =============================================================================

func BenchmarkParseSize(b *testing.B) {
	hs := &HyperScanner{}
	inputs := []string{"1KB", "100MB", "2GB", "500MB", "10GB"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			hs.parseSize(input)
		}
	}
}

func BenchmarkExpandPath(b *testing.B) {
	home := "/Users/testuser"
	paths := []string{
		"~/Documents",
		"~/projects/app",
		"/absolute/path",
		"relative/path",
		"~/deep/nested/path/to/file",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range paths {
			expandPath(p, home)
		}
	}
}

func BenchmarkCategorizeArtifact(b *testing.B) {
	cfg := &config.Config{
		Categories: config.Categories{
			NodeModules:    true,
			VirtualEnvs:    true,
			BuildArtifacts: true,
		},
	}
	hs := &HyperScanner{config: cfg}

	artifacts := []string{
		"node_modules",
		"venv",
		".venv",
		"virtualenv",
		"dist",
		"build",
		".next",
		"__pycache__",
		"target",
		".gradle",
		"out",
		"random_folder",
		"src",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, a := range artifacts {
			hs.categorizeArtifact(a)
		}
	}
}

func BenchmarkGroupByCategory(b *testing.B) {
	result := &ScanResult{
		Files: []FileInfo{
			{Path: "/cache/file1", Size: 100, Category: "cache"},
			{Path: "/cache/file2", Size: 200, Category: "cache"},
			{Path: "/tmp/file1", Size: 150, Category: "temp"},
			{Path: "/logs/file1", Size: 300, Category: "logs"},
			{Path: "/nm/pkg1", Size: 1000, Category: "node_modules"},
			{Path: "/nm/pkg2", Size: 2000, Category: "node_modules"},
			{Path: "/venv/lib", Size: 500, Category: "venv"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result.GroupByCategory()
	}
}

func BenchmarkDirChecksum(b *testing.B) {
	tmpDir := b.TempDir()

	// Create some files
	for i := 0; i < 10; i++ {
		f, _ := os.Create(filepath.Join(tmpDir, "file"+string(rune('0'+i))))
		f.Write(make([]byte, 100))
		f.Close()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dirChecksum(tmpDir)
	}
}

func BenchmarkNewHyperScanner(b *testing.B) {
	cfg := &config.Config{
		MinFileAge: 24,
		Categories: config.Categories{
			Cache:          true,
			Temp:           true,
			Logs:           true,
			NodeModules:    true,
			VirtualEnvs:    true,
			BuildArtifacts: true,
		},
	}
	pInfo := &platform.Info{
		CacheDirs: []string{"/tmp/cache"},
		TempDirs:  []string{"/tmp/temp"},
		LogDirs:   []string{"/tmp/logs"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewHyperScanner(cfg, pInfo)
	}
}

// =============================================================================
// Scan Operation Benchmarks
// =============================================================================

func BenchmarkScanSmallDirectory(b *testing.B) {
	f := testutil.NewFixture(&testing.T{})

	// Create 10 files
	for i := 0; i < 10; i++ {
		f.CreateCacheFile("file"+string(rune('0'+i))+".cache", 100)
	}

	cfg := &config.Config{
		MinFileAge: 0, // Include all files
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hs := NewHyperScanner(cfg, pInfo)
		hs.ScanCategory("cache")
	}
}

func BenchmarkScanMediumDirectory(b *testing.B) {
	f := testutil.NewFixture(&testing.T{})

	// Create 100 files
	for i := 0; i < 100; i++ {
		name := filepath.Join("subdir"+string(rune('0'+i/10)), "file"+string(rune('0'+i%10))+".cache")
		f.CreateRandomFile(filepath.Join("cache", name), 100)
	}

	cfg := &config.Config{
		MinFileAge: 0,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hs := NewHyperScanner(cfg, pInfo)
		hs.ScanCategory("cache")
	}
}

func BenchmarkScanWithCache(b *testing.B) {
	f := testutil.NewFixture(&testing.T{})

	// Create files
	for i := 0; i < 50; i++ {
		f.CreateCacheFile("file"+string(rune('0'+i/10))+string(rune('0'+i%10))+".cache", 100)
	}

	cfg := &config.Config{
		MinFileAge: 0,
		Categories: config.Categories{Cache: true},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		SystemCaches: []string{},
	}

	// First scan to populate cache
	hs := NewHyperScanner(cfg, pInfo)
	hs.ScanCategory("cache")

	b.ResetTimer()
	// Subsequent scans should use cache
	for i := 0; i < b.N; i++ {
		hs.ScanCategory("cache")
	}
}

func BenchmarkScanMultipleCategories(b *testing.B) {
	f := testutil.NewFixture(&testing.T{})

	// Create files in multiple directories
	for i := 0; i < 20; i++ {
		f.CreateCacheFile("cache"+string(rune('0'+i))+".cache", 100)
		f.CreateTempFile("temp"+string(rune('0'+i))+".tmp", 100)
		f.CreateLogFile("log"+string(rune('0'+i))+".log", 100)
	}

	cfg := &config.Config{
		MinFileAge: 0,
		Categories: config.Categories{
			Cache: true,
			Temp:  true,
			Logs:  true,
		},
	}
	pInfo := &platform.Info{
		CacheDirs:    []string{f.CacheDir},
		TempDirs:     []string{f.TempDir},
		LogDirs:      []string{f.LogsDir},
		SystemCaches: []string{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hs := NewHyperScanner(cfg, pInfo)
		hs.ScanAll()
	}
}

// =============================================================================
// Memory Allocation Benchmarks
// =============================================================================

func BenchmarkParseSizeAllocs(b *testing.B) {
	hs := &HyperScanner{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hs.parseSize("500MB")
	}
}

func BenchmarkExpandPathAllocs(b *testing.B) {
	home := "/Users/testuser"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expandPath("~/projects/app", home)
	}
}

func BenchmarkCategorizeArtifactAllocs(b *testing.B) {
	cfg := &config.Config{
		Categories: config.Categories{
			NodeModules:    true,
			VirtualEnvs:    true,
			BuildArtifacts: true,
		},
	}
	hs := &HyperScanner{config: cfg}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hs.categorizeArtifact("node_modules")
	}
}

// =============================================================================
// Parallel Benchmarks
// =============================================================================

func BenchmarkParseSizeParallel(b *testing.B) {
	hs := &HyperScanner{}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			hs.parseSize("100MB")
		}
	})
}

func BenchmarkCategorizeArtifactParallel(b *testing.B) {
	cfg := &config.Config{
		Categories: config.Categories{
			NodeModules:    true,
			VirtualEnvs:    true,
			BuildArtifacts: true,
		},
	}
	hs := &HyperScanner{config: cfg}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			hs.categorizeArtifact("node_modules")
		}
	})
}
