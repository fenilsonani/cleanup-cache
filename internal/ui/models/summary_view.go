package models

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fenilsonani/cleanup-cache/internal/cleaner"
	"github.com/fenilsonani/cleanup-cache/internal/ui/components"
	"github.com/fenilsonani/cleanup-cache/internal/ui/styles"
	uiutils "github.com/fenilsonani/cleanup-cache/internal/ui/utils"
	"github.com/fenilsonani/cleanup-cache/pkg/utils"
)

// SummaryViewModel handles the summary/results view
type SummaryViewModel struct {
	result *cleaner.CleanResult
	width  int
	height int
}

// NewSummaryViewModel creates a new summary view model
func NewSummaryViewModel(result interface{}, width, height int) *SummaryViewModel {
	// Use default dimensions if not provided
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	return &SummaryViewModel{
		result: result.(*cleaner.CleanResult),
		width:  width,
		height: height,
	}
}

// Init initializes the summary view
func (m *SummaryViewModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *SummaryViewModel) Update(msg tea.Msg) (*SummaryViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "enter":
			return m, tea.Quit
		}
	}

	return m, nil
}

// View renders the summary view
func (m *SummaryViewModel) View() string {
	var b strings.Builder

	// Show warning if terminal is too small
	if warning := uiutils.GetSizeWarningBanner(m.width, m.height); warning != "" {
		b.WriteString(warning)
	}

	b.WriteString(styles.TitleStyle.Render("✨ Cleanup Summary"))
	b.WriteString("\n\n")

	if m.result != nil {
		// Success metrics
		b.WriteString(styles.SuccessStyle.Render(fmt.Sprintf("✓ Successfully deleted %d files",
			len(m.result.DeletedFiles))))
		b.WriteString("\n")

		b.WriteString(styles.BoldStyle.Render(fmt.Sprintf("Space freed: %s",
			utils.FormatBytes(m.result.DeletedSize))))
		b.WriteString("\n\n")

		// Skipped files
		if len(m.result.SkippedFiles) > 0 {
			b.WriteString(styles.WarningStyle.Render(fmt.Sprintf("⚠ Skipped %d files",
				len(m.result.SkippedFiles))))
			b.WriteString("\n")
		}

		// Errors
		if len(m.result.Errors) > 0 {
			b.WriteString(styles.ErrorStyle.Render(fmt.Sprintf("✗ %d errors occurred",
				len(m.result.Errors))))
			b.WriteString("\n")
		}

		// Dry run note
		if m.result.DryRun {
			b.WriteString("\n")
			b.WriteString(styles.InfoStyle.Render("Note: This was a dry run. No files were actually deleted."))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\n")

	// Status bar
	statusBar := components.NewStatusBar()
	statusBar.SetView("Summary")

	var deletedCount int
	var freedSpace int64
	if m.result != nil {
		deletedCount = len(m.result.DeletedFiles)
		freedSpace = m.result.DeletedSize
	}

	statusBar.SetSelection(deletedCount, deletedCount, freedSpace)
	statusBar.SetShortcuts(map[string]string{
		"enter": "exit",
		"q":     "quit",
	})

	b.WriteString(statusBar.Render(m.width))

	return b.String()
}
