package platform

import "path/filepath"

// getMacOSInfo returns platform-specific information for macOS
func getMacOSInfo(homeDir, username string) *Info {
	return &Info{
		OS:       MacOS,
		HomeDir:  homeDir,
		Username: username,
		CacheDirs: []string{
			filepath.Join(homeDir, "Library/Caches"),
			"/Library/Caches",
			"/System/Library/Caches",
		},
		TempDirs: []string{
			"/tmp",
			"/var/tmp",
			"/private/tmp",
			"/private/var/tmp",
			filepath.Join(homeDir, ".Trash"),
		},
		LogDirs: []string{
			filepath.Join(homeDir, "Library/Logs"),
			"/Library/Logs",
			"/var/log",
			"/private/var/log",
		},
		DownloadsDir: filepath.Join(homeDir, "Downloads"),
		SystemCaches: []string{
			// Homebrew
			"/Library/Caches/Homebrew",
			filepath.Join(homeDir, "Library/Caches/Homebrew"),
			// Browser caches
			filepath.Join(homeDir, "Library/Caches/Google/Chrome"),
			filepath.Join(homeDir, "Library/Caches/Firefox"),
			filepath.Join(homeDir, "Library/Caches/com.apple.Safari"),
			filepath.Join(homeDir, "Library/Caches/Microsoft Edge"),
			// Developer tools
			filepath.Join(homeDir, "Library/Caches/go-build"),
			filepath.Join(homeDir, "Library/Caches/pip"),
			filepath.Join(homeDir, "Library/Caches/yarn"),
			filepath.Join(homeDir, "Library/Caches/npm"),
			filepath.Join(homeDir, ".cache/cargo"),
			filepath.Join(homeDir, ".npm"),
			filepath.Join(homeDir, ".yarn/cache"),
			// Xcode
			filepath.Join(homeDir, "Library/Developer/Xcode/DerivedData"),
			filepath.Join(homeDir, "Library/Developer/Xcode/Archives"),
			filepath.Join(homeDir, "Library/Developer/CoreSimulator/Caches"),
			// CocoaPods
			filepath.Join(homeDir, "Library/Caches/CocoaPods"),
			// Gradle
			filepath.Join(homeDir, ".gradle/caches"),
			// Maven
			filepath.Join(homeDir, ".m2/repository"),
		},
		ProtectedPaths: []string{
			"/",
			"/System",
			"/Applications",
			"/Library/System",
			"/bin",
			"/sbin",
			"/usr",
			"/etc",
			"/var",
			"/dev",
			"/private/etc",
			"/private/var/db",
			filepath.Join(homeDir, "Library/Application Support"),
			filepath.Join(homeDir, "Library/Preferences"),
			filepath.Join(homeDir, "Documents"),
			filepath.Join(homeDir, "Desktop"),
			filepath.Join(homeDir, "Pictures"),
			filepath.Join(homeDir, "Music"),
			filepath.Join(homeDir, "Movies"),
		},
	}
}

// GetBrewCachePath returns the Homebrew cache path
func GetBrewCachePath() string {
	return "/Library/Caches/Homebrew"
}

// GetUserBrewCachePath returns the user's Homebrew cache path
func GetUserBrewCachePath(homeDir string) string {
	return filepath.Join(homeDir, "Library/Caches/Homebrew")
}

// GetXcodeDerivedDataPath returns the Xcode derived data path
func GetXcodeDerivedDataPath(homeDir string) string {
	return filepath.Join(homeDir, "Library/Developer/Xcode/DerivedData")
}

// GetSimulatorCachePath returns the iOS Simulator cache path
func GetSimulatorCachePath(homeDir string) string {
	return filepath.Join(homeDir, "Library/Developer/CoreSimulator/Caches")
}
