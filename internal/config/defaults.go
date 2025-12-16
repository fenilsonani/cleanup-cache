package config

// GetDefault returns the default configuration
func GetDefault() *Config {
	return &Config{
		Categories: Categories{
			Cache:           true,
			Temp:            true,
			Logs:            true,
			Downloads:       false, // Disabled by default to prevent accidental deletion
			PackageManagers: true,
			Docker:          false, // Disabled by default - requires explicit opt-in
			// Development artifact categories - enabled by default
			NodeModules:    true,
			VirtualEnvs:    true,
			BuildArtifacts: true,
			// Large and old file categories - enabled (uses Spotlight for speed)
			LargeFiles: true,
			OldFiles:   true,
		},
		AgeThresholds: AgeThresholds{
			Logs:      30, // 30 days
			Downloads: 90, // 90 days
			Temp:      7,  // 7 days
		},
		SizeLimits: SizeLimits{
			MinFileSize: "1KB",
			MaxFileSize: "10GB",
		},
		ExcludePattern: []string{
			"*/important/*",
			"*.keep",
			"*/Documents/*",
			"*/Pictures/*",
			"*/Music/*",
			"*/Videos/*",
			"*/Movies/*",
		},
		WhitelistPaths: []string{
			// User can add paths they want to explicitly protect
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
			"/boot",
			"/lib",
			"/lib64",
			"/opt",
			"/proc",
			"/root",
			"/run",
			"/srv",
			"/sys",
		},
		DryRun:     false, // Production default - actually delete files
		MinFileAge: 1,     // 1 hour - never delete files younger than this
		Verbose:    false,
		Docker: DockerConfig{
			Enabled:               false,
			CleanImages:           true,
			CleanContainers:       true,
			CleanVolumes:          false, // Volumes disabled by default - may contain data
			CleanBuildCache:       true,
			OnlyDanglingImages:    true, // Only clean dangling images by default
			OnlyStoppedContainers: true, // Only clean stopped containers
			OnlyUnusedVolumes:     true, // Only clean unused volumes
			ImageAgeDays:          7,    // Clean images older than 7 days
			ContainerAgeDays:      1,    // Clean containers older than 1 day
			KeepImages:            []string{},
			KeepContainers:        []string{},
			KeepVolumes:           []string{},
		},
		SecureDeletion: SecureDeletionConfig{
			Enabled:      false,           // Disabled by default
			Standard:     "dod522022",     // DoD 5220.22-M standard
			CustomPasses: 3,               // 3 passes for custom
			VerifyWrites: true,            // Verify overwrites
			ForceSync:    true,            // Force sync to disk
			BufferSizeKB: 64,              // 64KB buffer
		},
		Dev: DevConfig{
			ProjectDirs: []string{
				"~/Projects",
				"~/Developer",
				"~/Code",
				"~/work",
				"~/src",
				"~/repos",
			},
			BuildPatterns: []string{
				"node_modules",
				".next",
				"dist",
				"build",
				"target",
				"__pycache__",
				".gradle",
				"*.egg-info",
				".tox",
				".pytest_cache",
				"vendor",
				".bundle",
				"Pods",
			},
		},
		LargeFiles: LargeFilesConfig{
			MinSize: "500MB",
			ScanPaths: []string{
				"~",
			},
			ExcludePaths: []string{
				"~/Library",
				"~/.Trash",
				"/System",
				"/Applications",
				"~/.local",
			},
			FileTypes: []string{
				".mp4", ".mkv", ".avi", ".mov",
				".iso", ".dmg",
				".zip", ".tar.gz", ".rar", ".7z",
			},
		},
		OldFiles: OldFilesConfig{
			MinAgeDays: 180, // 6 months
			ScanPaths: []string{
				"~/Downloads",
				"~/Documents",
				"~/Desktop",
			},
			ExcludePaths: []string{
				"~/Documents/Work",
				"~/Documents/Important",
			},
		},
	}
}

// GetExampleConfig returns an example configuration with comments
func GetExampleConfig() string {
	return `# TidyUp Configuration File
# This file controls what gets cleaned and how
# Location: ~/.config/tidyup/config.yaml

# Enable/disable cleanup categories
categories:
  cache: true            # Browser caches, app caches, system caches
  temp: true             # Temporary files
  logs: true             # Log files
  downloads: false       # Old files in Downloads folder (CAUTION: Review before enabling)
  package_managers: true # Package manager caches (brew, apt, npm, etc.)
  docker: false          # Docker cleanup (requires Docker to be installed)
  # Development artifact categories
  node_modules: true     # node_modules folders
  virtual_envs: true     # Python virtual environments (.venv, venv, etc.)
  build_artifacts: true  # Build output folders (dist, build, target, etc.)
  # Large and old file scanning
  large_files: true      # Find large files (uses Spotlight for fast scanning)
  old_files: true        # Find old unused files

# Age thresholds (in days) - Only clean files older than these thresholds
age_thresholds:
  logs: 30        # Clean log files older than 30 days
  downloads: 90   # Clean downloads older than 90 days
  temp: 7         # Clean temp files older than 7 days

# Size limits for files to consider
size_limits:
  min_file_size: "1KB"   # Ignore files smaller than this
  max_file_size: "10GB"  # Skip files larger than this (safety measure)

# Exclude patterns (glob patterns)
# Files matching these patterns will be skipped
exclude_patterns:
  - "*/important/*"
  - "*.keep"
  - "*/Documents/*"
  - "*/Pictures/*"
  - "*/Music/*"
  - "*/Videos/*"
  - "*/Movies/*"

# Whitelist paths - Explicitly protect these paths
# Paths listed here will never be cleaned
whitelist_paths:
  - "/Users/*/Projects"
  - "/Users/*/Work"

# Protected paths - System-critical paths that should never be touched
# These are already protected by default, but you can add more
protected_paths:
  - "/"
  - "/System"
  - "/Applications"
  - "/usr"
  - "/etc"
  - "/var"

# Dry-run mode - When true, shows what would be deleted without actually deleting
# Set to false to actually delete files (default in production)
dry_run: false

# Minimum file age (in hours) - Never delete files younger than this
# This is a safety measure to prevent deleting recently created/modified files
min_file_age: 1

# Verbose output - Show detailed information during execution
verbose: false

# ==============================================================================
# DEVELOPMENT ARTIFACTS CONFIGURATION
# ==============================================================================
# Scan project directories for build artifacts and dependencies that can be rebuilt

dev:
  # Directories where your projects are located
  # Use ~ for home directory, e.g., ~/Developer
  project_dirs:
    - "~/Projects"
    - "~/Developer"
    - "~/Code"
    - "~/work"
    - "~/src"
    - "~/repos"

  # Patterns to match as build artifacts
  # These are folders/files that can safely be deleted and rebuilt
  build_patterns:
    - "node_modules"     # Node.js dependencies
    - ".next"            # Next.js build output
    - "dist"             # Common build output folder
    - "build"            # Common build output folder
    - "target"           # Rust/Java build output
    - "__pycache__"      # Python bytecode cache
    - ".gradle"          # Gradle cache
    - "*.egg-info"       # Python egg info
    - ".tox"             # Python tox testing
    - ".pytest_cache"    # Pytest cache
    - "vendor"           # Go/PHP vendor dependencies
    - ".bundle"          # Ruby bundler
    - "Pods"             # iOS CocoaPods

# ==============================================================================
# LARGE FILES CONFIGURATION
# ==============================================================================
# Find large files that may be taking up unnecessary space

large_files:
  # Minimum size to consider as "large"
  min_size: "500MB"

  # Paths to scan for large files
  scan_paths:
    - "~"

  # Paths to exclude from large file scanning
  exclude_paths:
    - "~/Library"
    - "~/.Trash"
    - "/System"
    - "/Applications"
    - "~/.local"

  # File extensions to look for (leave empty for all types)
  file_types:
    - ".mp4"
    - ".mkv"
    - ".avi"
    - ".mov"
    - ".iso"
    - ".dmg"
    - ".zip"
    - ".tar.gz"
    - ".rar"
    - ".7z"

# ==============================================================================
# OLD FILES CONFIGURATION
# ==============================================================================
# Find files that haven't been accessed in a long time

old_files:
  # Minimum age in days (files not accessed for this many days)
  min_age_days: 180  # 6 months

  # Paths to scan for old files
  scan_paths:
    - "~/Downloads"
    - "~/Documents"
    - "~/Desktop"

  # Paths to exclude from old file scanning
  exclude_paths:
    - "~/Documents/Work"
    - "~/Documents/Important"

# ==============================================================================
# DOCKER CONFIGURATION
# ==============================================================================
# Configure Docker cleanup options (only used when docker category is enabled)

docker:
  enabled: false
  clean_images: true
  clean_containers: true
  clean_volumes: false          # Volumes may contain data - disabled by default
  clean_build_cache: true
  only_dangling_images: true    # Only remove images without tags
  only_stopped_containers: true # Only remove stopped containers
  only_unused_volumes: true     # Only remove volumes not in use
  image_age_days: 7             # Remove images older than 7 days
  container_age_days: 1         # Remove containers older than 1 day
  keep_images: []               # Image names/tags to never delete
  keep_containers: []           # Container names to never delete
  keep_volumes: []              # Volume names to never delete

# ==============================================================================
# SECURE DELETION CONFIGURATION
# ==============================================================================
# For sensitive data, use secure deletion (slower but more secure)

secure_deletion:
  enabled: false
  standard: "dod522022"  # Options: simple, dod522022, gutmann, custom
  custom_passes: 3       # Number of passes for custom standard
  verify_writes: true    # Verify overwrites completed
  force_sync: true       # Force sync to disk after each pass
  buffer_size_kb: 64     # Buffer size in KB
`
}
