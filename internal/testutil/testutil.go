// Package testutil provides test helpers and fixtures for tidyup tests.
// All file operations use t.TempDir() for safe, isolated testing.
package testutil

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestFixture holds paths to test directories and files
type TestFixture struct {
	T       *testing.T
	RootDir string // Root temp directory (auto-cleaned)

	// Standard test directories
	CacheDir      string
	TempDir       string
	LogsDir       string
	NodeModules   string
	Venv          string
	BuildDir      string
	LargeFilesDir string
}

// NewFixture creates a new test fixture with standard directory structure
func NewFixture(t *testing.T) *TestFixture {
	t.Helper()

	root := t.TempDir()

	f := &TestFixture{
		T:             t,
		RootDir:       root,
		CacheDir:      filepath.Join(root, "cache"),
		TempDir:       filepath.Join(root, "tmp"),
		LogsDir:       filepath.Join(root, "logs"),
		NodeModules:   filepath.Join(root, "project", "node_modules"),
		Venv:          filepath.Join(root, "project", "venv"),
		BuildDir:      filepath.Join(root, "project", "dist"),
		LargeFilesDir: filepath.Join(root, "large"),
	}

	// Create all directories
	dirs := []string{
		f.CacheDir,
		f.TempDir,
		f.LogsDir,
		f.NodeModules,
		f.Venv,
		f.BuildDir,
		f.LargeFilesDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	return f
}

// =============================================================================
// File Creation Helpers
// =============================================================================

// CreateFile creates a file with specified content and returns its path
func (f *TestFixture) CreateFile(relPath string, content []byte) string {
	f.T.Helper()

	fullPath := filepath.Join(f.RootDir, relPath)
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		f.T.Fatalf("failed to create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		f.T.Fatalf("failed to create file %s: %v", fullPath, err)
	}

	return fullPath
}

// CreateFileWithAge creates a file and sets its modification time to the past
func (f *TestFixture) CreateFileWithAge(relPath string, content []byte, age time.Duration) string {
	f.T.Helper()

	fullPath := f.CreateFile(relPath, content)
	oldTime := time.Now().Add(-age)

	if err := os.Chtimes(fullPath, oldTime, oldTime); err != nil {
		f.T.Fatalf("failed to set file time for %s: %v", fullPath, err)
	}

	return fullPath
}

// CreateCacheFile creates a file in the cache directory
func (f *TestFixture) CreateCacheFile(name string, size int) string {
	f.T.Helper()
	content := make([]byte, size)
	return f.CreateFileWithAge(filepath.Join("cache", name), content, 48*time.Hour)
}

// CreateTempFile creates a file in the temp directory
func (f *TestFixture) CreateTempFile(name string, size int) string {
	f.T.Helper()
	content := make([]byte, size)
	return f.CreateFileWithAge(filepath.Join("tmp", name), content, 48*time.Hour)
}

// CreateLogFile creates a file in the logs directory
func (f *TestFixture) CreateLogFile(name string, size int) string {
	f.T.Helper()
	content := make([]byte, size)
	return f.CreateFileWithAge(filepath.Join("logs", name), content, 48*time.Hour)
}

// CreateLargeFile creates a large file for testing large file detection
func (f *TestFixture) CreateLargeFile(name string, sizeMB int) string {
	f.T.Helper()
	content := make([]byte, sizeMB*1024*1024)
	return f.CreateFileWithAge(filepath.Join("large", name), content, 48*time.Hour)
}

// CreateRandomFile creates a file with random content
func (f *TestFixture) CreateRandomFile(relPath string, size int) string {
	f.T.Helper()
	content := make([]byte, size)
	rand.Read(content)
	return f.CreateFile(relPath, content)
}

// =============================================================================
// Directory Helpers
// =============================================================================

// CreateDir creates a directory and returns its path
func (f *TestFixture) CreateDir(relPath string) string {
	f.T.Helper()

	fullPath := filepath.Join(f.RootDir, relPath)
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		f.T.Fatalf("failed to create directory %s: %v", fullPath, err)
	}

	return fullPath
}

// CreateDirWithMode creates a directory with specific permissions
func (f *TestFixture) CreateDirWithMode(relPath string, mode os.FileMode) string {
	f.T.Helper()

	fullPath := filepath.Join(f.RootDir, relPath)
	if err := os.MkdirAll(fullPath, mode); err != nil {
		f.T.Fatalf("failed to create directory %s: %v", fullPath, err)
	}

	// Set mode explicitly (MkdirAll might be affected by umask)
	if err := os.Chmod(fullPath, mode); err != nil {
		f.T.Fatalf("failed to chmod directory %s: %v", fullPath, err)
	}

	return fullPath
}

// CreateDirWithAge creates a directory with a specific modification time
func (f *TestFixture) CreateDirWithAge(relPath string, age time.Duration) string {
	f.T.Helper()

	fullPath := f.CreateDir(relPath)
	oldTime := time.Now().Add(-age)

	if err := os.Chtimes(fullPath, oldTime, oldTime); err != nil {
		f.T.Fatalf("failed to set dir time for %s: %v", fullPath, err)
	}

	return fullPath
}

// =============================================================================
// Symlink Helpers
// =============================================================================

// CreateSymlink creates a symbolic link
func (f *TestFixture) CreateSymlink(target, linkPath string) string {
	f.T.Helper()

	fullLinkPath := filepath.Join(f.RootDir, linkPath)
	dir := filepath.Dir(fullLinkPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		f.T.Fatalf("failed to create directory %s: %v", dir, err)
	}

	if err := os.Symlink(target, fullLinkPath); err != nil {
		f.T.Fatalf("failed to create symlink %s -> %s: %v", fullLinkPath, target, err)
	}

	return fullLinkPath
}

// CreateBrokenSymlink creates a symlink pointing to a non-existent target
func (f *TestFixture) CreateBrokenSymlink(linkPath string) string {
	f.T.Helper()
	return f.CreateSymlink("/nonexistent/target/"+randomString(8), linkPath)
}

// CreateSymlinkChain creates a chain of symlinks: link1 -> link2 -> ... -> target
func (f *TestFixture) CreateSymlinkChain(target string, links ...string) string {
	f.T.Helper()

	currentTarget := target
	for i := len(links) - 1; i >= 0; i-- {
		linkPath := f.CreateSymlink(currentTarget, links[i])
		currentTarget = linkPath
	}

	return currentTarget
}

// CreateCircularSymlinks creates two symlinks that point to each other
func (f *TestFixture) CreateCircularSymlinks(link1, link2 string) (string, string) {
	f.T.Helper()

	fullLink1 := filepath.Join(f.RootDir, link1)
	fullLink2 := filepath.Join(f.RootDir, link2)

	dir1 := filepath.Dir(fullLink1)
	dir2 := filepath.Dir(fullLink2)

	os.MkdirAll(dir1, 0755)
	os.MkdirAll(dir2, 0755)

	// Create link1 pointing to link2
	os.Symlink(fullLink2, fullLink1)
	// Create link2 pointing to link1
	os.Symlink(fullLink1, fullLink2)

	return fullLink1, fullLink2
}

// =============================================================================
// Permission Helpers
// =============================================================================

// CreateFileWithMode creates a file with specific permissions
func (f *TestFixture) CreateFileWithMode(relPath string, content []byte, mode os.FileMode) string {
	f.T.Helper()

	fullPath := f.CreateFile(relPath, content)
	if err := os.Chmod(fullPath, mode); err != nil {
		f.T.Fatalf("failed to chmod file %s: %v", fullPath, err)
	}

	return fullPath
}

// CreateReadOnlyFile creates a file that cannot be written to
func (f *TestFixture) CreateReadOnlyFile(relPath string, content []byte) string {
	f.T.Helper()
	return f.CreateFileWithMode(relPath, content, 0444)
}

// CreateNoPermissionFile creates a file with no permissions (000)
func (f *TestFixture) CreateNoPermissionFile(relPath string, content []byte) string {
	f.T.Helper()
	return f.CreateFileWithMode(relPath, content, 0000)
}

// CreateReadOnlyDir creates a read-only directory (files inside can't be deleted)
func (f *TestFixture) CreateReadOnlyDir(relPath string) string {
	f.T.Helper()

	dirPath := f.CreateDir(relPath)
	// Create a file inside first
	f.CreateFile(filepath.Join(relPath, "trapped.txt"), []byte("trapped"))
	// Then make directory read-only
	if err := os.Chmod(dirPath, 0555); err != nil {
		f.T.Fatalf("failed to chmod directory %s: %v", dirPath, err)
	}

	// Register cleanup to restore permissions so TempDir cleanup works
	f.T.Cleanup(func() {
		os.Chmod(dirPath, 0755)
	})

	return dirPath
}

// =============================================================================
// Path Helpers
// =============================================================================

// Path returns the full path for a relative path within the fixture
func (f *TestFixture) Path(relPath string) string {
	return filepath.Join(f.RootDir, relPath)
}

// RelPath returns the relative path from the fixture root
func (f *TestFixture) RelPath(fullPath string) string {
	rel, _ := filepath.Rel(f.RootDir, fullPath)
	return rel
}

// =============================================================================
// Assertion Helpers
// =============================================================================

// FileExists checks if a file exists
func (f *TestFixture) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// AssertFileExists fails the test if the file doesn't exist
func (f *TestFixture) AssertFileExists(path string) {
	f.T.Helper()
	if !f.FileExists(path) {
		f.T.Errorf("expected file to exist: %s", path)
	}
}

// AssertFileNotExists fails the test if the file exists
func (f *TestFixture) AssertFileNotExists(path string) {
	f.T.Helper()
	if f.FileExists(path) {
		f.T.Errorf("expected file to not exist: %s", path)
	}
}

// AssertIsSymlink fails if path is not a symlink
func (f *TestFixture) AssertIsSymlink(path string) {
	f.T.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		f.T.Errorf("failed to stat %s: %v", path, err)
		return
	}
	if info.Mode()&os.ModeSymlink == 0 {
		f.T.Errorf("expected %s to be a symlink", path)
	}
}

// AssertFileMode checks if file has expected permissions
func (f *TestFixture) AssertFileMode(path string, expectedMode os.FileMode) {
	f.T.Helper()
	info, err := os.Stat(path)
	if err != nil {
		f.T.Errorf("failed to stat %s: %v", path, err)
		return
	}
	actualMode := info.Mode().Perm()
	if actualMode != expectedMode {
		f.T.Errorf("file %s has mode %o, want %o", path, actualMode, expectedMode)
	}
}

// AssertFileSize checks if file has expected size
func (f *TestFixture) AssertFileSize(path string, expectedSize int64) {
	f.T.Helper()
	info, err := os.Stat(path)
	if err != nil {
		f.T.Errorf("failed to stat %s: %v", path, err)
		return
	}
	if info.Size() != expectedSize {
		f.T.Errorf("file %s has size %d, want %d", path, info.Size(), expectedSize)
	}
}

// =============================================================================
// Dev Artifact Helpers
// =============================================================================

// PopulateNodeModules creates a mock node_modules structure
func (f *TestFixture) PopulateNodeModules() {
	f.T.Helper()

	packages := []string{"lodash", "express", "react", ".bin"}
	for _, pkg := range packages {
		pkgDir := filepath.Join(f.NodeModules, pkg)
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			f.T.Fatalf("failed to create package dir: %v", err)
		}

		pkgJSON := filepath.Join(pkgDir, "package.json")
		content := []byte(`{"name": "` + pkg + `", "version": "1.0.0"}`)
		if err := os.WriteFile(pkgJSON, content, 0644); err != nil {
			f.T.Fatalf("failed to create package.json: %v", err)
		}
	}
}

// PopulateVenv creates a mock Python venv structure
func (f *TestFixture) PopulateVenv() {
	f.T.Helper()

	dirs := []string{
		filepath.Join(f.Venv, "bin"),
		filepath.Join(f.Venv, "lib", "python3.11", "site-packages"),
		filepath.Join(f.Venv, "include"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			f.T.Fatalf("failed to create venv dir: %v", err)
		}
	}

	cfg := filepath.Join(f.Venv, "pyvenv.cfg")
	if err := os.WriteFile(cfg, []byte("home = /usr/bin\nversion = 3.11.0\n"), 0644); err != nil {
		f.T.Fatalf("failed to create pyvenv.cfg: %v", err)
	}
}

// PopulateBuildDir creates a mock build/dist directory
func (f *TestFixture) PopulateBuildDir() {
	f.T.Helper()

	files := []string{
		"index.js",
		"index.js.map",
		"styles.css",
		"assets/logo.png",
	}

	for _, file := range files {
		f.CreateFile(filepath.Join("project", "dist", file), []byte("build output"))
	}
}

// =============================================================================
// Security Test Helpers
// =============================================================================

// CreatePathTraversalAttempt creates files that could be used in path traversal attacks
func (f *TestFixture) CreatePathTraversalAttempt() map[string]string {
	f.T.Helper()

	attempts := make(map[string]string)

	// Create a target file outside the "safe" directory
	targetFile := f.CreateFile("sensitive/secret.txt", []byte("sensitive data"))
	attempts["target"] = targetFile

	// Create a directory that looks safe
	safeDir := f.CreateDir("safe")
	attempts["safe_dir"] = safeDir

	// Note: We can't actually create malicious symlinks pointing outside temp dir
	// in a safe way, but we can create test scenarios
	return attempts
}

// DangerousPathPatterns returns paths that should be rejected by security checks
func DangerousPathPatterns() []string {
	return []string{
		"../../../etc/passwd",
		"/etc/passwd",
		"file\x00.txt",      // null byte injection
		"file\n.txt",        // newline injection
		"file; rm -rf /",    // command injection attempt
		"file | cat /etc/passwd",
		"file$(whoami)",
		"file`whoami`",
		"/",
		"/bin",
		"/usr",
		"/etc",
		"/System",
		"/Applications",
	}
}

// SafePathPatterns returns paths that should be accepted
func SafePathPatterns() []string {
	return []string{
		"/tmp/cache/file.txt",
		"/home/user/.cache/app/data",
		"/Users/test/Library/Caches/com.app/cache.db",
		"/var/tmp/cleanup-12345",
	}
}

// =============================================================================
// Utility Functions
// =============================================================================

// GetDirSize returns the total size of all files in a directory
func GetDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// CountFiles returns the number of files in a directory (recursive)
func CountFiles(path string) (int, error) {
	var count int
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}

// IsRoot returns true if running as root/admin
func IsRoot() bool {
	return os.Geteuid() == 0
}

// SkipIfRoot skips the test if running as root
func SkipIfRoot(t *testing.T) {
	t.Helper()
	if IsRoot() {
		t.Skip("skipping test when running as root")
	}
}

// SkipIfNotRoot skips the test if not running as root
func SkipIfNotRoot(t *testing.T) {
	t.Helper()
	if !IsRoot() {
		t.Skip("skipping test: requires root")
	}
}

// SkipOnCI skips the test when running in CI environment
func SkipOnCI(t *testing.T) {
	t.Helper()
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		t.Skip("skipping test in CI environment")
	}
}

// IsMacOS returns true if running on macOS
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

// IsLinux returns true if running on Linux
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// randomString generates a random string of specified length
func randomString(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

// =============================================================================
// Test Table Helpers
// =============================================================================

// PathTestCase represents a test case for path validation
type PathTestCase struct {
	Name        string
	Path        string
	ShouldPass  bool
	Description string
}

// StandardPathTestCases returns common test cases for path validation
func StandardPathTestCases() []PathTestCase {
	return []PathTestCase{
		// Valid paths
		{Name: "absolute_path", Path: "/tmp/test.txt", ShouldPass: true, Description: "normal absolute path"},
		{Name: "deep_path", Path: "/home/user/.cache/app/data/file.db", ShouldPass: true, Description: "deep nested path"},
		{Name: "path_with_dots", Path: "/tmp/file.name.with.dots.txt", ShouldPass: true, Description: "filename with dots"},
		{Name: "path_with_dashes", Path: "/tmp/file-name-with-dashes", ShouldPass: true, Description: "filename with dashes"},
		{Name: "path_with_underscores", Path: "/tmp/file_name_with_underscores", ShouldPass: true, Description: "filename with underscores"},

		// Invalid paths - traversal
		{Name: "traversal_simple", Path: "../etc/passwd", ShouldPass: false, Description: "simple path traversal"},
		{Name: "traversal_deep", Path: "../../../../etc/passwd", ShouldPass: false, Description: "deep path traversal"},
		{Name: "traversal_mixed", Path: "/tmp/../../../etc/passwd", ShouldPass: false, Description: "mixed absolute and traversal"},

		// Invalid paths - injection
		{Name: "null_byte", Path: "/tmp/file\x00.txt", ShouldPass: false, Description: "null byte injection"},
		{Name: "newline", Path: "/tmp/file\n.txt", ShouldPass: false, Description: "newline injection"},
		{Name: "semicolon", Path: "/tmp/file;rm -rf /", ShouldPass: false, Description: "command injection with semicolon"},
		{Name: "pipe", Path: "/tmp/file|cat", ShouldPass: false, Description: "pipe injection"},
		{Name: "backtick", Path: "/tmp/`whoami`", ShouldPass: false, Description: "backtick injection"},
		{Name: "dollar_paren", Path: "/tmp/$(whoami)", ShouldPass: false, Description: "dollar paren injection"},

		// Invalid paths - protected
		{Name: "root", Path: "/", ShouldPass: false, Description: "root directory"},
		{Name: "etc", Path: "/etc", ShouldPass: false, Description: "etc directory"},
		{Name: "bin", Path: "/bin", ShouldPass: false, Description: "bin directory"},
		{Name: "usr", Path: "/usr", ShouldPass: false, Description: "usr directory"},
		{Name: "etc_file", Path: "/etc/passwd", ShouldPass: false, Description: "file in etc"},

		// Edge cases
		{Name: "empty", Path: "", ShouldPass: false, Description: "empty path"},
		{Name: "relative", Path: "relative/path", ShouldPass: false, Description: "relative path"},
		{Name: "double_slash", Path: "//tmp//test", ShouldPass: true, Description: "double slashes (should clean)"},
	}
}

// ErrorTestCase represents a test case for error handling
type ErrorTestCase struct {
	Name          string
	InputError    error
	ExpectedType  string
	ShouldRetry   bool
	Description   string
}

// ContainsString checks if a string contains a substring (case-insensitive)
func ContainsString(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
