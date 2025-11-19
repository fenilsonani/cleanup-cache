package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PathValidator handles secure path validation for file operations
type PathValidator struct {
	protectedPaths []string
}

// NewPathValidator creates a new PathValidator with default protected paths
func NewPathValidator() *PathValidator {
	return &PathValidator{
		protectedPaths: []string{
			// Unix system directories
			"/",
			"/bin",
			"/boot",
			"/dev",
			"/etc",
			"/lib",
			"/lib64",
			"/proc",
			"/root",
			"/sbin",
			"/sys",
			"/usr",
			"/var",
			// macOS system directories
			"/System",
			"/Applications",
			"/Library/System",
		},
	}
}

// ValidatePathForDeletion performs comprehensive validation on a path before deletion
// This is the single source of truth for all path validation in the application
func (pv *PathValidator) ValidatePathForDeletion(path string) error {
	// Step 1: Path must be absolute
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be absolute: %s", path)
	}

	// Step 2: Resolve symlinks to prevent traversal attacks
	// SECURITY: This prevents attacks like ~/cache/../../etc/passwd
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If symlink resolution fails, check if it's because file doesn't exist
		if os.IsNotExist(err) {
			// File doesn't exist yet, validate the path itself
			resolvedPath = path
		} else {
			return fmt.Errorf("failed to resolve symlinks: %w", err)
		}
	}

	// Step 3: Clean the resolved path
	cleanPath := filepath.Clean(resolvedPath)

	// Step 4: Validate against original path to detect suspicious elements
	// If the cleaned path is very different, something suspicious may be happening
	originalClean := filepath.Clean(path)
	if originalClean != path {
		return fmt.Errorf("path contains suspicious elements: %s", path)
	}

	// Step 5: Check for dangerous shell metacharacters
	// These could be used for command injection if path is ever used in shell
	dangerousChars := []string{";", "&", "|", "$", "`", "(", ")", "<", ">", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(cleanPath, char) {
			return fmt.Errorf("path contains dangerous characters: %s", cleanPath)
		}
	}

	// Step 6: Check against protected paths
	if err := pv.checkProtectedPaths(cleanPath); err != nil {
		return err
	}

	return nil
}

// checkProtectedPaths validates that a path is not in a protected system directory
func (pv *PathValidator) checkProtectedPaths(cleanPath string) error {
	for _, protected := range pv.protectedPaths {
		// Exact match
		if cleanPath == protected {
			return fmt.Errorf("refusing to delete protected path: %s", cleanPath)
		}

		// Check if path is directly under a protected directory
		// More dangerous: /usr/foo vs /usr/local/cache/foo
		if strings.HasPrefix(cleanPath, protected+"/") {
			// Check if it's only one level deep
			rel, _ := filepath.Rel(protected, cleanPath)
			if !strings.Contains(rel, "/") {
				return fmt.Errorf("refusing to delete critical system path: %s", cleanPath)
			}
		}
	}

	return nil
}

// IsProtectedPath checks if a path is a protected system path
func (pv *PathValidator) IsProtectedPath(path string) bool {
	cleanPath := filepath.Clean(path)
	for _, protected := range pv.protectedPaths {
		if cleanPath == protected || strings.HasPrefix(cleanPath, protected+"/") {
			return true
		}
	}
	return false
}

// AddProtectedPath adds a custom protected path
func (pv *PathValidator) AddProtectedPath(path string) {
	cleanPath := filepath.Clean(path)
	pv.protectedPaths = append(pv.protectedPaths, cleanPath)
}

// ValidateGlobPattern validates that a glob pattern is safe
func ValidateGlobPattern(pattern string) error {
	// Check for dangerous characters
	if strings.Contains(pattern, "..") {
		return fmt.Errorf("glob pattern contains directory traversal: %s", pattern)
	}

	// Try to match the pattern to ensure it's valid
	_, err := filepath.Match(pattern, "test")
	if err != nil {
		return fmt.Errorf("invalid glob pattern: %w", err)
	}

	return nil
}
