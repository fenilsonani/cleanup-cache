package platform

import "path/filepath"

// getLinuxInfo returns platform-specific information for Linux
func getLinuxInfo(homeDir, username string) *Info {
	return &Info{
		OS:       Linux,
		HomeDir:  homeDir,
		Username: username,
		CacheDirs: []string{
			filepath.Join(homeDir, ".cache"),
			"/var/cache",
			"/tmp",
		},
		TempDirs: []string{
			"/tmp",
			"/var/tmp",
			filepath.Join(homeDir, ".local/share/Trash"),
		},
		LogDirs: []string{
			"/var/log",
			filepath.Join(homeDir, ".local/share/logs"),
			filepath.Join(homeDir, ".xsession-errors"),
		},
		DownloadsDir: filepath.Join(homeDir, "Downloads"),
		SystemCaches: []string{
			// Package manager caches
			"/var/cache/apt/archives",
			"/var/cache/yum",
			"/var/cache/dnf",
			"/var/cache/pacman",
			// Browser caches
			filepath.Join(homeDir, ".cache/google-chrome"),
			filepath.Join(homeDir, ".cache/chromium"),
			filepath.Join(homeDir, ".cache/mozilla/firefox"),
			filepath.Join(homeDir, ".cache/microsoft-edge"),
			filepath.Join(homeDir, ".mozilla/firefox/*/cache2"),
			// Developer tools
			filepath.Join(homeDir, ".cache/go-build"),
			filepath.Join(homeDir, ".cache/pip"),
			filepath.Join(homeDir, ".cache/yarn"),
			filepath.Join(homeDir, ".cache/npm"),
			filepath.Join(homeDir, ".cache/cargo"),
			filepath.Join(homeDir, ".npm"),
			filepath.Join(homeDir, ".yarn/cache"),
			filepath.Join(homeDir, ".cargo/registry/cache"),
			// Gradle
			filepath.Join(homeDir, ".gradle/caches"),
			// Maven
			filepath.Join(homeDir, ".m2/repository"),
			// Docker (user must have permission)
			"/var/lib/docker/tmp",
			// Thumbnails
			filepath.Join(homeDir, ".cache/thumbnails"),
			filepath.Join(homeDir, ".thumbnails"),
			// Font cache
			filepath.Join(homeDir, ".cache/fontconfig"),
			// Mesa (graphics) cache
			filepath.Join(homeDir, ".cache/mesa_shader_cache"),
		},
		ProtectedPaths: []string{
			"/",
			"/bin",
			"/boot",
			"/dev",
			"/etc",
			"/home",
			"/lib",
			"/lib64",
			"/opt",
			"/proc",
			"/root",
			"/run",
			"/sbin",
			"/srv",
			"/sys",
			"/usr",
			"/var/lib",
			"/var/db",
			filepath.Join(homeDir, ".config"),
			filepath.Join(homeDir, ".local/share"),
			filepath.Join(homeDir, "Documents"),
			filepath.Join(homeDir, "Desktop"),
			filepath.Join(homeDir, "Pictures"),
			filepath.Join(homeDir, "Music"),
			filepath.Join(homeDir, "Videos"),
		},
	}
}

// GetAptCachePath returns the APT cache path
func GetAptCachePath() string {
	return "/var/cache/apt/archives"
}

// GetYumCachePath returns the YUM cache path
func GetYumCachePath() string {
	return "/var/cache/yum"
}

// GetDnfCachePath returns the DNF cache path
func GetDnfCachePath() string {
	return "/var/cache/dnf"
}

// GetPacmanCachePath returns the Pacman cache path
func GetPacmanCachePath() string {
	return "/var/cache/pacman/pkg"
}

// GetSnapCachePath returns the Snap cache path
func GetSnapCachePath() string {
	return "/var/lib/snapd/cache"
}

// GetFlatpakCachePath returns the Flatpak cache path
func GetFlatpakCachePath(homeDir string) string {
	return filepath.Join(homeDir, ".var/app/*/cache")
}
