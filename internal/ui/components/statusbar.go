package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/fenilsonani/cleanup-cache/internal/ui/styles"
	"github.com/fenilsonani/cleanup-cache/pkg/utils"
)

// StatusBar represents a status bar component that displays at the bottom of views
type StatusBar struct {
	viewName  string
	selected  int
	total     int
	size      int64
	shortcuts map[string]string
}

// NewStatusBar creates a new status bar
func NewStatusBar() *StatusBar {
	return &StatusBar{
		viewName:  "",
		selected:  0,
		total:     0,
		size:      0,
		shortcuts: make(map[string]string),
	}
}

// SetView sets the current view name
func (s *StatusBar) SetView(viewName string) {
	s.viewName = viewName
}

// SetSelection sets the selection count, total, and size
func (s *StatusBar) SetSelection(selected, total int, size int64) {
	s.selected = selected
	s.total = total
	s.size = size
}

// SetShortcuts sets the shortcuts to display
func (s *StatusBar) SetShortcuts(shortcuts map[string]string) {
	s.shortcuts = shortcuts
}

// Render renders the status bar with the given width
func (s *StatusBar) Render(width int) string {
	if width <= 0 {
		width = 80
	}

	var parts []string

	// View name
	if s.viewName != "" {
		parts = append(parts, styles.BoldStyle.Render(s.viewName))
	}

	// Selection info
	if s.total > 0 {
		selectionInfo := fmt.Sprintf("%d/%d selected", s.selected, s.total)
		parts = append(parts, selectionInfo)
	}

	// Size info
	if s.size > 0 {
		sizeInfo := utils.FormatBytes(s.size)
		parts = append(parts, styles.FileSizeStyle.Render(sizeInfo))
	}

	// Left side of status bar
	leftSide := strings.Join(parts, " • ")

	// Shortcuts (right side)
	var shortcutParts []string
	// Define order for common shortcuts
	orderedKeys := []string{"↑/↓", "space", "enter", "s", "/", "?", "esc", "q"}

	for _, key := range orderedKeys {
		if desc, ok := s.shortcuts[key]; ok {
			shortcutParts = append(shortcutParts, fmt.Sprintf("%s:%s",
				styles.DimStyle.Render(key), desc))
		}
	}

	// Add any remaining shortcuts not in ordered list
	for key, desc := range s.shortcuts {
		found := false
		for _, orderedKey := range orderedKeys {
			if key == orderedKey {
				found = true
				break
			}
		}
		if !found {
			shortcutParts = append(shortcutParts, fmt.Sprintf("%s:%s",
				styles.DimStyle.Render(key), desc))
		}
	}

	rightSide := strings.Join(shortcutParts, " ")

	// Calculate spacing
	leftLen := lipgloss.Width(leftSide)
	rightLen := lipgloss.Width(rightSide)
	spacing := width - leftLen - rightLen - 2 // -2 for padding

	if spacing < 1 {
		// Not enough space, truncate right side
		maxRightLen := width - leftLen - 5
		if maxRightLen > 0 && rightLen > maxRightLen {
			rightSide = rightSide[:maxRightLen-3] + "..."
		}
		spacing = 1
	}

	// Build the status bar
	statusLine := leftSide + strings.Repeat(" ", spacing) + rightSide

	// Style the status bar
	statusBarStyle := lipgloss.NewStyle().
		Foreground(styles.Text).
		Background(styles.BgDark).
		Padding(0, 1).
		Width(width)

	return statusBarStyle.Render(statusLine)
}

// RenderSimple renders a simple status bar with just a message
func RenderSimple(message string, width int) string {
	if width <= 0 {
		width = 80
	}

	statusBarStyle := lipgloss.NewStyle().
		Foreground(styles.Text).
		Background(styles.BgDark).
		Padding(0, 1).
		Width(width)

	return statusBarStyle.Render(message)
}
