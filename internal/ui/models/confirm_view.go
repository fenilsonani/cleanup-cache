package models

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fenilsonani/cleanup-cache/internal/scanner"
	"github.com/fenilsonani/cleanup-cache/internal/ui/styles"
	"github.com/fenilsonani/cleanup-cache/pkg/utils"
)

// ConfirmViewModel handles the confirmation screen
type ConfirmViewModel struct {
	files  []scanner.FileInfo
	cursor int // 0 = Yes, 1 = No
}

// NewConfirmViewModel creates a new confirm view model
func NewConfirmViewModel(files []scanner.FileInfo) *ConfirmViewModel {
	return &ConfirmViewModel{
		files:  files,
		cursor: 1, // Default to "No" for safety
	}
}

// Init initializes the confirm view
func (m *ConfirmViewModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *ConfirmViewModel) Update(msg tea.Msg) (*ConfirmViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "right", "h", "l":
			m.cursor = 1 - m.cursor
		case "enter":
			if m.cursor == 0 {
				// User confirmed
				return m, func() tea.Msg { return ConfirmedMsg{} }
			} else {
				// User cancelled
				return m, tea.Quit
			}
		case "y":
			return m, func() tea.Msg { return ConfirmedMsg{} }
		case "n":
			return m, tea.Quit
		}
	}

	return m, nil
}

// View renders the confirmation view
func (m *ConfirmViewModel) View() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("⚠️  Confirm Deletion"))
	b.WriteString("\n\n")

	// Summary
	var totalSize int64
	categoryCount := make(map[string]int)
	for _, file := range m.files {
		totalSize += file.Size
		categoryCount[file.Category]++
	}

	b.WriteString(styles.BoldStyle.Render(fmt.Sprintf("You are about to delete %d files (%s)",
		len(m.files), utils.FormatBytes(totalSize))))
	b.WriteString("\n\n")

	// Breakdown by category
	b.WriteString(styles.SubtitleStyle.Render("Breakdown by category:"))
	b.WriteString("\n")
	for cat, count := range categoryCount {
		b.WriteString(fmt.Sprintf("  %s: %d files\n",
			styles.CategoryStyle.Render(cat),
			count))
	}

	b.WriteString("\n")
	b.WriteString(styles.WarningStyle.Render("This action cannot be undone!"))
	b.WriteString("\n\n")

	// Buttons
	yesBtn := "[ Yes ]"
	noBtn := "[ No ]"

	if m.cursor == 0 {
		yesBtn = styles.HighlightStyle.Render("[ Yes ]")
	} else {
		noBtn = styles.HighlightStyle.Render("[ No ]")
	}

	b.WriteString(fmt.Sprintf("%s  %s", yesBtn, noBtn))
	b.WriteString("\n\n")

	b.WriteString(styles.HelpStyle.Render("Use ←/→ to choose, enter to confirm, or press y/n"))

	return b.String()
}
