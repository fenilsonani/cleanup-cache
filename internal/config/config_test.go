package config

import (
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// GetDefault Tests
// =============================================================================

func TestGetDefault(t *testing.T) {
	cfg := GetDefault()

	if cfg == nil {
		t.Fatal("GetDefault returned nil")
	}

	// Check default categories
	if !cfg.Categories.Cache {
		t.Error("expected Cache to be enabled by default")
	}
	if !cfg.Categories.Temp {
		t.Error("expected Temp to be enabled by default")
	}
	if !cfg.Categories.Logs {
		t.Error("expected Logs to be enabled by default")
	}
	if cfg.Categories.Downloads {
		t.Error("expected Downloads to be disabled by default")
	}
	if cfg.Categories.Docker {
		t.Error("expected Docker to be disabled by default")
	}
	if !cfg.Categories.NodeModules {
		t.Error("expected NodeModules to be enabled by default")
	}
	if !cfg.Categories.VirtualEnvs {
		t.Error("expected VirtualEnvs to be enabled by default")
	}
	if !cfg.Categories.BuildArtifacts {
		t.Error("expected BuildArtifacts to be enabled by default")
	}
}

func TestGetDefaultAgeThresholds(t *testing.T) {
	cfg := GetDefault()

	if cfg.AgeThresholds.Logs != 30 {
		t.Errorf("expected Logs threshold 30, got %d", cfg.AgeThresholds.Logs)
	}
	if cfg.AgeThresholds.Downloads != 90 {
		t.Errorf("expected Downloads threshold 90, got %d", cfg.AgeThresholds.Downloads)
	}
	if cfg.AgeThresholds.Temp != 7 {
		t.Errorf("expected Temp threshold 7, got %d", cfg.AgeThresholds.Temp)
	}
}

func TestGetDefaultSizeLimits(t *testing.T) {
	cfg := GetDefault()

	if cfg.SizeLimits.MinFileSize != "1KB" {
		t.Errorf("expected MinFileSize '1KB', got %q", cfg.SizeLimits.MinFileSize)
	}
	if cfg.SizeLimits.MaxFileSize != "10GB" {
		t.Errorf("expected MaxFileSize '10GB', got %q", cfg.SizeLimits.MaxFileSize)
	}
}

func TestGetDefaultMinFileAge(t *testing.T) {
	cfg := GetDefault()

	// Default is 1 hour - never delete files younger than this
	if cfg.MinFileAge != 1 {
		t.Errorf("expected MinFileAge 1, got %d", cfg.MinFileAge)
	}
}

// =============================================================================
// Load Tests
// =============================================================================

func TestLoadNonExistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load should not error for non-existent file: %v", err)
	}

	// Should return default config
	if cfg == nil {
		t.Fatal("Load returned nil config")
	}
	if !cfg.Categories.Cache {
		t.Error("expected default Cache to be enabled")
	}
}

func TestLoadValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write a valid config
	configContent := `
categories:
  cache: true
  temp: false
  logs: true
  downloads: true
  docker: true
age_thresholds:
  logs: 15
  downloads: 45
  temp: 3
min_file_age: 48
dry_run: true
verbose: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check loaded values
	if !cfg.Categories.Cache {
		t.Error("expected Cache to be true")
	}
	if cfg.Categories.Temp {
		t.Error("expected Temp to be false")
	}
	if !cfg.Categories.Downloads {
		t.Error("expected Downloads to be true")
	}
	if !cfg.Categories.Docker {
		t.Error("expected Docker to be true")
	}
	if cfg.AgeThresholds.Logs != 15 {
		t.Errorf("expected Logs threshold 15, got %d", cfg.AgeThresholds.Logs)
	}
	if cfg.AgeThresholds.Downloads != 45 {
		t.Errorf("expected Downloads threshold 45, got %d", cfg.AgeThresholds.Downloads)
	}
	if cfg.MinFileAge != 48 {
		t.Errorf("expected MinFileAge 48, got %d", cfg.MinFileAge)
	}
	if !cfg.DryRun {
		t.Error("expected DryRun to be true")
	}
	if !cfg.Verbose {
		t.Error("expected Verbose to be true")
	}
}

func TestLoadPartialConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write a partial config - only override some values
	configContent := `
categories:
  downloads: true
min_file_age: 72
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check that defaults are preserved for unspecified values
	if cfg.AgeThresholds.Logs != 30 {
		t.Errorf("expected default Logs threshold 30, got %d", cfg.AgeThresholds.Logs)
	}
	// Check overridden values
	if !cfg.Categories.Downloads {
		t.Error("expected Downloads to be true (overridden)")
	}
	if cfg.MinFileAge != 72 {
		t.Errorf("expected MinFileAge 72, got %d", cfg.MinFileAge)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write invalid YAML
	configContent := `
categories:
  cache: [invalid
  temp: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadInvalidAgeThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write config with negative age threshold
	configContent := `
age_thresholds:
  logs: -5
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for negative age threshold")
	}
}

func TestLoadInvalidMinFileAge(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
min_file_age: -10
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for negative min_file_age")
	}
}

func TestLoadInvalidExcludePattern(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write config with invalid glob pattern
	configContent := `
exclude_patterns:
  - "[invalid"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for invalid exclude pattern")
	}
}

func TestLoadRelativeWhitelistPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write config with relative whitelist path
	configContent := `
whitelist_paths:
  - "relative/path"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for relative whitelist path")
	}
}

func TestLoadRelativeProtectedPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write config with relative protected path
	configContent := `
protected_paths:
  - "relative/protected"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for relative protected path")
	}
}

// =============================================================================
// Save Tests
// =============================================================================

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.yaml")

	cfg := GetDefault()
	cfg.Categories.Downloads = true
	cfg.MinFileAge = 100

	err := Save(cfg, configPath)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}

	// Load it back and verify
	loadedCfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	if !loadedCfg.Categories.Downloads {
		t.Error("expected Downloads to be true after save/load")
	}
	if loadedCfg.MinFileAge != 100 {
		t.Errorf("expected MinFileAge 100 after save/load, got %d", loadedCfg.MinFileAge)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "deep", "nested", "dir", "config.yaml")

	cfg := GetDefault()
	err := Save(cfg, configPath)
	if err != nil {
		t.Fatalf("Save failed to create nested directories: %v", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created in nested directory")
	}
}

// =============================================================================
// Validate Tests
// =============================================================================

func TestValidateValidConfig(t *testing.T) {
	cfg := GetDefault()
	err := cfg.Validate()
	if err != nil {
		t.Errorf("default config should be valid: %v", err)
	}
}

func TestValidateNegativeLogsThreshold(t *testing.T) {
	cfg := GetDefault()
	cfg.AgeThresholds.Logs = -1

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for negative logs threshold")
	}
}

func TestValidateNegativeDownloadsThreshold(t *testing.T) {
	cfg := GetDefault()
	cfg.AgeThresholds.Downloads = -1

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for negative downloads threshold")
	}
}

func TestValidateNegativeTempThreshold(t *testing.T) {
	cfg := GetDefault()
	cfg.AgeThresholds.Temp = -1

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for negative temp threshold")
	}
}

func TestValidateNegativeMinFileAge(t *testing.T) {
	cfg := GetDefault()
	cfg.MinFileAge = -1

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for negative min file age")
	}
}

func TestValidateInvalidExcludePattern(t *testing.T) {
	cfg := GetDefault()
	cfg.ExcludePattern = []string{"[invalid"}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid exclude pattern")
	}
}

func TestValidateValidExcludePatterns(t *testing.T) {
	cfg := GetDefault()
	cfg.ExcludePattern = []string{
		"*.log",
		"**/cache/*",
		"temp_*",
		"[abc]*.txt",
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("valid exclude patterns should pass: %v", err)
	}
}

func TestValidateRelativeWhitelistPath(t *testing.T) {
	cfg := GetDefault()
	cfg.WhitelistPaths = []string{"relative/path"}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for relative whitelist path")
	}
}

func TestValidateAbsoluteWhitelistPath(t *testing.T) {
	cfg := GetDefault()
	cfg.WhitelistPaths = []string{"/absolute/path"}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("absolute whitelist path should be valid: %v", err)
	}
}

func TestValidateRelativeProtectedPath(t *testing.T) {
	cfg := GetDefault()
	cfg.ProtectedPaths = []string{"relative/protected"}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for relative protected path")
	}
}

func TestValidateAbsoluteProtectedPath(t *testing.T) {
	cfg := GetDefault()
	cfg.ProtectedPaths = []string{"/absolute/protected"}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("absolute protected path should be valid: %v", err)
	}
}

func TestValidateZeroThresholds(t *testing.T) {
	cfg := GetDefault()
	cfg.AgeThresholds.Logs = 0
	cfg.AgeThresholds.Downloads = 0
	cfg.AgeThresholds.Temp = 0
	cfg.MinFileAge = 0

	err := cfg.Validate()
	if err != nil {
		t.Errorf("zero thresholds should be valid: %v", err)
	}
}

// =============================================================================
// GetConfigPath Tests
// =============================================================================

func TestGetConfigPath(t *testing.T) {
	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath failed: %v", err)
	}

	if path == "" {
		t.Error("GetConfigPath returned empty string")
	}

	// Should contain tidyup
	if !filepath.IsAbs(path) {
		t.Error("GetConfigPath should return absolute path")
	}

	// Should end with config.yaml
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("expected path to end with config.yaml, got %s", filepath.Base(path))
	}
}

// =============================================================================
// Docker Config Tests
// =============================================================================

func TestDockerConfigDefaults(t *testing.T) {
	cfg := GetDefault()

	if cfg.Docker.Enabled {
		t.Error("Docker should be disabled by default")
	}
}

func TestLoadDockerConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
docker:
  enabled: true
  clean_images: true
  clean_containers: true
  clean_volumes: false
  only_dangling_images: true
  image_age_days: 30
  keep_images:
    - "nginx:latest"
    - "postgres:14"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !cfg.Docker.Enabled {
		t.Error("expected Docker.Enabled to be true")
	}
	if !cfg.Docker.CleanImages {
		t.Error("expected Docker.CleanImages to be true")
	}
	if cfg.Docker.CleanVolumes {
		t.Error("expected Docker.CleanVolumes to be false")
	}
	if !cfg.Docker.OnlyDanglingImages {
		t.Error("expected Docker.OnlyDanglingImages to be true")
	}
	if cfg.Docker.ImageAgeDays != 30 {
		t.Errorf("expected ImageAgeDays 30, got %d", cfg.Docker.ImageAgeDays)
	}
	if len(cfg.Docker.KeepImages) != 2 {
		t.Errorf("expected 2 keep images, got %d", len(cfg.Docker.KeepImages))
	}
}

// =============================================================================
// Dev Config Tests
// =============================================================================

func TestLoadDevConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
dev:
  project_dirs:
    - "~/projects"
    - "~/work"
  build_patterns:
    - "dist"
    - "build"
    - ".next"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Dev.ProjectDirs) != 2 {
		t.Errorf("expected 2 project dirs, got %d", len(cfg.Dev.ProjectDirs))
	}
	if len(cfg.Dev.BuildPatterns) != 3 {
		t.Errorf("expected 3 build patterns, got %d", len(cfg.Dev.BuildPatterns))
	}
}

// =============================================================================
// Secure Deletion Config Tests
// =============================================================================

func TestLoadSecureDeletionConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
secure_deletion:
  enabled: true
  standard: "dod522022"
  custom_passes: 7
  verify_writes: true
  force_sync: true
  buffer_size_kb: 4096
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !cfg.SecureDeletion.Enabled {
		t.Error("expected SecureDeletion.Enabled to be true")
	}
	if cfg.SecureDeletion.Standard != "dod522022" {
		t.Errorf("expected standard 'dod522022', got %q", cfg.SecureDeletion.Standard)
	}
	if cfg.SecureDeletion.CustomPasses != 7 {
		t.Errorf("expected 7 custom passes, got %d", cfg.SecureDeletion.CustomPasses)
	}
	if !cfg.SecureDeletion.VerifyWrites {
		t.Error("expected VerifyWrites to be true")
	}
	if cfg.SecureDeletion.BufferSizeKB != 4096 {
		t.Errorf("expected buffer size 4096, got %d", cfg.SecureDeletion.BufferSizeKB)
	}
}

// =============================================================================
// Large Files Config Tests
// =============================================================================

func TestLoadLargeFilesConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
large_files_config:
  min_size: "500MB"
  scan_paths:
    - "~/Downloads"
    - "~/Desktop"
  exclude_paths:
    - "~/Downloads/Important"
  file_types:
    - ".iso"
    - ".dmg"
    - ".zip"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.LargeFiles.MinSize != "500MB" {
		t.Errorf("expected min size '500MB', got %q", cfg.LargeFiles.MinSize)
	}
	if len(cfg.LargeFiles.ScanPaths) != 2 {
		t.Errorf("expected 2 scan paths, got %d", len(cfg.LargeFiles.ScanPaths))
	}
	if len(cfg.LargeFiles.FileTypes) != 3 {
		t.Errorf("expected 3 file types, got %d", len(cfg.LargeFiles.FileTypes))
	}
}

// =============================================================================
// Old Files Config Tests
// =============================================================================

func TestLoadOldFilesConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
old_files_config:
  min_age_days: 180
  scan_paths:
    - "~/Documents"
  exclude_paths:
    - "~/Documents/Archive"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.OldFiles.MinAgeDays != 180 {
		t.Errorf("expected min age 180 days, got %d", cfg.OldFiles.MinAgeDays)
	}
	if len(cfg.OldFiles.ScanPaths) != 1 {
		t.Errorf("expected 1 scan path, got %d", len(cfg.OldFiles.ScanPaths))
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestLoadEmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Empty file
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed for empty config: %v", err)
	}

	// Should still have defaults
	if !cfg.Categories.Cache {
		t.Error("expected default Cache to be enabled")
	}
}

func TestLoadConfigWithComments(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
# This is a comment
categories:
  cache: true  # inline comment
  temp: false
# Another comment
min_file_age: 48
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed for config with comments: %v", err)
	}

	if !cfg.Categories.Cache {
		t.Error("expected Cache to be true")
	}
	if cfg.Categories.Temp {
		t.Error("expected Temp to be false")
	}
	if cfg.MinFileAge != 48 {
		t.Errorf("expected MinFileAge 48, got %d", cfg.MinFileAge)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create a custom config
	original := GetDefault()
	original.Categories.Downloads = true
	original.Categories.Docker = true
	original.AgeThresholds.Logs = 60
	original.MinFileAge = 72
	original.DryRun = true
	original.Verbose = true
	original.WhitelistPaths = []string{"/home/user/important"}
	original.ProtectedPaths = []string{"/var/critical"}

	// Save it
	if err := Save(original, configPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load it back
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Compare
	if loaded.Categories.Downloads != original.Categories.Downloads {
		t.Error("Downloads mismatch after round-trip")
	}
	if loaded.Categories.Docker != original.Categories.Docker {
		t.Error("Docker mismatch after round-trip")
	}
	if loaded.AgeThresholds.Logs != original.AgeThresholds.Logs {
		t.Error("Logs threshold mismatch after round-trip")
	}
	if loaded.MinFileAge != original.MinFileAge {
		t.Error("MinFileAge mismatch after round-trip")
	}
	if loaded.DryRun != original.DryRun {
		t.Error("DryRun mismatch after round-trip")
	}
	if loaded.Verbose != original.Verbose {
		t.Error("Verbose mismatch after round-trip")
	}
	if len(loaded.WhitelistPaths) != len(original.WhitelistPaths) {
		t.Error("WhitelistPaths length mismatch after round-trip")
	}
}
