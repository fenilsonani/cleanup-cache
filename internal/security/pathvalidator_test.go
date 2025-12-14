package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePathForDeletion(t *testing.T) {
	pv := NewPathValidator()

	tests := []struct {
		name        string
		path        string
		setup       func() string // Returns actual path to test
		cleanup     func(string)
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "absolute path - valid",
			path:        "/tmp/test-cleanup-file.txt",
			setup:       func() string { return "/tmp/test-cleanup-file.txt" },
			cleanup:     func(p string) {},
			shouldError: false,
		},
		{
			name:        "relative path - invalid",
			path:        "relative/path.txt",
			setup:       func() string { return "relative/path.txt" },
			cleanup:     func(p string) {},
			shouldError: true,
			errorMsg:    "path must be absolute",
		},
		{
			name: "symlink to valid location - should resolve",
			setup: func() string {
				// Create temp file
				tmpFile := filepath.Join(os.TempDir(), "test-target.txt")
				os.WriteFile(tmpFile, []byte("test"), 0644)

				// Create symlink
				symlinkPath := filepath.Join(os.TempDir(), "test-symlink.txt")
				os.Symlink(tmpFile, symlinkPath)

				return symlinkPath
			},
			cleanup: func(p string) {
				os.Remove(p)
				os.Remove(filepath.Join(os.TempDir(), "test-target.txt"))
			},
			shouldError: false,
		},
		{
			name:        "path with null bytes - invalid",
			path:        "/tmp/test\x00malicious",
			setup:       func() string { return "/tmp/test\x00malicious" },
			cleanup:     func(p string) {},
			shouldError: true,
			errorMsg:    "invalid argument", // OS error, not our validation
		},
		{
			name:        "path with newline - invalid",
			path:        "/tmp/test\nmalicious",
			setup:       func() string { return "/tmp/test\nmalicious" },
			cleanup:     func(p string) {},
			shouldError: true,
			errorMsg:    "dangerous characters",
		},
		{
			name:        "empty path - invalid",
			path:        "",
			setup:       func() string { return "" },
			cleanup:     func(p string) {},
			shouldError: true,
			errorMsg:    "path must be absolute",
		},
		{
			name:        "root directory - protected",
			path:        "/",
			setup:       func() string { return "/" },
			cleanup:     func(p string) {},
			shouldError: true,
			errorMsg:    "protected path",
		},
		{
			name:        "/bin directory - protected",
			path:        "/bin",
			setup:       func() string { return "/bin" },
			cleanup:     func(p string) {},
			shouldError: true,
			errorMsg:    "refusing to delete protected path",
		},
		{
			name:        "/etc/direct-child - protected",
			path:        "/etc/newfile",
			setup:       func() string { return "/etc/newfile" },
			cleanup:     func(p string) {},
			shouldError: true,
			errorMsg:    "critical system path",
		},
		{
			name:        "/usr/newdir - protected (1 level)",
			path:        "/usr/newdir",
			setup:       func() string { return "/usr/newdir" },
			cleanup:     func(p string) {},
			shouldError: true,
			errorMsg:    "critical system path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testPath := tt.path
			if tt.setup != nil {
				testPath = tt.setup()
			}

			if tt.cleanup != nil {
				defer tt.cleanup(testPath)
			}

			err := pv.ValidatePathForDeletion(testPath)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestIsProtectedPath(t *testing.T) {
	pv := NewPathValidator()

	tests := []struct {
		name        string
		path        string
		isProtected bool
	}{
		{"root directory", "/", true},
		{"etc directory", "/etc", true},
		{"usr directory", "/usr", true},
		{"bin directory", "/bin", true},
		{"sbin directory", "/sbin", true},
		{"boot directory", "/boot", true},
		{"system directory (macOS)", "/System", true},
		{"applications directory (macOS)", "/Applications", true},
		{"file in etc", "/etc/hosts", true},
		{"file in System", "/System/Library/test", true},
		{"file in usr", "/usr/bin/ls", true},
		{"temp file", "/tmp/test.txt", false},
		{"var cache (not in protected list)", "/var/cache/test", true}, // /var is protected
		{"user cache", "/Users/test/.cache/test", false},
		{"home user subdir", "/home/user/Downloads/test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pv.IsProtectedPath(tt.path)
			if result != tt.isProtected {
				t.Errorf("IsProtectedPath(%s) = %v, want %v", tt.path, result, tt.isProtected)
			}
		})
	}
}

func TestValidateGlobPattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		shouldError bool
	}{
		{"simple wildcard", "*.txt", false},
		{"double wildcard", "**/*.log", false},
		{"character class", "[abc]*.txt", false},
		{"negation", "!*.tmp", false},
		{"multiple patterns", "*.{txt,log}", false},
		{"question mark", "file?.txt", false},
		{"valid complex pattern", "/var/log/**/*.log.[0-9]", false},
		{"empty pattern", "", false}, // filepath.Match allows empty patterns
		{"invalid syntax - unmatched bracket", "[abc", true},
		{"invalid syntax - unmatched brace", "{abc", false}, // Braces not special in filepath.Match
		{"pattern with traversal", "../*.txt", true},        // Should be rejected
		{"absolute path pattern", "/etc/*", false},
		{"home directory pattern", "~/.cache/*", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGlobPattern(tt.pattern)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for pattern '%s', got nil", tt.pattern)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for pattern '%s', got: %v", tt.pattern, err)
				}
			}
		})
	}
}

func TestPathCleaning(t *testing.T) {
	pv := NewPathValidator()

	tests := []struct {
		name        string
		path        string
		shouldError bool
		errorMsg    string
	}{
		{"path with dot segments", "/tmp/../var/test.txt", true, "suspicious elements"},
		{"path with double slashes", "/tmp//test//file.txt", true, "suspicious elements"},
		{"path with trailing slash", "/tmp/test/", true, "suspicious elements"},
		{"clean absolute path", "/tmp/test.txt", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pv.ValidatePathForDeletion(tt.path)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for path '%s', got nil", tt.path)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for path '%s', got: %v", tt.path, err)
				}
			}
		})
	}
}

func TestDangerousCharacters(t *testing.T) {
	pv := NewPathValidator()

	tests := []struct {
		name     string
		path     string
		hasError bool
	}{
		{"clean path", "/tmp/test.txt", false},
		{"path with spaces", "/tmp/test file.txt", false},
		{"path with dash", "/tmp/test-file.txt", false},
		{"path with underscore", "/tmp/test_file.txt", false},
		{"path with semicolon", "/tmp/test;file.txt", true},
		{"path with pipe", "/tmp/test|file.txt", true},
		{"path with newline", "/tmp/test\nfile.txt", true},
		{"path with dollar sign", "/tmp/test$file.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pv.ValidatePathForDeletion(tt.path)

			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for path with dangerous chars '%s', got nil", tt.path)
				}
			} else {
				if err != nil && !contains(err.Error(), "protected") {
					// Only fail if it's not a protected path error
					t.Errorf("Expected no error for clean path '%s', got: %v", tt.path, err)
				}
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr) && hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
