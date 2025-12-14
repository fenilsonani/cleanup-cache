package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fenilsonani/cleanup-cache/internal/security"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Categories       Categories         `yaml:"categories"`
	AgeThresholds    AgeThresholds      `yaml:"age_thresholds"`
	SizeLimits       SizeLimits         `yaml:"size_limits"`
	ExcludePattern   []string           `yaml:"exclude_patterns"`
	WhitelistPaths   []string           `yaml:"whitelist_paths"`
	ProtectedPaths   []string           `yaml:"protected_paths"`
	DryRun           bool               `yaml:"dry_run"`
	MinFileAge       int                `yaml:"min_file_age"` // in hours
	Verbose          bool               `yaml:"verbose"`
	Docker           DockerConfig       `yaml:"docker"`
	SecureDeletion   SecureDeletionConfig `yaml:"secure_deletion"`
	Daemon           *DaemonConfig      `yaml:"daemon,omitempty"`
}

// Categories defines which cleanup categories are enabled
type Categories struct {
	Cache           bool `yaml:"cache"`
	Temp            bool `yaml:"temp"`
	Logs            bool `yaml:"logs"`
	Downloads       bool `yaml:"downloads"`
	PackageManagers bool `yaml:"package_managers"`
	Docker          bool `yaml:"docker"`
}

// DockerConfig holds Docker cleanup configuration
type DockerConfig struct {
	Enabled               bool     `yaml:"enabled"`
	CleanImages           bool     `yaml:"clean_images"`
	CleanContainers       bool     `yaml:"clean_containers"`
	CleanVolumes          bool     `yaml:"clean_volumes"`
	CleanBuildCache       bool     `yaml:"clean_build_cache"`
	OnlyDanglingImages    bool     `yaml:"only_dangling_images"`
	OnlyStoppedContainers bool     `yaml:"only_stopped_containers"`
	OnlyUnusedVolumes     bool     `yaml:"only_unused_volumes"`
	ImageAgeDays          int      `yaml:"image_age_days"`
	ContainerAgeDays      int      `yaml:"container_age_days"`
	KeepImages            []string `yaml:"keep_images"`
	KeepContainers        []string `yaml:"keep_containers"`
	KeepVolumes           []string `yaml:"keep_volumes"`
}

// SecureDeletionConfig holds secure deletion configuration
type SecureDeletionConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Standard       string `yaml:"standard"` // "dod522022", "gutmann", "random", "none"
	CustomPasses   int    `yaml:"custom_passes"`
	VerifyWrites   bool   `yaml:"verify_writes"`
	ForceSync      bool   `yaml:"force_sync"`
	BufferSizeKB   int    `yaml:"buffer_size_kb"`
}

// DaemonConfig holds daemon mode configuration
type DaemonConfig struct {
	Enabled       bool              `yaml:"enabled"`
	PidFile       string            `yaml:"pid_file"`
	LogFile       string            `yaml:"log_file"`
	LogLevel      string            `yaml:"log_level"`
	Schedules     []CleanupSchedule `yaml:"schedules"`
	Notifications NotificationConfig `yaml:"notifications"`
}

// CleanupSchedule defines a scheduled cleanup
type CleanupSchedule struct {
	Name        string          `yaml:"name"`
	Schedule    string          `yaml:"schedule"` // Cron expression
	Categories  map[string]bool `yaml:"categories"`
	DryRun      bool            `yaml:"dry_run"`
	SkipIfBusy  bool            `yaml:"skip_if_busy"`
}

// NotificationConfig holds notification settings
type NotificationConfig struct {
	Enabled   bool         `yaml:"enabled"`
	OnSuccess bool         `yaml:"on_success"`
	OnFailure bool         `yaml:"on_failure"`
	Email     EmailConfig  `yaml:"email"`
	Webhook   WebhookConfig `yaml:"webhook"`
}

// EmailConfig holds email notification settings
type EmailConfig struct {
	SMTPHost string   `yaml:"smtp_host"`
	SMTPPort int      `yaml:"smtp_port"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
	From     string   `yaml:"from"`
	To       []string `yaml:"to"`
	UseTLS   bool     `yaml:"use_tls"`
}

// WebhookConfig holds webhook notification settings
type WebhookConfig struct {
	URL     string            `yaml:"url"`
	Method  string            `yaml:"method"`
	Headers map[string]string `yaml:"headers"`
}


// AgeThresholds defines age thresholds for different categories (in days)
type AgeThresholds struct {
	Logs      int `yaml:"logs"`
	Downloads int `yaml:"downloads"`
	Temp      int `yaml:"temp"`
}

// SizeLimits defines size limits for files to consider
type SizeLimits struct {
	MinFileSize string `yaml:"min_file_size"` // e.g., "1KB"
	MaxFileSize string `yaml:"max_file_size"` // e.g., "10GB"
}

// Load loads configuration from a file
func Load(configPath string) (*Config, error) {
	// If config doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return GetDefault(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Save saves configuration to a file
func Save(config *Config, configPath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate age thresholds
	if c.AgeThresholds.Logs < 0 {
		return fmt.Errorf("logs age threshold must be >= 0")
	}
	if c.AgeThresholds.Downloads < 0 {
		return fmt.Errorf("downloads age threshold must be >= 0")
	}
	if c.AgeThresholds.Temp < 0 {
		return fmt.Errorf("temp age threshold must be >= 0")
	}

	// Validate min file age
	if c.MinFileAge < 0 {
		return fmt.Errorf("min file age must be >= 0")
	}

	// Validate exclude patterns (glob syntax)
	for _, pattern := range c.ExcludePattern {
		if err := security.ValidateGlobPattern(pattern); err != nil {
			return fmt.Errorf("invalid exclude pattern '%s': %w", pattern, err)
		}
	}

	// Validate whitelist paths are absolute
	for _, path := range c.WhitelistPaths {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("whitelist path must be absolute: %s", path)
		}
	}

	// Validate protected paths are absolute
	for _, path := range c.ProtectedPaths {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("protected path must be absolute: %s", path)
		}
	}

	return nil
}

// GetConfigPath returns the default config path
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(homeDir, ".config", "cleanup-cache")
	return filepath.Join(configDir, "config.yaml"), nil
}

// EnsureConfigExists creates a default config file if it doesn't exist
func EnsureConfigExists() (string, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return "", err
	}

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		defaultConfig := GetDefault()
		if err := Save(defaultConfig, configPath); err != nil {
			return "", err
		}
	}

	return configPath, nil
}
