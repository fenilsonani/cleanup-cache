package models

import (
	"fmt"
	"strings"
	"time"

	bubblesProgress "github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fenilsonani/cleanup-cache/internal/cleaner"
	"github.com/fenilsonani/cleanup-cache/internal/config"
	"github.com/fenilsonani/cleanup-cache/internal/progress"
	"github.com/fenilsonani/cleanup-cache/internal/scanner"
	"github.com/fenilsonani/cleanup-cache/internal/ui/components"
	"github.com/fenilsonani/cleanup-cache/internal/ui/styles"
	uiutils "github.com/fenilsonani/cleanup-cache/internal/ui/utils"
	"github.com/fenilsonani/cleanup-cache/pkg/utils"
)

// CleanupViewModel handles the cleanup progress view
type CleanupViewModel struct {
	files        []scanner.FileInfo
	config       *config.Config
	spinner      spinner.Model
	progressBar  bubblesProgress.Model
	current      int
	currentFile  string
	deletionRate float64 // files per second
	startTime    time.Time
	result       *cleaner.CleanResult
	done         bool
	cleaner      *cleaner.Cleaner
	progressCh   <-chan interface{}
	resultCh     chan *cleaner.CleanResult
	width        int
	height       int
}

// NewCleanupViewModel creates a new cleanup view model
func NewCleanupViewModel(files []scanner.FileInfo, cfg *config.Config, width, height int) *CleanupViewModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.SelectedStyle

	p := bubblesProgress.New(bubblesProgress.WithDefaultGradient())

	// Use default dimensions if not provided
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	return &CleanupViewModel{
		files:       files,
		config:      cfg,
		spinner:     s,
		progressBar: p,
		startTime:   time.Now(),
		width:       width,
		height:      height,
	}
}

// Init initializes the cleanup view
func (m *CleanupViewModel) Init() tea.Cmd {
	// Create cleaner and subscribe to progress
	m.cleaner = cleaner.New(m.config)
	m.progressCh = m.cleaner.GetProgressReporter().Subscribe()
	m.resultCh = make(chan *cleaner.CleanResult, 1)

	// Start cleanup in background
	go func() {
		// Create a ScanResult with selected files
		scanResult := &scanner.ScanResult{
			Files: m.files,
		}

		// Perform cleanup
		result, _ := m.cleaner.Clean(scanResult)
		m.resultCh <- result
		close(m.resultCh)
	}()

	return tea.Batch(
		m.spinner.Tick,
		m.pollProgress,
	)
}

// Update handles messages
func (m *CleanupViewModel) Update(msg tea.Msg) (*CleanupViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case CleanupProgressMsg:
		// Update progress
		m.current = msg.DeletedFiles
		m.currentFile = msg.CurrentFile
		m.deletionRate = msg.Rate
		// Continue polling
		return m, m.pollProgress

	case CleanupCompleteMsg:
		m.done = true
		m.result = msg.Result.(*cleaner.CleanResult)
		return m, nil

	case CleanupPollMsg:
		// Continue polling for progress
		if !m.done {
			return m, m.pollProgress
		}
		return m, nil
	}

	return m, nil
}

// View renders the cleanup view
func (m *CleanupViewModel) View() string {
	var b strings.Builder

	// Show warning if terminal is too small
	if warning := uiutils.GetSizeWarningBanner(m.width, m.height); warning != "" {
		b.WriteString(warning)
	}

	b.WriteString(styles.TitleStyle.Render("🗑️  Cleaning Up"))
	b.WriteString("\n\n")

	if !m.done {
		b.WriteString(m.spinner.View())
		b.WriteString(" Deleting files... ")
		elapsed := time.Since(m.startTime).Round(time.Second)
		b.WriteString(styles.DimStyle.Render(fmt.Sprintf("(%s)", elapsed)))
		b.WriteString("\n\n")

		// Calculate available width for paths
		pathWidth := m.width - 20
		if pathWidth < 30 {
			pathWidth = 30
		}

		// Current file being deleted
		if m.currentFile != "" {
			b.WriteString(styles.DimStyle.Render("Current: "))
			b.WriteString(styles.FilePathStyle.Render(uiutils.TruncatePath(m.currentFile, pathWidth)))
			b.WriteString("\n\n")
		}

		// Progress bar
		percent := float64(m.current) / float64(len(m.files))
		b.WriteString(m.progressBar.ViewAs(percent))
		b.WriteString("\n\n")

		// Progress with rate and ETA
		b.WriteString(fmt.Sprintf("Progress: %d/%d files", m.current, len(m.files)))

		if m.deletionRate > 0 {
			b.WriteString(fmt.Sprintf(" • %s/s", styles.DimStyle.Render(fmt.Sprintf("%.1f files", m.deletionRate))))

			// Calculate ETA
			if m.current > 0 && m.current < len(m.files) {
				remaining := len(m.files) - m.current
				etaSeconds := float64(remaining) / m.deletionRate
				eta := time.Duration(etaSeconds) * time.Second
				b.WriteString(fmt.Sprintf(" • ETA: %s", styles.DimStyle.Render(eta.Round(time.Second).String())))
			}
		}
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

	b.WriteString("\n\n")

	// Status bar
	statusBar := components.NewStatusBar()
	if m.done {
		statusBar.SetView("Cleanup Complete")
	} else {
		statusBar.SetView("Cleaning")
	}

	statusBar.SetSelection(m.current, len(m.files), 0)
	if m.done {
		statusBar.SetShortcuts(map[string]string{
			"enter": "view summary",
		})
	} else {
		statusBar.SetShortcuts(map[string]string{
			"ctrl+c": "cancel",
		})
	}

	b.WriteString(statusBar.Render(m.width))

	return b.String()
}

// pollProgress polls for cleanup progress updates
func (m *CleanupViewModel) pollProgress() tea.Msg {
	// Check if cleanup is complete
	select {
	case result := <-m.resultCh:
		// Cleanup finished
		if m.cleaner != nil {
			m.cleaner.GetProgressReporter().Unsubscribe(m.progressCh)
		}
		return CleanupCompleteMsg{Result: result}
	case update := <-m.progressCh:
		// Progress update received
		if cleanProgress, ok := update.(*progress.CleanProgress); ok {
			// Calculate deletion rate
			elapsed := time.Since(m.startTime).Seconds()
			rate := 0.0
			if elapsed > 0 {
				rate = float64(cleanProgress.DeletedFiles) / elapsed
			}

			return CleanupProgressMsg{
				CurrentFile:  cleanProgress.CurrentFile,
				DeletedFiles: cleanProgress.DeletedFiles,
				TotalFiles:   cleanProgress.TotalFiles,
				Rate:         rate,
			}
		}
	case <-time.After(100 * time.Millisecond):
		// Timeout - continue polling
		return CleanupPollMsg{}
	}

	return CleanupPollMsg{}
}

// CleanupProgressMsg is sent during cleanup to update progress
type CleanupProgressMsg struct {
	CurrentFile  string
	DeletedFiles int
	TotalFiles   int
	Rate         float64 // files per second
}

// CleanupPollMsg signals to continue polling
type CleanupPollMsg struct{}
