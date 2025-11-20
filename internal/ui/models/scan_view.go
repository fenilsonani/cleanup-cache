package models

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fenilsonani/cleanup-cache/internal/config"
	"github.com/fenilsonani/cleanup-cache/internal/platform"
	"github.com/fenilsonani/cleanup-cache/internal/progress"
	"github.com/fenilsonani/cleanup-cache/internal/scanner"
	"github.com/fenilsonani/cleanup-cache/internal/ui/components"
	"github.com/fenilsonani/cleanup-cache/internal/ui/styles"
	uiutils "github.com/fenilsonani/cleanup-cache/internal/ui/utils"
	"github.com/fenilsonani/cleanup-cache/pkg/utils"
)

// ScanViewModel handles the scanning progress view
type ScanViewModel struct {
	config       *config.Config
	platformInfo *platform.Info
	spinner      spinner.Model
	scanning     bool
	result       *scanner.ScanResult
	startTime    time.Time
	currentDir   string
	progress     map[string]*CategoryProgress
	scanner      *scanner.Scanner
	progressCh   <-chan interface{}
	resultCh     chan *scanner.ScanResult
	width        int
	height       int
}

// CategoryProgress tracks progress for each category
type CategoryProgress struct {
	Name         string
	Count        int
	Size         int64
	Done         bool
	LastUpdate   time.Time
	DirectoryHit map[string]int // Track file counts per directory
}

// NewScanViewModel creates a new scan view model
func NewScanViewModel(cfg *config.Config, platformInfo *platform.Info, width, height int) *ScanViewModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.SelectedStyle

	// Use default dimensions if not provided
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	return &ScanViewModel{
		config:       cfg,
		platformInfo: platformInfo,
		spinner:      s,
		scanning:     true,
		startTime:    time.Now(),
		progress:     make(map[string]*CategoryProgress),
		width:        width,
		height:       height,
	}
}

// Init initializes the scan view
func (m *ScanViewModel) Init() tea.Cmd {
	// Create scanner and subscribe to progress
	m.scanner = scanner.New(m.config, m.platformInfo)
	m.progressCh = m.scanner.GetProgressReporter().Subscribe()
	m.resultCh = make(chan *scanner.ScanResult, 1)

	// Start scan in background
	go func() {
		result, _ := m.scanner.ScanAll()
		m.resultCh <- result
		close(m.resultCh)
	}()

	return tea.Batch(
		m.spinner.Tick,
		m.pollProgress,
	)
}

// Update handles messages
func (m *ScanViewModel) Update(msg tea.Msg) (*ScanViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case ScanProgressMsg:
		// Update progress
		if m.progress[msg.Category] == nil {
			m.progress[msg.Category] = &CategoryProgress{
				Name:         msg.Category,
				DirectoryHit: make(map[string]int),
			}
		}
		m.progress[msg.Category].Count = msg.Count
		m.progress[msg.Category].Size = msg.Size
		m.progress[msg.Category].LastUpdate = time.Now()

		// Track directory hits for "hot folders"
		if msg.CurrentPath != "" {
			dir := msg.CurrentPath
			// Extract parent directory
			if idx := strings.LastIndex(dir, "/"); idx > 0 {
				dir = dir[:idx]
			}
			m.progress[msg.Category].DirectoryHit[dir]++
		}

		m.currentDir = msg.CurrentPath
		// Continue polling
		return m, m.pollProgress

	case ScanCompleteMsg:
		m.scanning = false
		m.result = msg.Result
		return m, nil

	case PollMsg:
		// Continue polling for progress
		if m.scanning {
			return m, m.pollProgress
		}
		return m, nil
	}

	return m, nil
}

// View renders the scan view
func (m *ScanViewModel) View() string {
	var b strings.Builder

	// Show warning if terminal is too small
	if warning := uiutils.GetSizeWarningBanner(m.width, m.height); warning != "" {
		b.WriteString(warning)
	}

	// Title
	b.WriteString(styles.TitleStyle.Render("🔍 Scanning System"))
	b.WriteString("\n\n")

	// Calculate available width for paths (accounting for labels and spacing)
	pathWidth := m.width - 20 // Reserve space for labels
	if pathWidth < 30 {
		pathWidth = 30
	}

	if m.scanning {
		// Calculate metrics
		elapsed := time.Since(m.startTime)
		totalFiles := 0
		var totalSize int64

		for _, prog := range m.progress {
			totalFiles += prog.Count
			totalSize += prog.Size
		}

		// Calculate rates
		elapsedSecs := elapsed.Seconds()
		filesPerSec := 0.0
		bytesPerSec := 0.0
		if elapsedSecs > 0 {
			filesPerSec = float64(totalFiles) / elapsedSecs
			bytesPerSec = float64(totalSize) / elapsedSecs
		}

		// Calculate percentage (estimate based on typical scan time)
		percentage := 0
		if totalFiles > 0 {
			// Simple heuristic: assume 5000 files is 100%
			percentage = (totalFiles * 100) / 5000
			if percentage > 100 {
				percentage = 100
			}
		}

		// Spinner and status with percentage
		b.WriteString(m.spinner.View())
		b.WriteString(fmt.Sprintf(" Scanning... %s", styles.BoldStyle.Render(fmt.Sprintf("%d%%", percentage))))
		b.WriteString("\n")

		// Progress bar
		barWidth := 40
		if m.width < 100 {
			barWidth = 20
		}
		progressBar := styles.ProgressBar(percentage, 100, barWidth)
		b.WriteString(progressBar)
		b.WriteString("\n\n")

		// Metrics section
		b.WriteString(styles.SubtitleStyle.Render("📊 Metrics:"))
		b.WriteString("\n")

		// Format elapsed time as MM:SS
		minutes := int(elapsed.Minutes())
		seconds := int(elapsed.Seconds()) % 60
		elapsedStr := fmt.Sprintf("%02d:%02d", minutes, seconds)

		// Speed metrics
		b.WriteString(fmt.Sprintf("  Speed: %s, %s/sec\n",
			styles.BoldStyle.Render(fmt.Sprintf("%s files/sec", formatNumber(int(filesPerSec)))),
			styles.FileSizeStyle.Render(formatBytes(int64(bytesPerSec))),
		))
		b.WriteString(fmt.Sprintf("  Elapsed: %s\n",
			styles.DimStyle.Render(elapsedStr),
		))
		b.WriteString(fmt.Sprintf("  Found: %s files, %s\n\n",
			styles.BoldStyle.Render(formatNumber(totalFiles)),
			styles.GetFileSizeStyle(totalSize).Render(utils.FormatBytes(totalSize)),
		))

		// Hot Folders section
		hotFolders := m.getHotFolders(3)
		if len(hotFolders) > 0 {
			b.WriteString(styles.SubtitleStyle.Render("🔥 Hot Folders:"))
			b.WriteString("\n")
			for _, hf := range hotFolders {
				truncPath := uiutils.TruncatePath(hf.Path, pathWidth-20)
				b.WriteString(fmt.Sprintf("  %s %s\n",
					styles.FilePathStyle.Render(truncPath),
					styles.DimStyle.Render(fmt.Sprintf("(%s files)", formatNumber(hf.Count))),
				))
			}
			b.WriteString("\n")
		}

		// Current directory
		if m.currentDir != "" {
			b.WriteString(styles.DimStyle.Render("Current: "))
			b.WriteString(styles.FilePathStyle.Render(uiutils.TruncatePath(m.currentDir, pathWidth)))
			b.WriteString("\n")
		}

	} else {
		// Scan complete
		b.WriteString(styles.SuccessStyle.Render("✓ Scan Complete!"))
		b.WriteString("\n\n")

		if m.result != nil {
			b.WriteString(fmt.Sprintf("Found %s files totaling %s\n",
				styles.BoldStyle.Render(fmt.Sprintf("%d", m.result.TotalCount)),
				styles.FileSizeStyle.Render(utils.FormatBytes(m.result.TotalSize)),
			))
		}

		b.WriteString("\n")
		b.WriteString(styles.HelpStyle.Render("Moving to category selection..."))
	}

	b.WriteString("\n\n")

	// Status bar
	statusBar := components.NewStatusBar()

	// Set workflow breadcrumbs
	workflowSteps := []string{"Scan", "Categories", "Files", "Confirm", "Clean"}
	statusBar.SetWorkflowStep(1, len(workflowSteps), workflowSteps)

	// Calculate totals for status bar
	totalFiles := 0
	var totalSize int64
	for _, prog := range m.progress {
		totalFiles += prog.Count
		totalSize += prog.Size
	}

	statusBar.SetSelection(0, totalFiles, totalSize)
	statusBar.SetShortcuts(map[string]string{
		"?":      "help",
		"ctrl+c": "cancel",
	})

	b.WriteString(statusBar.Render(m.width))

	return b.String()
}

// pollProgress polls for scan progress updates
func (m *ScanViewModel) pollProgress() tea.Msg {
	// Check if scan is complete
	select {
	case result := <-m.resultCh:
		// Scan finished
		if m.scanner != nil {
			m.scanner.GetProgressReporter().Unsubscribe(m.progressCh)
		}
		return ScanCompleteMsg{Result: result}
	case update := <-m.progressCh:
		// Progress update received
		if scanProgress, ok := update.(*progress.ScanProgress); ok {
			return ScanProgressMsg{
				Category:    scanProgress.Category,
				Count:       scanProgress.FilesFound,
				Size:        scanProgress.TotalSize,
				CurrentPath: scanProgress.CurrentPath,
			}
		}
	case <-time.After(100 * time.Millisecond):
		// Timeout - continue polling
		return PollMsg{}
	}

	return PollMsg{}
}

// PollMsg signals to continue polling
type PollMsg struct{}

// ScanProgressMsg is sent during scanning to update progress
type ScanProgressMsg struct {
	Category    string
	Count       int
	Size        int64
	CurrentPath string
}

// HotFolder represents a directory with high file activity
type HotFolder struct {
	Path  string
	Count int
}

// getHotFolders returns the top N directories by file count
func (m *ScanViewModel) getHotFolders(n int) []HotFolder {
	// Aggregate directory hits across all categories
	dirCounts := make(map[string]int)
	for _, prog := range m.progress {
		for dir, count := range prog.DirectoryHit {
			dirCounts[dir] += count
		}
	}

	// Convert to slice and sort
	var hotFolders []HotFolder
	for dir, count := range dirCounts {
		hotFolders = append(hotFolders, HotFolder{Path: dir, Count: count})
	}

	// Sort by count descending
	sort.Slice(hotFolders, func(i, j int) bool {
		return hotFolders[i].Count > hotFolders[j].Count
	})

	// Return top N
	if len(hotFolders) > n {
		hotFolders = hotFolders[:n]
	}

	return hotFolders
}

// formatNumber formats a number with thousand separators
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	// Format with commas
	str := fmt.Sprintf("%d", n)
	var result strings.Builder
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}

// formatBytes formats bytes in a compact way for rates
func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
}

