package cleaner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/fenilsonani/system-cleanup/internal/config"
	"github.com/fenilsonani/system-cleanup/internal/scanner"
	"github.com/fenilsonani/system-cleanup/internal/testutil"
)

// =============================================================================
// Error Categorization Tests - Comprehensive
// =============================================================================

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		wantReason    ErrorReason
		wantRetryable bool
		wantNeedsSudo bool
	}{
		// Nil error
		{"nil error", nil, ErrorUnknown, false, false},

		// Standard errors
		{"os.ErrNotExist", os.ErrNotExist, ErrorFileNotFound, false, false},
		{"os.ErrPermission", os.ErrPermission, ErrorPermissionDenied, false, true},
		{"os.ErrExist", os.ErrExist, ErrorUnknown, false, false},

		// Syscall errors - Permission
		{"EACCES", syscall.EACCES, ErrorPermissionDenied, false, true},
		{"EPERM", syscall.EPERM, ErrorPermissionDenied, false, true},

		// Syscall errors - File in use (retryable)
		{"EBUSY", syscall.EBUSY, ErrorFileInUse, true, false},
		{"ETXTBSY", syscall.ETXTBSY, ErrorFileInUse, true, false},

		// Syscall errors - File operations
		{"ENOENT", syscall.ENOENT, ErrorFileNotFound, false, false},
		{"EISDIR", syscall.EISDIR, ErrorIsDirectory, false, false},

		// Unknown errors
		{"generic error", errors.New("something went wrong"), ErrorUnknown, false, false},
		{"wrapped error", fmt.Errorf("wrapped: %w", errors.New("inner")), ErrorUnknown, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CategorizeError("/test/path", tt.err)

			if tt.err == nil {
				if result != nil {
					t.Error("expected nil for nil error")
				}
				return
			}

			if result == nil {
				t.Fatal("unexpected nil result")
			}

			if result.Reason != tt.wantReason {
				t.Errorf("Reason = %v, want %v", result.Reason, tt.wantReason)
			}
			if result.Retryable != tt.wantRetryable {
				t.Errorf("Retryable = %v, want %v", result.Retryable, tt.wantRetryable)
			}
			if result.NeedsSudo != tt.wantNeedsSudo {
				t.Errorf("NeedsSudo = %v, want %v", result.NeedsSudo, tt.wantNeedsSudo)
			}
			if result.Path != "/test/path" {
				t.Errorf("Path = %q, want /test/path", result.Path)
			}
			if result.Original != tt.err {
				t.Errorf("Original error not preserved")
			}
		})
	}
}

func TestErrorReasonString(t *testing.T) {
	tests := []struct {
		reason ErrorReason
		want   string
	}{
		{ErrorPermissionDenied, "Permission denied"},
		{ErrorFileInUse, "File is in use"},
		{ErrorFileNotFound, "File not found"},
		{ErrorIsDirectory, "Is a directory"},
		{ErrorInvalidPath, "Invalid path"},
		{ErrorUnknown, "Unknown error"},
		{ErrorReason(999), "Unspecified error"}, // Invalid enum value
		{ErrorReason(-1), "Unspecified error"},  // Negative value
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.reason.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeletionErrorInterface(t *testing.T) {
	err := &DeletionError{
		Path:      "/test/path",
		Reason:    ErrorPermissionDenied,
		Original:  os.ErrPermission,
		Retryable: false,
		NeedsSudo: true,
	}

	// Test error interface
	var _ error = err

	errStr := err.Error()
	if !strings.Contains(errStr, "/test/path") {
		t.Error("Error() should contain path")
	}
	if !strings.Contains(errStr, "Permission denied") {
		t.Error("Error() should contain reason")
	}
}

func TestDeletionErrorUserMessage(t *testing.T) {
	tests := []struct {
		name   string
		err    *DeletionError
		substr string
	}{
		{
			name:   "permission denied with sudo",
			err:    &DeletionError{Path: "/test", Reason: ErrorPermissionDenied, NeedsSudo: true},
			substr: "elevated permissions",
		},
		{
			name:   "permission denied without sudo",
			err:    &DeletionError{Path: "/test", Reason: ErrorPermissionDenied, NeedsSudo: false},
			substr: "Permission denied",
		},
		{
			name:   "file in use",
			err:    &DeletionError{Path: "/test", Reason: ErrorFileInUse},
			substr: "being used",
		},
		{
			name:   "file not found",
			err:    &DeletionError{Path: "/test", Reason: ErrorFileNotFound},
			substr: "Already deleted",
		},
		{
			name:   "is directory",
			err:    &DeletionError{Path: "/test", Reason: ErrorIsDirectory},
			substr: "directory",
		},
		{
			name:   "invalid path",
			err:    &DeletionError{Path: "/test", Reason: ErrorInvalidPath},
			substr: "Invalid",
		},
		{
			name:   "unknown error",
			err:    &DeletionError{Path: "/test", Reason: ErrorUnknown, Original: errors.New("weird")},
			substr: "Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.UserMessage()
			if !strings.Contains(msg, tt.substr) {
				t.Errorf("UserMessage() = %q, want to contain %q", msg, tt.substr)
			}
			if !strings.Contains(msg, tt.err.Path) {
				t.Errorf("UserMessage() should contain path %q", tt.err.Path)
			}
		})
	}
}

func TestGroupErrors(t *testing.T) {
	errs := []*DeletionError{
		{Path: "/a", Reason: ErrorPermissionDenied},
		{Path: "/b", Reason: ErrorPermissionDenied},
		{Path: "/c", Reason: ErrorFileInUse},
		{Path: "/d", Reason: ErrorFileNotFound},
		{Path: "/e", Reason: ErrorFileInUse},
		{Path: "/f", Reason: ErrorUnknown},
	}

	grouped := GroupErrors(errs)

	if len(grouped[ErrorPermissionDenied]) != 2 {
		t.Errorf("permission denied count = %d, want 2", len(grouped[ErrorPermissionDenied]))
	}
	if len(grouped[ErrorFileInUse]) != 2 {
		t.Errorf("file in use count = %d, want 2", len(grouped[ErrorFileInUse]))
	}
	if len(grouped[ErrorFileNotFound]) != 1 {
		t.Errorf("file not found count = %d, want 1", len(grouped[ErrorFileNotFound]))
	}
	if len(grouped[ErrorUnknown]) != 1 {
		t.Errorf("unknown count = %d, want 1", len(grouped[ErrorUnknown]))
	}
}

func TestGroupErrorsEmpty(t *testing.T) {
	grouped := GroupErrors([]*DeletionError{})
	if len(grouped) != 0 {
		t.Errorf("expected empty map, got %d entries", len(grouped))
	}
}

func TestFormatErrorSummary(t *testing.T) {
	t.Run("empty errors", func(t *testing.T) {
		summary := FormatErrorSummary([]*DeletionError{})
		if summary != "" {
			t.Errorf("expected empty summary, got %q", summary)
		}
	})

	t.Run("permission errors", func(t *testing.T) {
		errs := []*DeletionError{
			{Path: "/a", Reason: ErrorPermissionDenied},
			{Path: "/b", Reason: ErrorPermissionDenied},
		}
		summary := FormatErrorSummary(errs)
		if !strings.Contains(summary, "Permission denied") {
			t.Error("summary should mention permission denied")
		}
		if !strings.Contains(summary, "2 files") {
			t.Error("summary should show count")
		}
	})

	t.Run("multiple error types", func(t *testing.T) {
		errs := []*DeletionError{
			{Path: "/a", Reason: ErrorPermissionDenied},
			{Path: "/b", Reason: ErrorFileInUse},
			{Path: "/c", Reason: ErrorFileNotFound},
			{Path: "/d", Reason: ErrorIsDirectory},
			{Path: "/e", Reason: ErrorUnknown},
		}
		summary := FormatErrorSummary(errs)
		if !strings.Contains(summary, "Permission denied") {
			t.Error("summary should mention permission denied")
		}
		if !strings.Contains(summary, "File in use") {
			t.Error("summary should mention file in use")
		}
	})
}

// =============================================================================
// Permission Manager Tests - Comprehensive
// =============================================================================

func TestNewPermissionManager(t *testing.T) {
	pm := NewPermissionManager()

	if pm == nil {
		t.Fatal("NewPermissionManager returned nil")
	}

	if pm.userName == "" {
		t.Error("userName not set")
	}

	if len(pm.userGroups) == 0 {
		t.Error("no groups found")
	}

	info := pm.GetUserInfo()
	if info == "" {
		t.Error("GetUserInfo returned empty")
	}
	if !strings.Contains(info, "uid=") {
		t.Error("GetUserInfo should contain uid")
	}
	if !strings.Contains(info, "gid=") {
		t.Error("GetUserInfo should contain gid")
	}
}

func TestIsRunningAsRoot(t *testing.T) {
	pm := NewPermissionManager()
	isRoot := pm.IsRunningAsRoot()

	// Just verify it returns consistent value
	if pm.IsRunningAsRoot() != isRoot {
		t.Error("IsRunningAsRoot not consistent")
	}

	// Verify it matches currentUID check
	if isRoot != (pm.currentUID == 0) {
		t.Error("IsRunningAsRoot doesn't match UID check")
	}
}

func TestAnalyzeFilePermissions(t *testing.T) {
	f := testutil.NewFixture(t)

	t.Run("regular file", func(t *testing.T) {
		file := f.CreateFile("regular.txt", []byte("content"))
		pm := NewPermissionManager()
		info := pm.AnalyzeFilePermissions(file)

		if !info.Exists {
			t.Error("file should exist")
		}
		if info.IsDir {
			t.Error("should not be directory")
		}
		if info.IsSymlink {
			t.Error("should not be symlink")
		}
		if info.IsSpecialFile {
			t.Error("should not be special file")
		}
		if info.Path != file {
			t.Errorf("Path = %q, want %q", info.Path, file)
		}
	})

	t.Run("directory", func(t *testing.T) {
		pm := NewPermissionManager()
		info := pm.AnalyzeFilePermissions(f.CacheDir)

		if !info.Exists {
			t.Error("directory should exist")
		}
		if !info.IsDir {
			t.Error("should be detected as directory")
		}
	})

	t.Run("symlink", func(t *testing.T) {
		target := f.CreateFile("target.txt", []byte("content"))
		link := f.CreateSymlink(target, "link.txt")

		pm := NewPermissionManager()
		info := pm.AnalyzeFilePermissions(link)

		if !info.Exists {
			t.Error("symlink should exist")
		}
		if !info.IsSymlink {
			t.Error("should be detected as symlink")
		}
		if info.SymlinkTarget == "" {
			t.Error("symlink target should be set")
		}
	})

	t.Run("non-existent", func(t *testing.T) {
		pm := NewPermissionManager()
		info := pm.AnalyzeFilePermissions("/nonexistent/path/12345")

		if info.Exists {
			t.Error("should not exist")
		}
		if info.Reason == "" {
			t.Error("should have a reason")
		}
	})

	t.Run("broken symlink", func(t *testing.T) {
		link := f.CreateBrokenSymlink("broken_link.txt")

		pm := NewPermissionManager()
		info := pm.AnalyzeFilePermissions(link)

		// Broken symlink behavior depends on implementation
		t.Logf("Broken symlink: Exists=%v, IsSymlink=%v", info.Exists, info.IsSymlink)
	})
}

func TestCanDelete(t *testing.T) {
	f := testutil.NewFixture(t)

	t.Run("own file", func(t *testing.T) {
		file := f.CreateFile("deletable.txt", []byte("content"))
		pm := NewPermissionManager()

		canDelete, err := pm.CanDelete(file)
		if !canDelete {
			t.Errorf("should be able to delete own file: %v", err)
		}
	})

	t.Run("non-existent", func(t *testing.T) {
		pm := NewPermissionManager()
		canDelete, err := pm.CanDelete("/nonexistent/path/12345")

		if canDelete {
			t.Error("should not be able to delete non-existent file")
		}
		if err == nil {
			t.Error("should return error for non-existent file")
		}
	})

	t.Run("directory", func(t *testing.T) {
		pm := NewPermissionManager()
		canDelete, _ := pm.CanDelete(f.CacheDir)

		// Should be able to delete own directory
		t.Logf("Can delete own directory: %v", canDelete)
	})
}

func TestRequiresElevation(t *testing.T) {
	f := testutil.NewFixture(t)

	file := f.CreateFile("owned.txt", []byte("content"))
	pm := NewPermissionManager()

	if pm.IsRunningAsRoot() {
		t.Skip("skipping elevation test when running as root")
	}

	requires := pm.RequiresElevation(file)
	if requires {
		t.Error("should not require elevation for owned file")
	}
}

func TestAnalyzePermissions(t *testing.T) {
	f := testutil.NewFixture(t)

	file1 := f.CreateFile("file1.txt", []byte("content1"))
	file2 := f.CreateFile("file2.txt", []byte("content2"))
	file3 := f.CreateFile("file3.txt", []byte("content3"))

	pm := NewPermissionManager()
	report := pm.AnalyzePermissions([]string{file1, file2, file3}, nil)

	if len(report.NormalFiles) != 3 {
		t.Errorf("NormalFiles = %d, want 3", len(report.NormalFiles))
	}
	if len(report.RequiresSudo) != 0 {
		t.Errorf("RequiresSudo = %d, want 0", len(report.RequiresSudo))
	}
	if len(report.AnalysisDetails) != 3 {
		t.Errorf("AnalysisDetails = %d, want 3", len(report.AnalysisDetails))
	}
}

func TestAnalyzePermissionsWithSizeFunc(t *testing.T) {
	f := testutil.NewFixture(t)

	file := f.CreateFile("sized.txt", []byte("12345"))

	pm := NewPermissionManager()
	sizeFunc := func(path string) int64 { return 5 }
	report := pm.AnalyzePermissions([]string{file}, sizeFunc)

	if report.TotalNormalSize != 5 {
		t.Errorf("TotalNormalSize = %d, want 5", report.TotalNormalSize)
	}
}

func TestAnalyzePermissionsNonExistent(t *testing.T) {
	pm := NewPermissionManager()
	report := pm.AnalyzePermissions([]string{"/nonexistent/path"}, nil)

	// Non-existent files should be silently skipped
	if len(report.NormalFiles) != 0 {
		t.Errorf("NormalFiles = %d, want 0", len(report.NormalFiles))
	}
}

// =============================================================================
// Safety Check Tests - Comprehensive Security Edge Cases
// =============================================================================

func TestIsSafeToDelete(t *testing.T) {
	f := testutil.NewFixture(t)

	t.Run("regular file", func(t *testing.T) {
		file := f.CreateFile("safe.txt", []byte("content"))
		err := IsSafeToDelete(file)
		if err != nil {
			t.Errorf("regular file should be safe: %v", err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		err := IsSafeToDelete(f.CacheDir)
		if err != nil {
			t.Errorf("directory should be safe: %v", err)
		}
	})

	t.Run("non-existent", func(t *testing.T) {
		err := IsSafeToDelete("/nonexistent/path/12345")
		if err == nil {
			t.Error("non-existent file should return error")
		}
	})
}

func TestIsSafeToDeletePathTraversal(t *testing.T) {
	traversalPaths := []string{
		"/some/path/../../../etc/passwd",
		"/tmp/../../../etc/shadow",
		"./../../etc/passwd",
	}

	for _, path := range traversalPaths {
		t.Run(path, func(t *testing.T) {
			err := IsSafeToDelete(path)
			if err == nil {
				t.Errorf("path traversal should be rejected: %s", path)
			}
		})
	}
}

func TestIsSpecialFile(t *testing.T) {
	f := testutil.NewFixture(t)

	t.Run("regular file", func(t *testing.T) {
		file := f.CreateFile("regular.txt", []byte("content"))
		isSpecial, err := IsSpecialFile(file)
		if isSpecial {
			t.Error("regular file should not be special")
		}
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		isSpecial, _ := IsSpecialFile(f.CacheDir)
		if isSpecial {
			t.Error("directory should not be flagged as special file")
		}
	})

	t.Run("symlink to regular file", func(t *testing.T) {
		target := f.CreateFile("target.txt", []byte("content"))
		link := f.CreateSymlink(target, "link.txt")

		isSpecial, err := IsSpecialFile(link)
		if isSpecial {
			t.Errorf("symlink to regular file should not be special: %v", err)
		}
	})

	t.Run("broken symlink", func(t *testing.T) {
		link := f.CreateBrokenSymlink("broken_link")

		isSpecial, _ := IsSpecialFile(link)
		// Broken symlinks are not special (safe to delete the link itself)
		if isSpecial {
			t.Error("broken symlink should not be marked as special")
		}
	})

	t.Run("non-existent", func(t *testing.T) {
		_, err := IsSpecialFile("/nonexistent/path/12345")
		if err == nil {
			t.Error("should return error for non-existent file")
		}
	})
}

func TestVerifyDeletionSafe(t *testing.T) {
	f := testutil.NewFixture(t)

	t.Run("unchanged file", func(t *testing.T) {
		file := f.CreateFile("verify.txt", []byte("content"))

		inode, err := GetFileInode(file)
		if err != nil {
			t.Fatalf("failed to get inode: %v", err)
		}

		info, _ := os.Stat(file)
		size := info.Size()

		err = VerifyDeletionSafe(file, inode, size)
		if err != nil {
			t.Errorf("verification should pass: %v", err)
		}
	})

	t.Run("non-existent is OK", func(t *testing.T) {
		err := VerifyDeletionSafe("/nonexistent/12345", 0, 0)
		if err != nil {
			t.Errorf("non-existent should be fine: %v", err)
		}
	})

	t.Run("size change detection", func(t *testing.T) {
		file := f.CreateFile("changing.txt", []byte("small"))

		inode, _ := GetFileInode(file)

		// Report a much larger original size (simulating file replacement)
		err := VerifyDeletionSafe(file, inode, 1000000)
		if err == nil {
			t.Error("should detect significant size change")
		}
	})

	t.Run("inode change detection", func(t *testing.T) {
		file := f.CreateFile("inode_test.txt", []byte("content"))

		info, _ := os.Stat(file)
		size := info.Size()

		// Use wrong inode
		err := VerifyDeletionSafe(file, 99999999, size)
		if err == nil {
			t.Error("should detect inode change")
		}
	})
}

func TestGetFileInode(t *testing.T) {
	f := testutil.NewFixture(t)

	t.Run("valid file", func(t *testing.T) {
		file := f.CreateFile("inode.txt", []byte("content"))

		inode, err := GetFileInode(file)
		if err != nil {
			t.Fatalf("failed to get inode: %v", err)
		}
		if inode == 0 {
			t.Error("inode should not be zero")
		}

		// Consistent
		inode2, _ := GetFileInode(file)
		if inode != inode2 {
			t.Error("inode should be consistent")
		}
	})

	t.Run("non-existent", func(t *testing.T) {
		_, err := GetFileInode("/nonexistent/path/12345")
		if err == nil {
			t.Error("should return error for non-existent file")
		}
	})

	t.Run("symlink", func(t *testing.T) {
		target := f.CreateFile("target.txt", []byte("content"))
		link := f.CreateSymlink(target, "link.txt")

		inodeTarget, _ := GetFileInode(target)
		inodeLink, _ := GetFileInode(link)

		// Lstat should give different inodes for link vs target
		t.Logf("Target inode: %d, Link inode: %d", inodeTarget, inodeLink)
	})
}

// =============================================================================
// Cleaner Tests - Comprehensive
// =============================================================================

func TestNewCleaner(t *testing.T) {
	cfg := &config.Config{DryRun: true}
	c := New(cfg)

	if c == nil {
		t.Fatal("New returned nil")
	}
	if c.config != cfg {
		t.Error("config not set")
	}
	if c.permissionManager == nil {
		t.Error("permission manager not initialized")
	}
	if c.manifest == nil {
		t.Error("manifest not initialized")
	}
	if c.sudoManager == nil {
		t.Error("sudo manager not initialized")
	}
}

func TestSetAskSudo(t *testing.T) {
	c := New(&config.Config{})

	if !c.askSudo {
		t.Error("askSudo should default to true")
	}

	c.SetAskSudo(false)
	if c.askSudo {
		t.Error("askSudo should be false after SetAskSudo(false)")
	}
}

func TestCleanDryRun(t *testing.T) {
	f := testutil.NewFixture(t)

	file1 := f.CreateCacheFile("cache1.txt", 100)
	file2 := f.CreateCacheFile("cache2.txt", 200)

	cfg := &config.Config{DryRun: true, MinFileAge: 24}
	c := New(cfg)

	scanResult := &scanner.ScanResult{
		Files: []scanner.FileInfo{
			{Path: file1, Size: 100, Category: "cache", ModTime: time.Now().Add(-48 * time.Hour)},
			{Path: file2, Size: 200, Category: "cache", ModTime: time.Now().Add(-48 * time.Hour)},
		},
		TotalSize:  300,
		TotalCount: 2,
	}

	result, err := c.Clean(scanResult)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if !result.DryRun {
		t.Error("should be marked as dry run")
	}
	if len(result.DeletedFiles) != 2 {
		t.Errorf("DeletedFiles = %d, want 2", len(result.DeletedFiles))
	}
	if result.DeletedSize != 300 {
		t.Errorf("DeletedSize = %d, want 300", result.DeletedSize)
	}

	// Files should still exist
	f.AssertFileExists(file1)
	f.AssertFileExists(file2)
}

func TestCleanActualDeletion(t *testing.T) {
	f := testutil.NewFixture(t)

	file := f.CreateFileWithAge("cache/old.txt", []byte("content"), 48*time.Hour)

	cfg := &config.Config{DryRun: false, MinFileAge: 24}
	c := New(cfg)
	c.SetAskSudo(false)

	scanResult := &scanner.ScanResult{
		Files: []scanner.FileInfo{
			{Path: file, Size: 7, Category: "cache", ModTime: time.Now().Add(-48 * time.Hour)},
		},
		TotalSize:  7,
		TotalCount: 1,
	}

	result, err := c.Clean(scanResult)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if result.DryRun {
		t.Error("should not be dry run")
	}
	if len(result.DeletedFiles) != 1 {
		t.Errorf("DeletedFiles = %d, want 1", len(result.DeletedFiles))
	}

	f.AssertFileNotExists(file)
}

func TestCleanSkipsNewFiles(t *testing.T) {
	f := testutil.NewFixture(t)

	// New file (too young)
	file := f.CreateFile("cache/new.txt", []byte("content"))

	cfg := &config.Config{DryRun: false, MinFileAge: 24}
	c := New(cfg)
	c.SetAskSudo(false)

	scanResult := &scanner.ScanResult{
		Files: []scanner.FileInfo{
			{Path: file, Size: 7, Category: "cache", ModTime: time.Now()},
		},
		TotalSize:  7,
		TotalCount: 1,
	}

	result, err := c.Clean(scanResult)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if len(result.DeletedFiles) != 0 {
		t.Errorf("DeletedFiles = %d, want 0", len(result.DeletedFiles))
	}
	if len(result.SkippedFiles) != 1 {
		t.Errorf("SkippedFiles = %d, want 1", len(result.SkippedFiles))
	}

	f.AssertFileExists(file)
}

func TestCleanSkipsSymlinks(t *testing.T) {
	f := testutil.NewFixture(t)

	target := f.CreateFileWithAge("cache/target.txt", []byte("content"), 48*time.Hour)
	link := f.CreateSymlink(target, "cache/link.txt")

	cfg := &config.Config{DryRun: false, MinFileAge: 24}
	c := New(cfg)
	c.SetAskSudo(false)

	scanResult := &scanner.ScanResult{
		Files: []scanner.FileInfo{
			{Path: link, Size: 7, Category: "cache", ModTime: time.Now().Add(-48 * time.Hour)},
		},
		TotalSize:  7,
		TotalCount: 1,
	}

	result, err := c.Clean(scanResult)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Symlink should be skipped (security)
	if len(result.DeletedFiles) != 0 {
		t.Errorf("DeletedFiles = %d, want 0", len(result.DeletedFiles))
	}

	f.AssertFileExists(target)
	f.AssertFileExists(link)
}

func TestCleanDirectory(t *testing.T) {
	f := testutil.NewFixture(t)

	dir := f.CreateDir("project/node_modules")
	f.CreateFileWithAge("project/node_modules/package.json", []byte("{}"), 48*time.Hour)
	f.CreateFileWithAge("project/node_modules/index.js", []byte("module.exports = {}"), 48*time.Hour)

	oldTime := time.Now().Add(-48 * time.Hour)
	os.Chtimes(dir, oldTime, oldTime)

	cfg := &config.Config{DryRun: false, MinFileAge: 24}
	c := New(cfg)
	c.SetAskSudo(false)

	scanResult := &scanner.ScanResult{
		Files: []scanner.FileInfo{
			{Path: dir, Size: 1000, Category: "node_modules", ModTime: oldTime},
		},
		TotalSize:  1000,
		TotalCount: 1,
	}

	result, err := c.Clean(scanResult)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if len(result.DeletedFiles) != 1 {
		t.Errorf("DeletedFiles = %d, want 1", len(result.DeletedFiles))
	}

	f.AssertFileNotExists(dir)
}

func TestCleanCategory(t *testing.T) {
	f := testutil.NewFixture(t)

	cacheFile := f.CreateCacheFile("cache.txt", 100)
	logFile := f.CreateLogFile("log.txt", 50)

	cfg := &config.Config{DryRun: true, MinFileAge: 24}
	c := New(cfg)

	scanResult := &scanner.ScanResult{
		Files: []scanner.FileInfo{
			{Path: cacheFile, Size: 100, Category: "cache", ModTime: time.Now().Add(-48 * time.Hour)},
			{Path: logFile, Size: 50, Category: "logs", ModTime: time.Now().Add(-48 * time.Hour)},
		},
		TotalSize:  150,
		TotalCount: 2,
	}

	result, err := c.CleanCategory(scanResult, "cache")
	if err != nil {
		t.Fatalf("CleanCategory failed: %v", err)
	}

	if len(result.DeletedFiles) != 1 {
		t.Errorf("DeletedFiles = %d, want 1", len(result.DeletedFiles))
	}
	if result.DeletedSize != 100 {
		t.Errorf("DeletedSize = %d, want 100", result.DeletedSize)
	}
}

func TestCleanEmptyScanResult(t *testing.T) {
	cfg := &config.Config{DryRun: false}
	c := New(cfg)
	c.SetAskSudo(false)

	scanResult := &scanner.ScanResult{
		Files:      []scanner.FileInfo{},
		TotalSize:  0,
		TotalCount: 0,
	}

	result, err := c.Clean(scanResult)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if len(result.DeletedFiles) != 0 {
		t.Error("should have no deleted files")
	}
	if len(result.Errors) != 0 {
		t.Error("should have no errors")
	}
}

func TestCleanNonExistentFile(t *testing.T) {
	cfg := &config.Config{DryRun: false, MinFileAge: 0}
	c := New(cfg)
	c.SetAskSudo(false)

	scanResult := &scanner.ScanResult{
		Files: []scanner.FileInfo{
			{Path: "/nonexistent/file/12345", Size: 100, Category: "cache", ModTime: time.Now().Add(-48 * time.Hour)},
		},
		TotalSize:  100,
		TotalCount: 1,
	}

	result, err := c.Clean(scanResult)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Non-existent files should be handled gracefully
	t.Logf("Deleted: %d, Skipped: %d, Errors: %d",
		len(result.DeletedFiles), len(result.SkippedFiles), len(result.Errors))
}

func TestGetPermissionReport(t *testing.T) {
	f := testutil.NewFixture(t)

	file := f.CreateFile("test.txt", []byte("content"))

	cfg := &config.Config{}
	c := New(cfg)

	scanResult := &scanner.ScanResult{
		Files: []scanner.FileInfo{
			{Path: file, Size: 7, Category: "cache"},
		},
	}

	report := c.GetPermissionReport(scanResult)

	if report == nil {
		t.Fatal("report is nil")
	}
	if len(report.NormalFiles) != 1 {
		t.Errorf("NormalFiles = %d, want 1", len(report.NormalFiles))
	}
}

func TestCleanerGetManifest(t *testing.T) {
	c := New(&config.Config{})
	m := c.GetManifest()

	if m == nil {
		t.Error("GetManifest returned nil")
	}
}

// =============================================================================
// Manifest Tests
// =============================================================================

func TestDeletionManifest(t *testing.T) {
	m := NewDeletionManifest()

	if m == nil {
		t.Fatal("NewDeletionManifest returned nil")
	}
	if len(m.Files) != 0 {
		t.Error("should start empty")
	}
	if m.TotalSize != 0 {
		t.Error("total size should start at 0")
	}

	m.Add("/path/file1", 100, "cache")
	m.Add("/path/file2", 200, "logs")

	if len(m.Files) != 2 {
		t.Errorf("Files = %d, want 2", len(m.Files))
	}
	if m.TotalSize != 300 {
		t.Errorf("TotalSize = %d, want 300", m.TotalSize)
	}

	// Check individual entries
	if m.Files[0].Path != "/path/file1" {
		t.Error("first file path incorrect")
	}
	if m.Files[0].Size != 100 {
		t.Error("first file size incorrect")
	}
	if m.Files[0].Category != "cache" {
		t.Error("first file category incorrect")
	}
	if m.Files[0].DeletedAt.IsZero() {
		t.Error("DeletedAt should be set")
	}
}

func TestDeletionManifestSave(t *testing.T) {
	f := testutil.NewFixture(t)

	m := NewDeletionManifest()
	m.Add("/path/file1", 100, "cache")
	m.Add("/path/file2", 200, "logs")

	manifestPath := filepath.Join(f.RootDir, "manifest.txt")
	err := m.Save(manifestPath)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	f.AssertFileExists(manifestPath)

	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Deletion Manifest") {
		t.Error("manifest should have header")
	}
	if !strings.Contains(contentStr, "/path/file1") {
		t.Error("manifest should contain file1")
	}
	if !strings.Contains(contentStr, "/path/file2") {
		t.Error("manifest should contain file2")
	}
	if !strings.Contains(contentStr, "Total Size:") {
		t.Error("manifest should contain total size")
	}
	if !strings.Contains(contentStr, "Total Files:") {
		t.Error("manifest should contain file count")
	}
}

func TestDeletionManifestSaveError(t *testing.T) {
	m := NewDeletionManifest()
	m.Add("/path/file", 100, "cache")

	err := m.Save("/nonexistent/directory/manifest.txt")
	if err == nil {
		t.Error("should fail to save to non-existent directory")
	}
}

func TestCleanerSaveManifest(t *testing.T) {
	f := testutil.NewFixture(t)

	c := New(&config.Config{})
	c.manifest.Add("/test/file", 100, "cache")

	manifestPath := filepath.Join(f.RootDir, "clean_manifest.txt")
	err := c.SaveManifest(manifestPath)
	if err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}

	f.AssertFileExists(manifestPath)
}

// =============================================================================
// Edge Cases and Security Tests
// =============================================================================

func TestCleanMultipleFiles(t *testing.T) {
	f := testutil.NewFixture(t)

	files := make([]scanner.FileInfo, 10)
	for i := 0; i < 10; i++ {
		path := f.CreateCacheFile(fmt.Sprintf("file%d.cache", i), 100)
		files[i] = scanner.FileInfo{
			Path:     path,
			Size:     100,
			Category: "cache",
			ModTime:  time.Now().Add(-48 * time.Hour),
		}
	}

	cfg := &config.Config{DryRun: false, MinFileAge: 24}
	c := New(cfg)
	c.SetAskSudo(false)

	scanResult := &scanner.ScanResult{
		Files:      files,
		TotalSize:  1000,
		TotalCount: 10,
	}

	result, err := c.Clean(scanResult)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if len(result.DeletedFiles) != 10 {
		t.Errorf("DeletedFiles = %d, want 10", len(result.DeletedFiles))
	}
	if result.DeletedSize != 1000 {
		t.Errorf("DeletedSize = %d, want 1000", result.DeletedSize)
	}
}

func TestCleanMixedResults(t *testing.T) {
	f := testutil.NewFixture(t)

	// Old file (should delete)
	oldFile := f.CreateFileWithAge("cache/old.txt", []byte("old"), 48*time.Hour)

	// New file (should skip)
	newFile := f.CreateFile("cache/new.txt", []byte("new"))

	cfg := &config.Config{DryRun: false, MinFileAge: 24}
	c := New(cfg)
	c.SetAskSudo(false)

	scanResult := &scanner.ScanResult{
		Files: []scanner.FileInfo{
			{Path: oldFile, Size: 3, Category: "cache", ModTime: time.Now().Add(-48 * time.Hour)},
			{Path: newFile, Size: 3, Category: "cache", ModTime: time.Now()},
		},
		TotalSize:  6,
		TotalCount: 2,
	}

	result, err := c.Clean(scanResult)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if len(result.DeletedFiles) != 1 {
		t.Errorf("DeletedFiles = %d, want 1", len(result.DeletedFiles))
	}
	if len(result.SkippedFiles) != 1 {
		t.Errorf("SkippedFiles = %d, want 1", len(result.SkippedFiles))
	}

	f.AssertFileNotExists(oldFile)
	f.AssertFileExists(newFile)
}

func TestCleanWithReadOnlyDirectory(t *testing.T) {
	testutil.SkipIfRoot(t)
	f := testutil.NewFixture(t)

	// Create read-only directory with a file inside
	roDir := f.CreateReadOnlyDir("readonly")
	trappedFile := filepath.Join(roDir, "trapped.txt")

	cfg := &config.Config{DryRun: false, MinFileAge: 0}
	c := New(cfg)
	c.SetAskSudo(false)

	scanResult := &scanner.ScanResult{
		Files: []scanner.FileInfo{
			{Path: trappedFile, Size: 7, Category: "cache", ModTime: time.Now().Add(-48 * time.Hour)},
		},
		TotalSize:  7,
		TotalCount: 1,
	}

	result, err := c.Clean(scanResult)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Should fail to delete (no write permission on parent)
	if len(result.DeletedFiles) != 0 {
		t.Error("should not be able to delete file in read-only directory")
	}
}

func TestCleanSpecialCharactersInPath(t *testing.T) {
	f := testutil.NewFixture(t)

	specialPaths := []string{
		"file with spaces.txt",
		"file-with-dashes.txt",
		"file_with_underscores.txt",
		"file.multiple.dots.txt",
	}

	var files []scanner.FileInfo
	for _, name := range specialPaths {
		path := f.CreateFileWithAge(filepath.Join("cache", name), []byte("content"), 48*time.Hour)
		files = append(files, scanner.FileInfo{
			Path:     path,
			Size:     7,
			Category: "cache",
			ModTime:  time.Now().Add(-48 * time.Hour),
		})
	}

	cfg := &config.Config{DryRun: false, MinFileAge: 24}
	c := New(cfg)
	c.SetAskSudo(false)

	scanResult := &scanner.ScanResult{
		Files:      files,
		TotalSize:  int64(7 * len(files)),
		TotalCount: len(files),
	}

	result, err := c.Clean(scanResult)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if len(result.DeletedFiles) != len(specialPaths) {
		t.Errorf("DeletedFiles = %d, want %d", len(result.DeletedFiles), len(specialPaths))
	}
}

// =============================================================================
// Concurrent Safety Tests
// =============================================================================

func TestCleanConcurrentSafety(t *testing.T) {
	f := testutil.NewFixture(t)

	// Create files
	for i := 0; i < 5; i++ {
		f.CreateCacheFile(fmt.Sprintf("concurrent%d.txt", i), 100)
	}

	cfg := &config.Config{DryRun: true} // Use dry run for safety
	c := New(cfg)

	// Run multiple cleans concurrently
	done := make(chan bool, 3)
	for i := 0; i < 3; i++ {
		go func() {
			scanResult := &scanner.ScanResult{
				Files: []scanner.FileInfo{
					{Path: f.Path("cache/concurrent0.txt"), Size: 100, Category: "cache"},
				},
			}
			c.Clean(scanResult)
			done <- true
		}()
	}

	for i := 0; i < 3; i++ {
		<-done
	}
}
