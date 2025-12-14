package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/fenilsonani/cleanup-cache/internal/cleaner"
	"github.com/fenilsonani/cleanup-cache/internal/config"
	"github.com/fenilsonani/cleanup-cache/internal/platform"
	"github.com/fenilsonani/cleanup-cache/internal/scanner"
)

// Daemon represents the cleanup daemon
type Daemon struct {
	config       *config.Config
	scheduler    *Scheduler
	notifier     *Notifier
	logger       *Logger
	running      bool
	shutdownCtx  context.Context
	cancelFunc   context.CancelFunc
	mu           sync.RWMutex
}

// New creates a new daemon instance
func New(cfg *config.Config) (*Daemon, error) {
	if cfg.Daemon == nil || !cfg.Daemon.Enabled {
		return nil, fmt.Errorf("daemon not enabled in configuration")
	}

	// Create context for shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize logger
	logger, err := NewLogger(cfg.Daemon.LogFile, cfg.Daemon.LogLevel)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	daemon := &Daemon{
		config:      cfg,
		logger:      logger,
		running:     false,
		shutdownCtx: ctx,
		cancelFunc:  cancel,
	}

	// Initialize scheduler
	daemon.scheduler = NewScheduler(daemon, cfg.Daemon.Schedules)

	// Initialize notifier if enabled
	if cfg.Daemon.Notifications.Enabled {
		daemon.notifier = NewNotifier(&cfg.Daemon.Notifications, logger)
	}

	return daemon, nil
}

// Start starts the daemon
func (d *Daemon) Start() error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return fmt.Errorf("daemon already running")
	}
	d.running = true
	d.mu.Unlock()

	d.logger.Info("Starting cleanup daemon")

	// Check lock file
	if err := d.acquireLock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer d.releaseLock()

	// Write PID file
	if err := d.writePidFile(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer d.removePidFile()

	// Setup signal handlers
	d.setupSignalHandlers()

	// Start scheduler
	if err := d.scheduler.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}
	defer d.scheduler.Stop()

	d.logger.Info("Daemon started successfully")

	// Send startup notification
	if d.notifier != nil {
		d.notifier.SendStartupNotification()
	}

	// Wait for shutdown signal
	<-d.shutdownCtx.Done()

	d.logger.Info("Daemon shutting down")

	d.mu.Lock()
	d.running = false
	d.mu.Unlock()

	// Send shutdown notification
	if d.notifier != nil {
		d.notifier.SendShutdownNotification()
	}

	return nil
}

// Stop stops the daemon
func (d *Daemon) Stop() {
	if d.cancelFunc != nil {
		d.cancelFunc()
	}
}

// IsRunning returns whether the daemon is running
func (d *Daemon) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// RunCleanupJob executes a cleanup job
func (d *Daemon) RunCleanupJob(job *CleanupJob) error {
	d.logger.Info("Running cleanup job: %s", job.Name)
	startTime := time.Now()

	// Get platform info
	platformInfo, err := platform.GetInfo()
	if err != nil {
		return fmt.Errorf("failed to get platform info: %w", err)
	}

	// Create job-specific config
	jobConfig := d.createJobConfig(job)

	// Create scanner
	scnr := scanner.NewOptimized(jobConfig, platformInfo)

	// Perform scan
	scanResult, err := scnr.ScanAll()
	if err != nil {
		d.logger.Error("Scan failed for job %s: %v", job.Name, err)
		return fmt.Errorf("scan failed: %w", err)
	}

	d.logger.Info("Scan completed for job %s: %d files, %d bytes",
		job.Name, scanResult.TotalCount, scanResult.TotalSize)

	// Skip cleanup if dry-run
	if jobConfig.DryRun {
		d.logger.Info("Dry-run mode - skipping cleanup for job %s", job.Name)
		return nil
	}

	// Create cleaner
	clnr := cleaner.New(jobConfig)

	// Perform cleanup
	cleanResult, err := clnr.Clean(scanResult)
	if err != nil {
		d.logger.Error("Cleanup failed for job %s: %v", job.Name, err)
		return fmt.Errorf("cleanup failed: %w", err)
	}

	// Log results
	duration := time.Since(startTime)
	d.logger.Info("Cleanup job %s completed in %v: deleted %d files, freed %d bytes, %d errors",
		job.Name, duration, len(cleanResult.DeletedFiles), cleanResult.DeletedSize, len(cleanResult.Errors))

	// Send notification
	if d.notifier != nil {
		d.notifier.SendCleanupNotification(job, cleanResult, duration)
	}

	return nil
}

// createJobConfig creates a config for a specific job
func (d *Daemon) createJobConfig(job *CleanupJob) *config.Config {
	// Copy base config
	cfg := *d.config

	// Override categories based on job
	if job.Categories != nil {
		cfg.Categories.Cache = job.Categories["cache"]
		cfg.Categories.Temp = job.Categories["temp"]
		cfg.Categories.Logs = job.Categories["logs"]
		cfg.Categories.Downloads = job.Categories["downloads"]
		cfg.Categories.PackageManagers = job.Categories["package_managers"]
		cfg.Categories.Docker = job.Categories["docker"]
	}

	// Override dry-run
	if job.DryRun {
		cfg.DryRun = true
	}

	return &cfg
}

// setupSignalHandlers sets up signal handlers for graceful shutdown
func (d *Daemon) setupSignalHandlers() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for sig := range sigChan {
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				d.logger.Info("Received shutdown signal: %v", sig)
				d.Stop()
			case syscall.SIGHUP:
				d.logger.Info("Received reload signal")
				// TODO: Implement config reload
			}
		}
	}()
}

// acquireLock acquires the lock file
func (d *Daemon) acquireLock() error {
	lockFile := d.config.Daemon.PidFile + ".lock"
	if lockFile == ".lock" {
		lockFile = "/var/run/cleanup-cache.lock"
	}

	file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("daemon already running (lock file exists)")
		}
		return err
	}

	pid := os.Getpid()
	_, err = fmt.Fprintf(file, "%d\n", pid)
	file.Close()
	return err
}

// releaseLock releases the lock file
func (d *Daemon) releaseLock() error {
	lockFile := d.config.Daemon.PidFile + ".lock"
	if lockFile == ".lock" {
		lockFile = "/var/run/cleanup-cache.lock"
	}
	return os.Remove(lockFile)
}

// writePidFile writes the PID file
func (d *Daemon) writePidFile() error {
	pidFile := d.config.Daemon.PidFile
	if pidFile == "" {
		pidFile = "/var/run/cleanup-cache.pid"
	}

	pid := os.Getpid()
	return os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", pid)), 0644)
}

// removePidFile removes the PID file
func (d *Daemon) removePidFile() error {
	pidFile := d.config.Daemon.PidFile
	if pidFile == "" {
		pidFile = "/var/run/cleanup-cache.pid"
	}
	return os.Remove(pidFile)
}

// Logger provides logging for the daemon
type Logger struct {
	logger   *log.Logger
	logLevel string
	file     *os.File
}

// NewLogger creates a new logger
func NewLogger(logFile, logLevel string) (*Logger, error) {
	var file *os.File
	var err error

	if logFile != "" {
		file, err = os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
	}

	var logger *log.Logger
	if file != nil {
		logger = log.New(file, "", log.LstdFlags)
	} else {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	}

	return &Logger{
		logger:   logger,
		logLevel: logLevel,
		file:     file,
	}, nil
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.logger.Printf("[INFO] "+format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.logger.Printf("[ERROR] "+format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.logger.Printf("[WARN] "+format, args...)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.logLevel == "debug" {
		l.logger.Printf("[DEBUG] "+format, args...)
	}
}

// Close closes the logger
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
