package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fenilsonani/system-cleanup/internal/config"
)

// AppData holds information about an installed app and its related data
type AppData struct {
	Name       string
	AppPath    string          // Path to the .app bundle
	DataPaths  []string        // Paths to app data, caches, settings
	TotalSize  int64
	IsInstalled bool
}

// AppManager handles app discovery and uninstallation
type AppManager struct {
	cfg    *config.Config
	home   string
	apps   []AppData
	loaded bool
}

// NewAppManager creates a new app manager
func NewAppManager(cfg *config.Config) *AppManager {
	home, _ := os.UserHomeDir()
	return &AppManager{
		cfg:    cfg,
		home:   home,
		apps:   []AppData{},
		loaded: false,
	}
}

// LoadApps discovers all installed apps and their data
func (am *AppManager) LoadApps() error {
	if am.loaded {
		return nil
	}

	// Find installed apps in /Applications
	appDir := "/Applications"
	entries, err := os.ReadDir(appDir)
	if err != nil {
		return fmt.Errorf("failed to read Applications: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Skip system apps and unneeded folders
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "Utilities") {
			continue
		}

		appPath := filepath.Join(appDir, name)
		appName := strings.TrimSuffix(name, ".app")

		// Find all related data for this app
		dataPaths := am.findAppData(appName)

		// Calculate total size
		totalSize := int64(0)
		for _, dataPath := range dataPaths {
			totalSize += am.getDirSize(dataPath)
		}

		// Only include apps that have some data (saves time)
		if totalSize > 0 || len(dataPaths) > 0 {
			am.apps = append(am.apps, AppData{
				Name:        appName,
				AppPath:     appPath,
				DataPaths:   dataPaths,
				TotalSize:   totalSize,
				IsInstalled: true,
			})
		}
	}

	// Sort by total size descending
	sort.Slice(am.apps, func(i, j int) bool {
		return am.apps[i].TotalSize > am.apps[j].TotalSize
	})

	am.loaded = true
	return nil
}

// findAppData finds all data locations for an app by name pattern
func (am *AppManager) findAppData(appName string) []string {
	var paths []string

	searchDirs := []string{
		filepath.Join(am.home, "Library/Application Support"),
		filepath.Join(am.home, "Library/Caches"),
		filepath.Join(am.home, "Library/Preferences"),
		filepath.Join(am.home, "Library/Saved Application State"),
		filepath.Join(am.home, "Library/HTTPStorages"),
		filepath.Join(am.home, "Library/Cookies"),
	}

	for _, searchDir := range searchDirs {
		if _, err := os.Stat(searchDir); err != nil {
			continue
		}

		entries, err := os.ReadDir(searchDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()

			// Match by app name (case-insensitive, partial match)
			if am.matchesApp(name, appName) {
				fullPath := filepath.Join(searchDir, name)
				paths = append(paths, fullPath)
			}
		}
	}

	return paths
}

// matchesApp checks if a path matches an app name
func (am *AppManager) matchesApp(pathName, appName string) bool {
	// Exact match
	if strings.EqualFold(pathName, appName) {
		return true
	}

	// Case-insensitive prefix match
	if strings.HasPrefix(strings.ToLower(pathName), strings.ToLower(appName)) {
		return true
	}

	// Reverse - check if appName is in pathName
	if strings.Contains(strings.ToLower(pathName), strings.ToLower(appName)) {
		return true
	}

	return false
}

// getDirSize calculates directory size recursively
func (am *AppManager) getDirSize(path string) int64 {
	var size int64
	filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return size
}

// ListApps prints all discovered apps
func (am *AppManager) ListApps() error {
	if err := am.LoadApps(); err != nil {
		return err
	}

	if len(am.apps) == 0 {
		fmt.Println("No apps with associated data found")
		return nil
	}

	fmt.Println("\nInstalled Apps with Data\n")
	fmt.Println("-----+----------------------+----------+------------------")
	fmt.Println("No.  | App Name             | Size     | Data Locations")
	fmt.Println("-----+----------------------+----------+------------------")

	for i, app := range am.apps {
		numericID := i + 1
		size := formatBytes(app.TotalSize)
		numData := len(app.DataPaths)

		fmt.Printf("%2d   | %-20s | %8s | %d locations\n",
			numericID, app.Name, size, numData)
	}
	fmt.Println("-----+----------------------+----------+------------------")

	// Show total
	totalSize := int64(0)
	for _, app := range am.apps {
		totalSize += app.TotalSize
	}
	fmt.Printf("\nTotal space used by app data: %s\n", formatBytes(totalSize))

	return nil
}

// InteractiveUninstall shows a menu and lets user select apps to uninstall
func (am *AppManager) InteractiveUninstall() error {
	if err := am.LoadApps(); err != nil {
		return err
	}

	if len(am.apps) == 0 {
		fmt.Println("No apps with associated data found")
		return nil
	}

	fmt.Println("\nInstalled Apps with Associated Data\n")
	fmt.Println("Select apps to uninstall (with all related data):\n")

	// Show list
	for i, app := range am.apps {
		fmt.Printf("%2d. %-25s %8s  [%d data locations]\n",
			i+1, app.Name, formatBytes(app.TotalSize), len(app.DataPaths))
	}

	fmt.Println("\nEnter app numbers to uninstall (comma-separated, e.g., 1,3,5)")
	fmt.Print("Or press Enter to cancel: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		fmt.Println("Cancelled")
		return nil
	}

	// Parse selections
	selections := strings.Split(input, ",")
	var selectedApps []*AppData

	for _, sel := range selections {
		sel = strings.TrimSpace(sel)
		var idx int
		if _, err := fmt.Sscanf(sel, "%d", &idx); err != nil {
			fmt.Printf("Invalid selection: %s\n", sel)
			continue
		}
		if idx < 1 || idx > len(am.apps) {
			fmt.Printf("Selection out of range: %d\n", idx)
			continue
		}
		selectedApps = append(selectedApps, &am.apps[idx-1])
	}

	if len(selectedApps) == 0 {
		fmt.Println("No valid selections")
		return nil
	}

	// Confirm and delete
	return am.confirmAndDelete(selectedApps)
}

// UninstallApp uninstalls a specific app by name
func (am *AppManager) UninstallApp(targetName string, force bool) error {
	if err := am.LoadApps(); err != nil {
		return err
	}

	// Find matching app
	var selectedApp *AppData
	for i := range am.apps {
		if strings.EqualFold(am.apps[i].Name, targetName) ||
		   strings.Contains(strings.ToLower(am.apps[i].Name), strings.ToLower(targetName)) {
			selectedApp = &am.apps[i]
			break
		}
	}

	if selectedApp == nil {
		return fmt.Errorf("app not found: %s", targetName)
	}

	if !force {
		fmt.Printf("\nWARNING: This will uninstall %s and remove all related data\n", selectedApp.Name)
		fmt.Printf("Total space to be freed: %s\n\n", formatBytes(selectedApp.TotalSize))
		fmt.Print("Proceed? (y/N): ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	return am.deleteApp(selectedApp)
}

// confirmAndDelete shows confirmation and deletes selected apps
func (am *AppManager) confirmAndDelete(apps []*AppData) error {
	totalSize := int64(0)
	for _, app := range apps {
		totalSize += app.TotalSize
	}

	fmt.Printf("\nWARNING: Will uninstall %d app(s) and remove all related data\n", len(apps))
	fmt.Println("\nApps to be removed:")
	for _, app := range apps {
		fmt.Printf("  - %s (%s)\n", app.Name, formatBytes(app.TotalSize))
	}
	fmt.Printf("\nTotal space to be freed: %s\n\n", formatBytes(totalSize))
	fmt.Print("Proceed? (y/N): ")

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(response)) != "y" {
		fmt.Println("Cancelled")
		return nil
	}

	// Delete all selected apps
	for _, app := range apps {
		if err := am.deleteApp(app); err != nil {
			fmt.Printf(" Error uninstalling %s: %v\n", app.Name, err)
		}
	}

	return nil
}

// deleteApp removes an app and all its related data
func (am *AppManager) deleteApp(app *AppData) error {
	fmt.Printf("\nUninstalling %s...\n", app.Name)

	// Delete app bundle
	if _, err := os.Stat(app.AppPath); err == nil {
		fmt.Printf("  Removing app: %s\n", app.AppPath)
		if err := os.RemoveAll(app.AppPath); err != nil {
			fmt.Printf("    ERROR: Could not remove app: %v\n", err)
		} else {
			fmt.Printf("    [OK] Removed\n")
		}
	}

	// Delete all data paths
	totalFreed := int64(0)
	for _, dataPath := range app.DataPaths {
		if _, err := os.Stat(dataPath); err == nil {
			size := am.getDirSize(dataPath)
			fmt.Printf("  Removing data: %s (%s)\n", filepath.Base(dataPath), formatBytes(size))
			if err := os.RemoveAll(dataPath); err != nil {
				fmt.Printf("    ERROR: Could not remove: %v\n", err)
			} else {
				fmt.Printf("    [OK] Removed\n")
				totalFreed += size
			}
		}
	}

	fmt.Printf("\nSuccessfully uninstalled %s\n", app.Name)
	fmt.Printf("   Freed: %s\n", formatBytes(totalFreed))

	return nil
}
