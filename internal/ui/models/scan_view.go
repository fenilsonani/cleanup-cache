package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fenilsonani/cleanup-cache/internal/config"
	"github.com/fenilsonani/cleanup-cache/internal/platform"
	"github.com/fenilsonani/cleanup-cache/internal/scanner"
	"github.com/fenilsonani/cleanup-cache/internal/ui/styles"
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
}

// CategoryProgress tracks progress for each category
type CategoryProgress struct {
	Name  string
	Count int
	Size  int64
	Done  bool
}

// NewScanViewModel creates a new scan view model
func NewScanViewModel(cfg *config.Config, platformInfo *platform.Info) *ScanViewModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.SelectedStyle

	return &ScanViewModel{
		config:       cfg,
		platformInfo: platformInfo,
		spinner:      s,
		scanning:     true,
		startTime:    time.Now(),
		progress:     make(map[string]*CategoryProgress),
	}
}

// Init initializes the scan view
func (m *ScanViewModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.performScan,
	)
}

// Update handles messages
func (m *ScanViewModel) Update(msg tea.Msg) (*ScanViewModel, tea.Cmd) {
	switch msg := msg.(type) {
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
		return m, nil

	case ScanCompleteMsg:
		m.scanning = false
		m.result = msg.Result
		return m, nil
	}

	return m, nil
}

// View renders the scan view
func (m *ScanViewModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(styles.TitleStyle.Render("🔍 Scanning System"))
	b.WriteString("\n\n")

	if m.scanning {
		// Spinner and current status
		b.WriteString(m.spinner.View())
		b.WriteString(" Scanning... ")
		b.WriteString(styles.DimStyle.Render(fmt.Sprintf("(%s)", time.Since(m.startTime).Round(time.Second))))
		b.WriteString("\n\n")

		// Current directory
		if m.currentDir != "" {
			b.WriteString(styles.DimStyle.Render("Current: "))
			b.WriteString(styles.FilePathStyle.Render(truncatePath(m.currentDir, 60)))
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
	b.WriteString(styles.HelpStyle.Render("Press ctrl+c to cancel"))

	return b.String()
}

// performScan performs the actual scanning
func (m *ScanViewModel) performScan() tea.Msg {
	// Create scanner
	scnr := scanner.New(m.config, m.platformInfo)

	// Perform scan
	result, err := scnr.ScanAll()
	if err != nil {
		return ScanCompleteMsg{Result: result} // Return partial results
	}

	return ScanCompleteMsg{Result: result}
}

// ScanProgressMsg is sent during scanning to update progress
type ScanProgressMsg struct {
	Category    string
	Count       int
	Size        int64
	CurrentPath string
}

// Helper function to truncate paths
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}
