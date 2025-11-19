package models

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fenilsonani/cleanup-cache/internal/scanner"
	"github.com/fenilsonani/cleanup-cache/internal/ui/styles"
	uiutils "github.com/fenilsonani/cleanup-cache/internal/ui/utils"
	"github.com/fenilsonani/cleanup-cache/pkg/utils"
)

// RiskLevel represents the risk level of a deletion operation
type RiskLevel int

const (
	RiskLow RiskLevel = iota
	RiskMedium
	RiskHigh
)

// ConfirmViewModel handles the confirmation screen
type ConfirmViewModel struct {
	files     []scanner.FileInfo
	cursor    int // 0 = Yes, 1 = Review, 2 = Cancel
	riskLevel RiskLevel
	width     int
	height    int
}

// NewConfirmViewModel creates a new confirm view model
func NewConfirmViewModel(files []scanner.FileInfo, width, height int) *ConfirmViewModel {
	// Determine default cursor based on risk
	risk := calculateRiskLevel(files)
	defaultCursor := 0 // Default to "Yes" if files were explicitly selected

	// Override for high risk
	if risk == RiskHigh {
		defaultCursor = 2 // Default to "Cancel" for high risk
	}

	// Use default dimensions if not provided
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	return &ConfirmViewModel{
		files:     files,
		cursor:    defaultCursor,
		riskLevel: risk,
		width:     width,
		height:    height,
	}
}

// calculateRiskLevel determines the risk level based on file count and categories
func calculateRiskLevel(files []scanner.FileInfo) RiskLevel {
	categoryCount := make(map[string]bool)
	for _, file := range files {
		categoryCount[file.Category] = true
	}

	// HIGH: > 500 files or includes package managers
	if len(files) > 500 || categoryCount["package_managers"] {
		return RiskHigh
	}

	// MEDIUM: 50-500 files, includes logs or multiple categories
	if len(files) >= 50 || categoryCount["logs"] || len(categoryCount) > 2 {
		return RiskMedium
	}

	// LOW: < 50 files, only cache/temp
	return RiskLow
}

// Init initializes the confirm view
func (m *ConfirmViewModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *ConfirmViewModel) Update(msg tea.Msg) (*ConfirmViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			if m.cursor > 0 {
				m.cursor--
			}
		case "right", "l":
			if m.cursor < 2 {
				m.cursor++
			}
		case "tab":
			m.cursor = (m.cursor + 1) % 3
		case "enter":
			switch m.cursor {
			case 0: // Yes
				return m, func() tea.Msg { return ConfirmedMsg{} }
			case 1: // Review
				return m, func() tea.Msg { return ReviewSelectionMsg{} }
			case 2: // Cancel
				return m, tea.Quit
			}
		case "y":
			// Quick confirm
			return m, func() tea.Msg { return ConfirmedMsg{} }
		case "e":
			// Edit/Review selection
			return m, func() tea.Msg { return ReviewSelectionMsg{} }
		case "n":
			// Quick cancel
			return m, tea.Quit
		}
	}

	return m, nil
}

// View renders the confirmation view
func (m *ConfirmViewModel) View() string {
	var b strings.Builder

	// Show warning if terminal is too small
	if warning := uiutils.GetSizeWarningBanner(m.width, m.height); warning != "" {
		b.WriteString(warning)
	}

	// Title with warning
	b.WriteString(styles.TitleStyle.Render("⚠️  Confirm Deletion"))
	b.WriteString("\n\n")

	// Summary
	var totalSize int64
	categoryBreakdown := make(map[string]struct {
		count int
		size  int64
	})

	for _, file := range m.files {
		totalSize += file.Size
		entry := categoryBreakdown[file.Category]
		entry.count++
		entry.size += file.Size
		categoryBreakdown[file.Category] = entry
	}

	b.WriteString(styles.BoldStyle.Render(fmt.Sprintf("You are about to delete %d files (%s)",
		len(m.files), utils.FormatBytes(totalSize))))
	b.WriteString("\n\n")

	// Breakdown by category with sizes
	b.WriteString(styles.SubtitleStyle.Render("Breakdown:"))
	b.WriteString("\n")

	// Order categories for consistent display
	orderedCats := []string{"cache", "temp", "logs", "duplicates", "downloads", "package_managers"}
	for _, catName := range orderedCats {
		if entry, ok := categoryBreakdown[catName]; ok {
			b.WriteString(fmt.Sprintf("  %-15s %3d files (%s)\n",
				styles.CategoryStyle.Render(formatCategoryName(catName)+":"),
				entry.count,
				styles.FileSizeStyle.Render(utils.FormatBytes(entry.size))))
		}
	}

	// Add any other categories not in ordered list
	for catName, entry := range categoryBreakdown {
		found := false
		for _, ordered := range orderedCats {
			if catName == ordered {
				found = true
				break
			}
		}
		if !found {
			b.WriteString(fmt.Sprintf("  %-15s %3d files (%s)\n",
				styles.CategoryStyle.Render(formatCategoryName(catName)+":"),
				entry.count,
				styles.FileSizeStyle.Render(utils.FormatBytes(entry.size))))
		}
	}

	b.WriteString("\n")

	// Risk level indicator
	riskText, riskStyle, riskIcon := m.getRiskDisplay()
	b.WriteString(fmt.Sprintf("Risk Level: %s %s\n",
		riskIcon,
		riskStyle(riskText)))

	// Show warning banner for high-risk operations
	if m.riskLevel == RiskHigh {
		b.WriteString("\n")
		b.WriteString(styles.ErrorStyle.Render("⚠️  HIGH RISK OPERATION ⚠️"))
		b.WriteString("\n")
		b.WriteString(styles.WarningStyle.Render("This includes package managers or a large number of files!"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.WarningStyle.Render("⚠️  This action cannot be undone!"))
	b.WriteString("\n\n")

	// Three buttons: Yes, Review, Cancel
	yesBtn := "[ Yes, delete ]"
	reviewBtn := "[ Review ]"
	cancelBtn := "[ Cancel ]"

	switch m.cursor {
	case 0:
		yesBtn = styles.HighlightStyle.Render("[ Yes, delete ]")
	case 1:
		reviewBtn = styles.HighlightStyle.Render("[ Review ]")
	case 2:
		cancelBtn = styles.HighlightStyle.Render("[ Cancel ]")
	}

	b.WriteString(fmt.Sprintf("%s  %s  %s", yesBtn, reviewBtn, cancelBtn))
	b.WriteString("\n\n")

	// Adjust help text based on terminal width
	helpText := "y:confirm  e:edit  n:cancel  ←/→:navigate"
	if m.width < 60 {
		helpText = "y:yes  e:edit  n:no  ←/→"
	}
	b.WriteString(styles.HelpStyle.Render(helpText))

	return b.String()
}

// getRiskDisplay returns the display text, style render function, and icon for the current risk level
func (m *ConfirmViewModel) getRiskDisplay() (string, func(string) string, string) {
	switch m.riskLevel {
	case RiskHigh:
		return "HIGH (includes package managers or many files)", func(s string) string { return styles.ErrorStyle.Render(s) }, "🔴"
	case RiskMedium:
		return "MEDIUM (includes system caches)", func(s string) string { return styles.WarningStyle.Render(s) }, "⚠️"
	default:
		return "LOW (temporary/cache files only)", func(s string) string { return styles.SuccessStyle.Render(s) }, "✓"
	}
}

// formatCategoryName formats category names for display
func formatCategoryName(cat string) string {
	switch cat {
	case "cache":
		return "Cache"
	case "temp":
		return "Temp"
	case "logs":
		return "Logs"
	case "duplicates":
		return "Duplicates"
	case "downloads":
		return "Downloads"
	case "package_managers":
		return "Pkg Managers"
	default:
		return strings.Title(strings.ReplaceAll(cat, "_", " "))
	}
}
