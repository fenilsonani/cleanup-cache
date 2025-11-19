package models

import (
	"fmt"
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
	Name  string
	Count int
	Size  int64
	Done  bool
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
				Name: msg.Category,
			}
		}
		m.progress[msg.Category].Count = msg.Count
		m.progress[msg.Category].Size = msg.Size
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
		// Spinner and current status
		b.WriteString(m.spinner.View())
		b.WriteString(" Scanning... ")
		b.WriteString(styles.DimStyle.Render(fmt.Sprintf("(%s)", time.Since(m.startTime).Round(time.Second))))
		b.WriteString("\n\n")

		// Current directory
		if m.currentDir != "" {
			b.WriteString(styles.DimStyle.Render("Current: "))
			b.WriteString(styles.FilePathStyle.Render(uiutils.TruncatePath(m.currentDir, pathWidth)))
			b.WriteString("\n\n")
		}

		// Progress by category
		b.WriteString(styles.SubtitleStyle.Render("Progress by Category:"))
		b.WriteString("\n")

		totalFiles := 0
		var totalSize int64

		for _, category := range []string{"cache", "temp", "logs", "duplicates", "downloads", "package_managers"} {
			if prog, ok := m.progress[category]; ok && prog.Count > 0 {
				b.WriteString(fmt.Sprintf("  %s: %s files, %s\n",
					styles.CategoryStyle.Render(category),
					styles.BoldStyle.Render(fmt.Sprintf("%d", prog.Count)),
					styles.FileSizeStyle.Render(utils.FormatBytes(prog.Size)),
				))
				totalFiles += prog.Count
				totalSize += prog.Size
			}
		}

		b.WriteString("\n")
		b.WriteString(styles.BoldStyle.Render(fmt.Sprintf("Total: %d files, %s",
			totalFiles,
			utils.FormatBytes(totalSize))))

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
	statusBar.SetView("Scanning")

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

