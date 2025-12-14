package main

import (
	"fmt"
	"os"

	"github.com/fenilsonani/cleanup-cache/internal/cleaner"
	"github.com/fenilsonani/cleanup-cache/internal/config"
	"github.com/fenilsonani/cleanup-cache/internal/platform"
	"github.com/fenilsonani/cleanup-cache/internal/reporter"
	"github.com/fenilsonani/cleanup-cache/internal/scanner"
	"github.com/spf13/cobra"
)

var (
	Version   = "0.3.0"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

var (
	configPath string
	verbose    bool
	dryRun     bool
	force      bool
	category   string
	outputFmt  string
	outputFile string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Multi-platform system cleaner",
	Long: `CleanupCache is a powerful, safe, and intelligent CLI tool to clean up your
Mac or Linux system by removing unnecessary files, caches, logs, and duplicates.`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildTime),
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan the system for cleanable files",
	Long:  `Scans the system and reports what can be cleaned without making any changes.`,
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

		// Create scanner
		scnr := scanner.New(cfg, platformInfo)

		fmt.Println("Scanning system...")

		// Scan all categories
		result, err := scnr.ScanAll()
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		// Create reporter
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

		// Create scanner
		scnr := scanner.New(cfg, platformInfo)

		fmt.Println("Scanning system...")

		// Scan
		var scanResult *scanner.ScanResult
		if category != "" {
			// Scan all first, then filter
			allResults, err := scnr.ScanAll()
			if err != nil {
				return fmt.Errorf("scan failed: %w", err)
			}

			// Filter by category
			scanResult = &scanner.ScanResult{
				Files:  []scanner.FileInfo{},
				Errors: []error{},
			}
			for _, file := range allResults.Files {
				if file.Category == category {
					scanResult.Files = append(scanResult.Files, file)
					scanResult.TotalSize += file.Size
					scanResult.TotalCount++
				}
			}
		} else {
			var err error
			scanResult, err = scnr.ScanAll()
			if err != nil {
				return fmt.Errorf("scan failed: %w", err)
			}
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

		// Create scanner
		scnr := scanner.New(cfg, platformInfo)

		fmt.Println("Scanning system...")

		// Scan
		result, err := scnr.ScanAll()
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


func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file path")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose output")

	// Scan command flags
	scanCmd.Flags().StringVar(&outputFmt, "output", "summary", "output format (summary, table, json, yaml)")

	// Clean command flags
	cleanCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be deleted without actually deleting")
	cleanCmd.Flags().BoolVar(&force, "force", false, "skip confirmation prompts")
	cleanCmd.Flags().StringVar(&category, "category", "", "clean only specific category")

	// Report command flags
	reportCmd.Flags().StringVar(&outputFmt, "output", "summary", "output format (summary, table, json, yaml)")
	reportCmd.Flags().StringVar(&outputFile, "file", "", "save report to file")

	// Add commands
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(configCmd)
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
