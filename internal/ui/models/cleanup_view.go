package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fenilsonani/cleanup-cache/internal/cleaner"
	"github.com/fenilsonani/cleanup-cache/internal/config"
	"github.com/fenilsonani/cleanup-cache/internal/scanner"
	"github.com/fenilsonani/cleanup-cache/internal/ui/styles"
	"github.com/fenilsonani/cleanup-cache/pkg/utils"
)

// CleanupViewModel handles the cleanup progress view
type CleanupViewModel struct {
	files      []scanner.FileInfo
	config     *config.Config
	spinner    spinner.Model
	progress   progress.Model
	current    int
	startTime  time.Time
	result     *cleaner.CleanResult
	done       bool
}

// NewCleanupViewModel creates a new cleanup view model
func NewCleanupViewModel(files []scanner.FileInfo, cfg *config.Config) *CleanupViewModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.SelectedStyle

	p := progress.New(progress.WithDefaultGradient())

	return &CleanupViewModel{
		files:     files,
		config:    cfg,
		spinner:   s,
		progress:  p,
		startTime: time.Now(),
	}
}

// Init initializes the cleanup view
func (m *CleanupViewModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.performCleanup,
	)
}

// Update handles messages
func (m *CleanupViewModel) Update(msg tea.Msg) (*CleanupViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case CleanupCompleteMsg:
		m.done = true
		m.result = msg.Result.(*cleaner.CleanResult)
		return m, nil
	}

	return m, nil
}

// View renders the cleanup view
func (m *CleanupViewModel) View() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("🗑️  Cleaning Up"))
	b.WriteString("\n\n")

	if !m.done {
		b.WriteString(m.spinner.View())
		b.WriteString(" Deleting files... ")
		b.WriteString(styles.DimStyle.Render(fmt.Sprintf("(%s)", time.Since(m.startTime).Round(time.Second))))
		b.WriteString("\n\n")

		// Progress bar
		percent := float64(m.current) / float64(len(m.files))
		b.WriteString(m.progress.ViewAs(percent))
		b.WriteString("\n\n")

		b.WriteString(fmt.Sprintf("Progress: %d/%d files", m.current, len(m.files)))
	} else {
		b.WriteString(styles.SuccessStyle.Render("✓ Cleanup Complete!"))
		b.WriteString("\n\n")

		if m.result != nil {
			b.WriteString(fmt.Sprintf("Deleted: %d files\n", len(m.result.DeletedFiles)))
			if len(m.result.SkippedFiles) > 0 {
				b.WriteString(fmt.Sprintf("Skipped: %d files\n", len(m.result.SkippedFiles)))
			}
			b.WriteString(fmt.Sprintf("Size freed: %s\n", utils.FormatBytes(m.result.DeletedSize)))
		}

		b.WriteString("\n")
		b.WriteString(styles.HelpStyle.Render("Moving to summary..."))
	}

	return b.String()
}

// performCleanup performs the actual cleanup
func (m *CleanupViewModel) performCleanup() tea.Msg {
	// Create cleaner
	clnr := cleaner.New(m.config)

	// Create a ScanResult with selected files
	scanResult := &scanner.ScanResult{
		Files: m.files,
	}

	// Perform cleanup
	result, _ := clnr.Clean(scanResult)

	return CleanupCompleteMsg{Result: result}
}
