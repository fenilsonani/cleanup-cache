package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// LiveProgress handles live terminal progress display
type LiveProgress struct {
	mu           sync.Mutex
	currentPath  string
	filesFound   int
	totalSize    int64
	category     string
	startTime    time.Time
	lastUpdate   time.Time
	termWidth    int
	enabled      bool
	statusLines  int
}

// NewLiveProgress creates a new live progress display
func NewLiveProgress() *LiveProgress {
	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
	}

	return &LiveProgress{
		startTime:   time.Now(),
		termWidth:   width,
		enabled:     true,
		statusLines: 3,
	}
}

// Start initializes the progress display area
func (lp *LiveProgress) Start() {
	if !lp.enabled {
		return
	}
	// Reserve space for status lines
	fmt.Print("\n\n\n")
	// Move cursor up to the reserved area
	fmt.Printf("\033[%dA", lp.statusLines)
}

// Update updates the progress display
func (lp *LiveProgress) Update(category, currentPath string, filesFound int, totalSize int64) {
	lp.mu.Lock()
	defer lp.mu.Unlock()

	if !lp.enabled {
		return
	}

	// Throttle updates to avoid flickering (max 10 updates per second)
	now := time.Now()
	if now.Sub(lp.lastUpdate) < 100*time.Millisecond {
		return
	}
	lp.lastUpdate = now

	lp.category = category
	lp.currentPath = currentPath
	lp.filesFound = filesFound
	lp.totalSize = totalSize

	lp.render()
}

// render draws the progress display
func (lp *LiveProgress) render() {
	// Save cursor position
	fmt.Print("\033[s")

	// Clear lines and write status
	width := lp.termWidth - 2

	// Line 1: Category and stats
	elapsed := time.Since(lp.startTime).Round(time.Second)
	line1 := fmt.Sprintf("📂 Scanning: %-20s | Found: %d files | Size: %s | Time: %s",
		lp.category, lp.filesFound, formatBytes(lp.totalSize), elapsed)
	fmt.Printf("\033[K%s\n", truncate(line1, width))

	// Line 2: Current path with animation
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinIdx := int(time.Now().UnixMilli()/100) % len(spinner)
	path := lp.currentPath
	if len(path) > width-10 {
		// Show last part of path
		path = "..." + path[len(path)-(width-13):]
	}
	line2 := fmt.Sprintf("%s %s", spinner[spinIdx], path)
	fmt.Printf("\033[K%s\n", truncate(line2, width))

	// Line 3: Progress bar
	line3 := strings.Repeat("─", width)
	fmt.Printf("\033[K%s", line3)

	// Restore cursor position
	fmt.Print("\033[u")
}

// Finish completes the progress display
func (lp *LiveProgress) Finish() {
	if !lp.enabled {
		return
	}
	lp.mu.Lock()
	defer lp.mu.Unlock()

	// Move to the end and clear the status lines
	fmt.Printf("\033[%dB", lp.statusLines)
	fmt.Print("\033[K\n")
}

// SetEnabled enables or disables live progress
func (lp *LiveProgress) SetEnabled(enabled bool) {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	lp.enabled = enabled
}

// truncate truncates a string to fit width
func truncate(s string, width int) string {
	if len(s) <= width {
		return s
	}
	return s[:width-3] + "..."
}

// formatBytes formats bytes to human readable
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
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// PrintDetailedTree prints a detailed tree view of scan results
func PrintDetailedTree(files []FileInfo, totalSize int64) {
	// Group by category
	categories := make(map[string][]FileInfo)
	categorySizes := make(map[string]int64)

	for _, f := range files {
		categories[f.Category] = append(categories[f.Category], f)
		categorySizes[f.Category] += f.Size
	}

	// Print each category
	for cat, catFiles := range categories {
		fmt.Printf("\n╭─ %s (%s)\n", categoryName(cat), formatBytes(categorySizes[cat]))

		// Group by parent directory for tree view
		dirs := make(map[string][]FileInfo)
		for _, f := range catFiles {
			dir := getParentDir(f.Path)
			dirs[dir] = append(dirs[dir], f)
		}

		dirCount := 0
		totalDirs := len(dirs)
		for dir, dirFiles := range dirs {
			dirCount++
			isLastDir := dirCount == totalDirs

			// Directory connector
			connector := "├"
			if isLastDir {
				connector = "╰"
			}

			// Calculate dir total size
			var dirSize int64
			for _, f := range dirFiles {
				dirSize += f.Size
			}

			fmt.Printf("%s── 📁 %s (%s)\n", connector, dir, formatBytes(dirSize))

			// Show files in this directory (limit to 5, show count if more)
			maxFiles := 5
			fileCount := len(dirFiles)
			showCount := fileCount
			if showCount > maxFiles {
				showCount = maxFiles
			}

			for i := 0; i < showCount; i++ {
				f := dirFiles[i]
				fileConnector := "│   ├"
				if isLastDir {
					fileConnector = "    ├"
				}
				if i == showCount-1 && fileCount <= maxFiles {
					if isLastDir {
						fileConnector = "    ╰"
					} else {
						fileConnector = "│   ╰"
					}
				}
				fmt.Printf("%s── %s (%s)\n", fileConnector, getFileName(f.Path), formatBytes(f.Size))
			}

			if fileCount > maxFiles {
				moreConnector := "│   ╰"
				if isLastDir {
					moreConnector = "    ╰"
				}
				fmt.Printf("%s── ... and %d more files\n", moreConnector, fileCount-maxFiles)
			}
		}
	}

	fmt.Printf("\n════════════════════════════════════════════════════════\n")
	fmt.Printf("Total: %d items | %s\n", len(files), formatBytes(totalSize))
}

// FileInfo represents file information for display
type FileInfo struct {
	Path     string
	Size     int64
	Category string
	Reason   string
}

// categoryName returns a friendly name for a category
func categoryName(cat string) string {
	names := map[string]string{
		"node_modules":    "📦 Node Modules",
		"virtual_envs":    "🐍 Virtual Environments",
		"build_artifacts": "🔨 Build Artifacts",
		"cache":           "💾 Cache Files",
		"temp":            "  Temporary Files",
		"logs":            "📜 Log Files",
		"large_files":     "📀 Large Files",
		"old_files":       "📅 Old Files",
		"homebrew_cache":  "🍺 Homebrew Cache",
		"npm_cache":       "📦 NPM Cache",
		"go_cache":        "🐹 Go Cache",
	}
	if name, ok := names[cat]; ok {
		return name
	}
	return cat
}

// getParentDir extracts the parent directory from a path
func getParentDir(path string) string {
	// Find last separator
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return path
}

// getFileName extracts the file name from a path
func getFileName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
