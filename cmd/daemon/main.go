package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/fenilsonani/cleanup-cache/internal/config"
	"github.com/fenilsonani/cleanup-cache/internal/daemon"
)

var (
	Version   = "0.3.0"
	GitCommit = "unknown"
	BuildTime = "unknown"

	configPath  string
	foreground  bool
	testConfig  bool
	showVersion bool
)

func init() {
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.BoolVar(&foreground, "foreground", false, "Run in foreground (don't daemonize)")
	flag.BoolVar(&testConfig, "test-config", false, "Test configuration and exit")
	flag.BoolVar(&showVersion, "version", false, "Show version and exit")
}

func main() {
	flag.Parse()

	if showVersion {
		fmt.Printf("CleanupCache Daemon v%s (commit: %s, built: %s)\n", Version, GitCommit, BuildTime)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Check if daemon is enabled
	if cfg.Daemon == nil || !cfg.Daemon.Enabled {
		fmt.Fprintf(os.Stderr, "Daemon not enabled in configuration\n")
		fmt.Fprintf(os.Stderr, "Add the following to your config file:\n")
		fmt.Fprintf(os.Stderr, "daemon:\n")
		fmt.Fprintf(os.Stderr, "  enabled: true\n")
		fmt.Fprintf(os.Stderr, "  schedules:\n")
		fmt.Fprintf(os.Stderr, "    - name: daily\n")
		fmt.Fprintf(os.Stderr, "      schedule: \"0 2 * * *\"\n")
		os.Exit(1)
	}

	// Validate schedules
	if len(cfg.Daemon.Schedules) == 0 {
		fmt.Fprintf(os.Stderr, "No schedules configured. Add at least one schedule.\n")
		os.Exit(1)
	}

	// Test configuration if requested
	if testConfig {
		fmt.Println("Configuration is valid")
		fmt.Printf("Daemon enabled: %v\n", cfg.Daemon.Enabled)
		fmt.Printf("Schedules: %d\n", len(cfg.Daemon.Schedules))
		for _, sched := range cfg.Daemon.Schedules {
			fmt.Printf("  - %s: %s\n", sched.Name, sched.Schedule)
		}
		os.Exit(0)
	}

	// Check if already running
	if isRunning(cfg) {
		fmt.Fprintf(os.Stderr, "Daemon is already running\n")
		os.Exit(1)
	}

	// Create and start daemon
	d, err := daemon.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating daemon: %v\n", err)
		os.Exit(1)
	}

	// Start daemon
	fmt.Println("Starting CleanupCache Daemon...")
	if err := d.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting daemon: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig() (*config.Config, error) {
	if configPath != "" {
		return config.Load(configPath)
	}

	// Try system config first
	if _, err := os.Stat("/etc/cleanup-cache/config.yaml"); err == nil {
		return config.Load("/etc/cleanup-cache/config.yaml")
	}

	// Fall back to user config
	cfgPath, err := config.GetConfigPath()
	if err != nil {
		return nil, err
	}

	return config.Load(cfgPath)
}

func isRunning(cfg *config.Config) bool {
	pidFile := cfg.Daemon.PidFile
	if pidFile == "" {
		pidFile = "/var/run/cleanup-cache.pid"
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}

	var pid int
	_, err = fmt.Sscanf(string(data), "%d", &pid)
	if err != nil {
		return false
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Try to signal process
	err = process.Signal(nil)
	return err == nil
}
