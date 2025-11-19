package cleaner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanDelete(t *testing.T) {
	pm := NewPermissionManager()

	// Create test directory structure
	tempDir := os.TempDir()
	testDir := filepath.Join(tempDir, "cleanup-test-permissions")
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	tests := []struct {
		name      string
		setup     func() string // Returns path to test file
		cleanup   func(string)
		canDelete bool
	}{
		{
			name: "writable file by user",
			setup: func() string {
				path := filepath.Join(testDir, "writable.txt")
				os.WriteFile(path, []byte("test"), 0644)
				return path
			},
			cleanup:   func(p string) { os.Remove(p) },
			canDelete: true,
		},
		{
			name: "read-only file owned by user",
			setup: func() string {
				path := filepath.Join(testDir, "readonly.txt")
				os.WriteFile(path, []byte("test"), 0444)
				return path
			},
			cleanup:   func(p string) { os.Chmod(p, 0644); os.Remove(p) },
			canDelete: true, // Can still delete if parent dir is writable
		},
		{
			name: "directory with write permission",
			setup: func() string {
				path := filepath.Join(testDir, "writable-dir")
				os.Mkdir(path, 0755)
				return path
			},
			cleanup:   func(p string) { os.RemoveAll(p) },
			canDelete: true,
		},
		{
			name: "non-existent file",
			setup: func() string {
				return filepath.Join(testDir, "non-existent.txt")
			},
			cleanup:   func(p string) {},
			canDelete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testPath := tt.setup()
			defer tt.cleanup(testPath)

			canDelete, err := pm.CanDelete(testPath)

			if tt.canDelete {
				if !canDelete {
					t.Errorf("CanDelete(%s) = false, want true", testPath)
				}
			} else {
				if err == nil && canDelete {
					t.Errorf("CanDelete(%s) = true, want false or error", testPath)
				}
			}
		})
	}
}

func TestIsSpecialFile(t *testing.T) {
	// Create test directory structure
	tempDir := os.TempDir()
	testDir := filepath.Join(tempDir, "cleanup-test-special")
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	tests := []struct {
		name          string
		setup         func() string // Returns path to test file
		cleanup       func(string)
		isSpecial     bool
		errorContains string
	}{
		{
			name: "regular file",
			setup: func() string {
				path := filepath.Join(testDir, "regular.txt")
				os.WriteFile(path, []byte("test"), 0644)
				return path
			},
			cleanup:   func(p string) { os.Remove(p) },
			isSpecial: false,
		},
		{
			name: "regular directory",
			setup: func() string {
				path := filepath.Join(testDir, "regular-dir")
				os.Mkdir(path, 0755)
				return path
			},
			cleanup:   func(p string) { os.RemoveAll(p) },
			isSpecial: false,
		},
		{
			name: "symlink to regular file",
			setup: func() string {
				target := filepath.Join(testDir, "symlink-target.txt")
				os.WriteFile(target, []byte("test"), 0644)

				symlinkPath := filepath.Join(testDir, "symlink.txt")
				os.Symlink(target, symlinkPath)
				return symlinkPath
			},
			cleanup: func(p string) {
				os.Remove(p)
				os.Remove(filepath.Join(testDir, "symlink-target.txt"))
			},
			isSpecial:     false, // Symlink resolves to regular file
			errorContains: "",
		},
		{
			name: "non-existent file",
			setup: func() string {
				return filepath.Join(testDir, "non-existent-special.txt")
			},
			cleanup:   func(p string) {},
			isSpecial: false, // Will return error, but not special
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testPath := tt.setup()
			defer tt.cleanup(testPath)

			isSpecial, err := IsSpecialFile(testPath)

			if isSpecial != tt.isSpecial {
				t.Errorf("IsSpecialFile(%s) = %v, want %v", testPath, isSpecial, tt.isSpecial)
			}

			if tt.errorContains != "" {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorContains)
				} else if !stringContains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			}
		})
	}
}

func TestPermissionManager_AnalyzePermissions(t *testing.T) {
	pm := NewPermissionManager()

	// Create test directory structure
	tempDir := os.TempDir()
	testDir := filepath.Join(tempDir, "cleanup-test-analyze")
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	// Create test files
	normalFile := filepath.Join(testDir, "normal.txt")
	os.WriteFile(normalFile, []byte("test"), 0644)

	readOnlyFile := filepath.Join(testDir, "readonly.txt")
	os.WriteFile(readOnlyFile, []byte("test"), 0444)

	nonExistentFile := filepath.Join(testDir, "non-existent.txt")

	// Create size getter function
	getSizeFunc := func(path string) int64 {
		info, err := os.Stat(path)
		if err != nil {
			return 0
		}
		return info.Size()
	}

	// Test analysis
	paths := []string{normalFile, readOnlyFile, nonExistentFile}
	report := pm.AnalyzePermissions(paths, getSizeFunc)

	// Verify report
	if len(report.NormalFiles) == 0 {
		t.Error("Expected at least one normal file in report")
	}

	// Verify total sizes are calculated
	if report.TotalNormalSize == 0 && len(report.NormalFiles) > 0 {
		t.Error("Expected TotalNormalSize > 0 when normal files exist")
	}

	// Clean up
	os.Remove(normalFile)
	os.Remove(readOnlyFile)
}

func TestPermissionManager_IsRunningAsRoot(t *testing.T) {
	pm := NewPermissionManager()

	// Just verify it returns a boolean without error
	isRoot := pm.IsRunningAsRoot()

	// We can't reliably test the value since tests may run as root or not
	_ = isRoot
}

func TestPermissionManager_RequiresElevation(t *testing.T) {
	pm := NewPermissionManager()

	// Create test directory structure
	tempDir := os.TempDir()
	testDir := filepath.Join(tempDir, "cleanup-test-elevation")
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	// Create test file
	testFile := filepath.Join(testDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	defer os.Remove(testFile)

	requiresElevation := pm.RequiresElevation(testFile)

	// If running as root, should not require elevation
	if pm.IsRunningAsRoot() {
		if requiresElevation {
			t.Error("Running as root should not require elevation")
		}
	}
	// If not root, we can delete our own file in our own directory
	// so it should not require elevation
}

func TestIsSafeToDelete(t *testing.T) {
	// Create test directory structure
	tempDir := os.TempDir()
	testDir := filepath.Join(tempDir, "cleanup-test-safe")
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	tests := []struct {
		name      string
		setup     func() string
		cleanup   func(string)
		shouldErr bool
	}{
		{
			name: "regular file - safe",
			setup: func() string {
				path := filepath.Join(testDir, "safe.txt")
				os.WriteFile(path, []byte("test"), 0644)
				return path
			},
			cleanup:   func(p string) { os.Remove(p) },
			shouldErr: false,
		},
		{
			name: "non-existent file",
			setup: func() string {
				return filepath.Join(testDir, "non-existent.txt")
			},
			cleanup:   func(p string) {},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testPath := tt.setup()
			defer tt.cleanup(testPath)

			err := IsSafeToDelete(testPath)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for %s, got: %v", tt.name, err)
				}
			}
		})
	}
}

// Helper function
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
