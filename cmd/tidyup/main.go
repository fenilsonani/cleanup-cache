package main

import (
	"fmt"
	"os"

	"github.com/fenilsonani/system-cleanup/internal/cleaner"
	"github.com/fenilsonani/system-cleanup/internal/config"
	"github.com/fenilsonani/system-cleanup/internal/platform"
	"github.com/fenilsonani/system-cleanup/internal/reporter"
	"github.com/fenilsonani/system-cleanup/internal/scanner"
	"github.com/fenilsonani/system-cleanup/internal/ui"
	"github.com/spf13/cobra"
)

var (
	Version   = "0.4.0"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

var (
	configPath  string
	verbose     bool
	dryRun      bool
	force       bool
	category    string
	outputFmt   string
	outputFile  string
	minSize     string
	minAgeDays  int
	cleanAction bool
	detailed    bool
	showLive    bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "tidyup",
	Short: "System cleanup and disk space recovery tool",
	Long: `TidyUp is a powerful system cleanup tool that helps you reclaim disk space by removing:
  - Development artifacts (node_modules, venv, build folders)
  - Large forgotten files (movies, archives, disk images)
  - Old unused files in Downloads and Documents
  - System caches, logs, and temporary files`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildTime),
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan the system for cleanable files",
	Long: `Scans the system and reports what can be cleaned without making any changes.

Use --detailed (-d) to see a tree view of all files found.
Use --live (-l) to see real-time scanning progress.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Get platform info
		platformInfo, err := platform.GetInfo()
		if err != nil {
			return fmt.Errorf("failed to get platform info: %w", err)
		}

		// Use HyperScanner - blazingly fast with caching & Spotlight
		fmt.Println("⚡ Scanning...")
		hyperScnr := scanner.NewHyperScanner(cfg, platformInfo)

		// Setup live progress if enabled
		var liveProgress *ui.LiveProgress
		if showLive {
			liveProgress = ui.NewLiveProgress()
			liveProgress.Start()
			hyperScnr.SetProgressCallback(func(cat, path string, filesFound int, totalSize int64) {
				liveProgress.Update(cat, path, filesFound, totalSize)
			})
		}

		result, err := hyperScnr.ScanAll()

		if liveProgress != nil {
			liveProgress.Finish()
		}

		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		// Show detailed tree view if requested
		if detailed {
			files := make([]ui.FileInfo, len(result.Files))
			for i, f := range result.Files {
				files[i] = ui.FileInfo{
					Path:     f.Path,
					Size:     f.Size,
					Category: f.Category,
					Reason:   f.Reason,
				}
			}
			ui.PrintDetailedTree(files, result.TotalSize)
			return nil
		}

		// Create reporter for summary view
		rptr := reporter.New(os.Stdout, reporter.FormatSummary)

		// Print report
		if err := rptr.Report(result); err != nil {
			return fmt.Errorf("failed to generate report: %w", err)
		}

		return nil
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean the system based on configuration",
	Long:  `Cleans the system by removing files identified during scanning.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Override config with flags
		if cmd.Flags().Changed("dry-run") {
			cfg.DryRun = dryRun
		}

		// Get platform info
		platformInfo, err := platform.GetInfo()
		if err != nil {
			return fmt.Errorf("failed to get platform info: %w", err)
		}

		// Use HyperScanner - blazingly fast with caching & Spotlight
		hyperScnr := scanner.NewHyperScanner(cfg, platformInfo)

		// Setup live progress if enabled
		var liveProgress *ui.LiveProgress
		if showLive {
			liveProgress = ui.NewLiveProgress()
			liveProgress.Start()
			hyperScnr.SetProgressCallback(func(cat, path string, filesFound int, totalSize int64) {
				liveProgress.Update(cat, path, filesFound, totalSize)
			})
		}

		var scanResult *scanner.ScanResult

		if category != "" {
			fmt.Printf("⚡ Scanning category: %s...\n", category)
			scanResult = hyperScnr.ScanCategory(category)
		} else {
			fmt.Println("⚡ Scanning...")
			var scanErr error
			scanResult, scanErr = hyperScnr.ScanAll()
			if scanErr != nil {
				if liveProgress != nil {
					liveProgress.Finish()
				}
				return fmt.Errorf("scan failed: %w", scanErr)
			}
		}

		if liveProgress != nil {
			liveProgress.Finish()
		}

		// Check if any files found
		if scanResult.TotalCount == 0 {
			fmt.Println("\n✨ No files found for cleanup. Your system is already clean!")
			return nil
		}

		// Show summary
		rptr := reporter.New(os.Stdout, reporter.FormatSummary)
		if err := rptr.Report(scanResult); err != nil {
			return fmt.Errorf("failed to generate report: %w", err)
		}

		// Confirm if not force mode
		if !force && !cfg.DryRun {
			fmt.Print("\nProceed with cleanup? (y/N): ")
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println("Cleanup cancelled")
				return nil
			}
		}

		// Create cleaner
		clnr := cleaner.New(cfg)

		// Don't prompt for sudo if --force is used
		if force {
			clnr.SetAskSudo(false)
		}

		if cfg.DryRun {
			fmt.Println("\n[DRY RUN MODE] No files will be deleted.")

			// Show permission analysis
			permReport := clnr.GetPermissionReport(scanResult)
			if len(permReport.RequiresSudo) > 0 {
				fmt.Printf("\n📋 Permission Analysis:\n")
				fmt.Printf("   ✅ Normal files: %d (%s)\n",
					len(permReport.NormalFiles),
					formatBytes(permReport.TotalNormalSize))
				fmt.Printf("   🔐 Requires sudo: %d (%s)\n",
					len(permReport.RequiresSudo),
					formatBytes(permReport.TotalSudoSize))
				if len(permReport.InaccessibleFiles) > 0 {
					fmt.Printf("   ⚠️  Inaccessible: %d files\n", len(permReport.InaccessibleFiles))
				}
			}
		} else {
			fmt.Println("\nCleaning...")
		}

		// Clean
		cleanResult, err := clnr.Clean(scanResult)
		if err != nil {
			return fmt.Errorf("clean failed: %w", err)
		}

		// Show results
		fmt.Printf("\n📊 Cleanup Complete!\n")
		fmt.Printf("✅ Successfully deleted: %d files (%s)\n",
			len(cleanResult.DeletedFiles),
			formatBytes(cleanResult.DeletedSize))

		if cleanResult.UsedSudo {
			fmt.Printf("🔐 Used elevated permissions: %d succeeded, %d failed\n",
				cleanResult.SudoSucceeded,
				cleanResult.SudoFailed)
		}

		if len(cleanResult.SkippedFiles) > 0 {
			fmt.Printf("\n⚠️  Skipped: %d files\n", len(cleanResult.SkippedFiles))
		}

		if len(cleanResult.Errors) > 0 {
			fmt.Printf("\n%s", cleaner.FormatErrorSummary(cleanResult.Errors))
		}

		return nil
	},
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate a detailed report",
	Long:  `Generates a detailed report of system cleanup opportunities.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Get platform info
		platformInfo, err := platform.GetInfo()
		if err != nil {
			return fmt.Errorf("failed to get platform info: %w", err)
		}

		// Use HyperScanner
		fmt.Println("⚡ Scanning...")
		hyperScnr := scanner.NewHyperScanner(cfg, platformInfo)

		result, err := hyperScnr.ScanAll()
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		// Parse format
		var format reporter.OutputFormat
		switch outputFmt {
		case "json":
			format = reporter.FormatJSON
		case "yaml":
			format = reporter.FormatYAML
		case "table":
			format = reporter.FormatTable
		default:
			format = reporter.FormatSummary
		}

		// Generate report
		if outputFile != "" {
			if err := reporter.SaveToFile(result, outputFile, format); err != nil {
				return fmt.Errorf("failed to save report: %w", err)
			}
			fmt.Printf("Report saved to: %s\n", outputFile)
		} else {
			rptr := reporter.New(os.Stdout, format)
			if err := rptr.Report(result); err != nil {
				return fmt.Errorf("failed to generate report: %w", err)
			}
		}

		return nil
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Display current configuration",
	Long:  `Shows the current configuration being used.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, err := config.GetConfigPath()
		if err != nil {
			return err
		}

		fmt.Printf("Config file: %s\n", cfgPath)

		// Check if config exists
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			fmt.Println("Config file does not exist. Using default configuration.")
			fmt.Println("\nTo create a config file:")
			fmt.Printf("  mkdir -p %s\n", cfgPath[:len(cfgPath)-len("/config.yaml")])
			fmt.Printf("  cp configs/cleanup.example.yaml %s\n", cfgPath)
		}

		return nil
	},
}

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Scan for development artifacts",
	Long: `Scans for development artifacts like node_modules, virtual environments,
and build directories (.next, dist, target, __pycache__, etc.)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Disable all categories except dev ones
		cfg.Categories.Cache = false
		cfg.Categories.Temp = false
		cfg.Categories.Logs = false
		cfg.Categories.Downloads = false
		cfg.Categories.PackageManagers = false
		cfg.Categories.Docker = false
		cfg.Categories.LargeFiles = false
		cfg.Categories.OldFiles = false
		// Enable dev categories
		cfg.Categories.NodeModules = true
		cfg.Categories.VirtualEnvs = true
		cfg.Categories.BuildArtifacts = true

		// Override config with flags
		if cmd.Flags().Changed("dry-run") {
			cfg.DryRun = dryRun
		}

		platformInfo, err := platform.GetInfo()
		if err != nil {
			return fmt.Errorf("failed to get platform info: %w", err)
		}

		fmt.Println("⚡ Scanning for development artifacts...")
		hyperScnr := scanner.NewHyperScanner(cfg, platformInfo)

		result, err := hyperScnr.ScanAll()
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		if result.TotalCount == 0 {
			fmt.Println("\nNo development artifacts found in configured project directories.")
			fmt.Println("Configure project directories in your config file under 'dev.project_dirs'")
			return nil
		}

		// Show results grouped by category
		fmt.Println("\n=== Development Artifacts ===")
		grouped := result.GroupByCategory()

		for _, cat := range []string{"node_modules", "virtual_envs", "build_artifacts"} {
			if catResult, ok := grouped[cat]; ok && catResult.TotalCount > 0 {
				fmt.Printf("  %s: %d items, %s\n", cat, catResult.TotalCount, formatBytes(catResult.TotalSize))
			}
		}

		fmt.Printf("\nTotal reclaimable: %s\n", formatBytes(result.TotalSize))

		// If --clean flag is set, proceed with cleanup
		if cleanAction {
			return cleanDevArtifacts(cfg, result)
		}

		fmt.Println("\nRun 'tidyup dev --clean' to remove these artifacts")
		return nil
	},
}

var largeCmd = &cobra.Command{
	Use:   "large",
	Short: "Find large files",
	Long:  `Finds large files across your home directory. Default minimum size is 500MB.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Disable all categories except large files
		cfg.Categories.Cache = false
		cfg.Categories.Temp = false
		cfg.Categories.Logs = false
		cfg.Categories.Downloads = false
		cfg.Categories.PackageManagers = false
		cfg.Categories.Docker = false
		cfg.Categories.NodeModules = false
		cfg.Categories.VirtualEnvs = false
		cfg.Categories.BuildArtifacts = false
		cfg.Categories.OldFiles = false
		// Enable large files
		cfg.Categories.LargeFiles = true

		// Override config with flags
		if cmd.Flags().Changed("min") {
			cfg.LargeFiles.MinSize = minSize
		}
		if cmd.Flags().Changed("dry-run") {
			cfg.DryRun = dryRun
		}

		platformInfo, err := platform.GetInfo()
		if err != nil {
			return fmt.Errorf("failed to get platform info: %w", err)
		}

		fmt.Printf("⚡ Scanning for files larger than %s...\n", cfg.LargeFiles.MinSize)
		hyperScnr := scanner.NewHyperScanner(cfg, platformInfo)

		result, err := hyperScnr.ScanAll()
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		if result.TotalCount == 0 {
			fmt.Printf("\nNo files larger than %s found.\n", cfg.LargeFiles.MinSize)
			return nil
		}

		// Show results
		fmt.Println("\n=== Large Files ===")
		for _, file := range result.Files {
			fmt.Printf("  %s - %s\n", formatBytes(file.Size), file.Path)
		}

		fmt.Printf("\nTotal: %d files, %s\n", result.TotalCount, formatBytes(result.TotalSize))

		// If --clean flag is set, proceed with cleanup
		if cleanAction {
			return cleanFiles(cfg, result, "large files")
		}

		fmt.Println("\nRun 'tidyup large --clean' to remove these files")
		return nil
	},
}

var oldCmd = &cobra.Command{
	Use:   "old",
	Short: "Find old/unused files",
	Long: `Finds files that haven't been accessed in a long time.
Scans Downloads, Documents, and Desktop by default.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Disable all categories except old files
		cfg.Categories.Cache = false
		cfg.Categories.Temp = false
		cfg.Categories.Logs = false
		cfg.Categories.Downloads = false
		cfg.Categories.PackageManagers = false
		cfg.Categories.Docker = false
		cfg.Categories.NodeModules = false
		cfg.Categories.VirtualEnvs = false
		cfg.Categories.BuildArtifacts = false
		cfg.Categories.LargeFiles = false
		// Enable old files
		cfg.Categories.OldFiles = true

		// Override config with flags
		if cmd.Flags().Changed("days") {
			cfg.OldFiles.MinAgeDays = minAgeDays
		}
		if cmd.Flags().Changed("dry-run") {
			cfg.DryRun = dryRun
		}

		platformInfo, err := platform.GetInfo()
		if err != nil {
			return fmt.Errorf("failed to get platform info: %w", err)
		}

		fmt.Printf("⚡ Scanning for files not accessed in %d days...\n", cfg.OldFiles.MinAgeDays)
		hyperScnr := scanner.NewHyperScanner(cfg, platformInfo)

		result, err := hyperScnr.ScanAll()
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		if result.TotalCount == 0 {
			fmt.Printf("\nNo files found that haven't been accessed in %d days.\n", cfg.OldFiles.MinAgeDays)
			return nil
		}

		// Show results
		fmt.Println("\n=== Old/Unused Files ===")
		for _, file := range result.Files {
			fmt.Printf("  %s - %s\n    %s\n", formatBytes(file.Size), file.Path, file.Reason)
		}

		fmt.Printf("\nTotal: %d files, %s\n", result.TotalCount, formatBytes(result.TotalSize))

		// If --clean flag is set, proceed with cleanup
		if cleanAction {
			return cleanFiles(cfg, result, "old files")
		}

		fmt.Println("\nRun 'tidyup old --clean' to remove these files")
		return nil
	},
}

// cleanDevArtifacts handles cleanup of development artifacts
func cleanDevArtifacts(cfg *config.Config, scanResult *scanner.ScanResult) error {
	return cleanFiles(cfg, scanResult, "development artifacts")
}

// cleanFiles is a generic function to clean files from any category
func cleanFiles(cfg *config.Config, scanResult *scanner.ScanResult, description string) error {
	if !force && !cfg.DryRun {
		fmt.Print("\nProceed with cleanup? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cleanup cancelled")
			return nil
		}
	}

	clnr := cleaner.New(cfg)

	if cfg.DryRun {
		fmt.Println("\n[DRY RUN MODE] No files will be deleted.")
	} else {
		fmt.Printf("\nCleaning %s...\n", description)
	}

	cleanResult, err := clnr.Clean(scanResult)
	if err != nil {
		return fmt.Errorf("clean failed: %w", err)
	}

	fmt.Printf("\nCleanup Complete!\n")
	fmt.Printf("Successfully removed: %d items (%s)\n",
		len(cleanResult.DeletedFiles),
		formatBytes(cleanResult.DeletedSize))

	if len(cleanResult.Errors) > 0 {
		fmt.Printf("\n%s", cleaner.FormatErrorSummary(cleanResult.Errors))
	}

	return nil
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file path")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose output")

	// Scan command flags
	scanCmd.Flags().StringVar(&outputFmt, "output", "summary", "output format (summary, table, json, yaml)")
	scanCmd.Flags().BoolVarP(&detailed, "detailed", "d", false, "show detailed tree view of all files")
	scanCmd.Flags().BoolVarP(&showLive, "live", "l", false, "show live scanning progress")

	// Clean command flags
	cleanCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be deleted without actually deleting")
	cleanCmd.Flags().BoolVar(&force, "force", false, "skip confirmation prompts")
	cleanCmd.Flags().StringVar(&category, "category", "", "clean only specific category (uses turbo scanner)")
	cleanCmd.Flags().BoolVarP(&showLive, "live", "l", false, "show live scanning progress")

	// Report command flags
	reportCmd.Flags().StringVar(&outputFmt, "output", "summary", "output format (summary, table, json, yaml)")
	reportCmd.Flags().StringVar(&outputFile, "file", "", "save report to file")

	// Dev command flags
	devCmd.Flags().BoolVar(&cleanAction, "clean", false, "clean the found artifacts")
	devCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be deleted without actually deleting")
	devCmd.Flags().BoolVar(&force, "force", false, "skip confirmation prompts")

	// Large command flags
	largeCmd.Flags().StringVar(&minSize, "min", "500MB", "minimum file size (e.g., 500MB, 1GB)")
	largeCmd.Flags().BoolVar(&cleanAction, "clean", false, "clean the found files")
	largeCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be deleted without actually deleting")
	largeCmd.Flags().BoolVar(&force, "force", false, "skip confirmation prompts")

	// Old command flags
	oldCmd.Flags().IntVar(&minAgeDays, "days", 180, "minimum age in days (default 180)")
	oldCmd.Flags().BoolVar(&cleanAction, "clean", false, "clean the found files")
	oldCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be deleted without actually deleting")
	oldCmd.Flags().BoolVar(&force, "force", false, "skip confirmation prompts")

	// Add commands
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(largeCmd)
	rootCmd.AddCommand(oldCmd)
}

func loadConfig() (*config.Config, error) {
	if configPath != "" {
		return config.Load(configPath)
	}

	cfgPath, err := config.GetConfigPath()
	if err != nil {
		return nil, err
	}

	return config.Load(cfgPath)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGT"[exp])
}
