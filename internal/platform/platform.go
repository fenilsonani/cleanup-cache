package platform

import (
	"os"
	"os/user"
	"runtime"
)

// Platform represents the operating system platform
type Platform string

const (
	MacOS   Platform = "darwin"
	Linux   Platform = "linux"
	Unknown Platform = "unknown"
)

// Info contains platform-specific information and paths
type Info struct {
	OS             Platform
	HomeDir        string
	Username       string
	CacheDirs      []string
	TempDirs       []string
	LogDirs        []string
	DownloadsDir   string
	SystemCaches   []string
	ProtectedPaths []string
}

// Detect returns the current platform
func Detect() Platform {
	switch runtime.GOOS {
	case "darwin":
		return MacOS
	case "linux":
		return Linux
	default:
		return Unknown
	}
}

// GetInfo returns platform-specific information
func GetInfo() (*Info, error) {
	platform := Detect()

	// Get current user info
	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}

	homeDir := currentUser.HomeDir
	username := currentUser.Username

	var info *Info

	switch platform {
	case MacOS:
		info = getMacOSInfo(homeDir, username)
	case Linux:
		info = getLinuxInfo(homeDir, username)
	default:
		return nil, ErrUnsupportedPlatform
	}

	return info, nil
}

// GetUserCacheDir returns the user's cache directory
func GetUserCacheDir() (string, error) {
	switch Detect() {
	case MacOS:
		return os.UserCacheDir()
	case Linux:
		// Try XDG_CACHE_HOME first
		if cacheDir := os.Getenv("XDG_CACHE_HOME"); cacheDir != "" {
			return cacheDir, nil
		}
		// Fall back to ~/.cache
		currentUser, err := user.Current()
		if err != nil {
			return "", err
		}
		return currentUser.HomeDir + "/.cache", nil
	default:
		return "", ErrUnsupportedPlatform
	}
}

// GetUserConfigDir returns the user's config directory
func GetUserConfigDir() (string, error) {
	switch Detect() {
	case MacOS:
		currentUser, err := user.Current()
		if err != nil {
			return "", err
		}
		return currentUser.HomeDir + "/Library/Application Support", nil
	case Linux:
		// Try XDG_CONFIG_HOME first
		if configDir := os.Getenv("XDG_CONFIG_HOME"); configDir != "" {
			return configDir, nil
		}
		// Fall back to ~/.config
		currentUser, err := user.Current()
		if err != nil {
			return "", err
		}
		return currentUser.HomeDir + "/.config", nil
	default:
		return "", ErrUnsupportedPlatform
	}
}

// IsProtectedPath checks if a path is protected and should never be deleted
func IsProtectedPath(path string) bool {
	protectedPaths := []string{
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
		"/System",         // macOS
		"/Applications",   // macOS
		"/Library/System", // macOS
	}

	for _, protected := range protectedPaths {
		if path == protected {
			return true
		}
	}

	return false
}

// Errors
var (
	ErrUnsupportedPlatform = &PlatformError{"unsupported platform"}
)

// PlatformError represents a platform-related error
type PlatformError struct {
	Message string
}

func (e *PlatformError) Error() string {
	return e.Message
}
