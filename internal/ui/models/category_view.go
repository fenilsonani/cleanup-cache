package models

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fenilsonani/cleanup-cache/internal/scanner"
	"github.com/fenilsonani/cleanup-cache/internal/ui/components"
	"github.com/fenilsonani/cleanup-cache/internal/ui/styles"
	uiutils "github.com/fenilsonani/cleanup-cache/internal/ui/utils"
	"github.com/fenilsonani/cleanup-cache/pkg/utils"
)

// SafetyLevel represents the safety level of a category
type SafetyLevel int

const (
	SafetyLow SafetyLevel = iota
	SafetyMedium
	SafetyHigh
)

// CategoryItem represents a selectable category
type CategoryItem struct {
	Name        string
	Count       int
	Size        int64
	Selected    bool
	SafetyLevel SafetyLevel
	Recommended bool
	Description string
}

// CategoryViewModel handles category selection
type CategoryViewModel struct {
	scanResult *scanner.ScanResult
	categories []CategoryItem
	cursor     int
	width      int
	height     int
}

// NewCategoryViewModel creates a new category view model
func NewCategoryViewModel(scanResult *scanner.ScanResult, width, height int) *CategoryViewModel {
	// Group scan results by category
	grouped := scanResult.GroupByCategory()

	// Create category items with smart defaults
	var categories []CategoryItem
	for _, catName := range []string{"cache", "temp", "logs", "duplicates", "downloads", "package_managers"} {
		if catResult, ok := grouped[catName]; ok && catResult.TotalCount > 0 {
			// Determine safety level and defaults
			safetyLevel, recommended, selected := getCategoryDefaults(catName)

			categories = append(categories, CategoryItem{
				Name:        catName,
				Count:       catResult.TotalCount,
				Size:        catResult.TotalSize,
				Selected:    selected,
				SafetyLevel: safetyLevel,
				Recommended: recommended,
				Description: getCategoryDescription(catName),
			})
		}
	}

	// Also add any other categories (with conservative defaults)
	for catName, catResult := range grouped {
		found := false
		for _, cat := range categories {
			if cat.Name == catName {
				found = true
				break
			}
		}
		if !found && catResult.TotalCount > 0 {
			categories = append(categories, CategoryItem{
				Name:        catName,
				Count:       catResult.TotalCount,
				Size:        catResult.TotalSize,
				Selected:    false, // Unknown categories default to unselected
				SafetyLevel: SafetyLow,
				Recommended: false,
				Description: "Unknown category - review carefully",
			})
		}
	}

	// Use default dimensions if not provided
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	return &CategoryViewModel{
		scanResult: scanResult,
		categories: categories,
		cursor:     0,
		width:      width,
		height:     height,
	}
}

// Init initializes the category view
func (m *CategoryViewModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *CategoryViewModel) Update(msg tea.Msg) (*CategoryViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.categories)-1 {
				m.cursor++
			}
		case "g":
			// Jump to top (gg)
			m.cursor = 0
		case "G":
			// Jump to bottom
			if len(m.categories) > 0 {
				m.cursor = len(m.categories) - 1
			}
		case "space", " ":
			// Toggle selection
			if m.cursor >= 0 && m.cursor < len(m.categories) {
				m.categories[m.cursor].Selected = !m.categories[m.cursor].Selected
			}
		case "x":
			// Toggle and move down (quick selection)
			if m.cursor < len(m.categories) {
				m.categories[m.cursor].Selected = !m.categories[m.cursor].Selected
				if m.cursor < len(m.categories)-1 {
					m.cursor++
				}
			}
		case "ctrl+a":
			// Select all
			for i := range m.categories {
				m.categories[i].Selected = true
			}
		case "ctrl+d":
			// Deselect all
			for i := range m.categories {
				m.categories[i].Selected = false
			}
		case "enter":
			// Proceed to file browser
			return m, m.proceedToFileBrowser()
		}
	}

	return m, nil
}

// View renders the category selection view
func (m *CategoryViewModel) View() string {
	var b strings.Builder

	// Show warning if terminal is too small
	if warning := uiutils.GetSizeWarningBanner(m.width, m.height); warning != "" {
		b.WriteString(warning)
	}

	// Title
	b.WriteString(styles.TitleStyle.Render("📦 Select Categories to Clean"))
	b.WriteString("\n\n")

	// Instructions
	helpText := "↑/↓:navigate  space:toggle  x:toggle+down  ctrl+a:all  ctrl+d:none  enter:continue"
	if m.width < 80 {
		helpText = "↑/↓:move  space:toggle  enter:continue"
	}
	b.WriteString(styles.HelpStyle.Render(helpText))
	b.WriteString("\n\n")

	// Category list with enhanced display
	for i, cat := range m.categories {
		cursor := "  "
		if i == m.cursor {
			cursor = styles.SelectedStyle.Render("→ ")
		}

		checkbox := styles.UncheckedBox()
		if cat.Selected {
			checkbox = styles.CheckedBox()
		}

		// Category icon and name with color
		categoryIcon := styles.GetCategoryIcon(cat.Name)
		categoryNameStyle := lipgloss.NewStyle().Foreground(styles.GetCategoryColor(cat.Name)).Bold(true)

		line := fmt.Sprintf("%s%s %s %s",
			cursor,
			checkbox,
			categoryIcon,
			categoryNameStyle.Render(cat.Name),
		)

		// Add safety indicator
		safetyIcon := styles.GetSafetyIcon(cat.SafetyLevel.String())
		safetyStyle := lipgloss.NewStyle().Foreground(styles.GetSafetyColor(cat.SafetyLevel.String()))
		line += " " + safetyStyle.Render(safetyIcon)

		// Add recommended badge
		if cat.Recommended {
			line += " " + styles.RecommendedBadgeStyle.Render("RECOMMENDED")
		}

		// Add count and size with color coding
		sizeColor := styles.GetFileSizeColor(cat.Size)
		sizeStyle := lipgloss.NewStyle().Foreground(sizeColor).Bold(true)
		line += fmt.Sprintf(" (%s files, %s)",
			styles.DimStyle.Render(fmt.Sprintf("%d", cat.Count)),
			sizeStyle.Render(utils.FormatBytes(cat.Size)),
		)

		b.WriteString(line)
		b.WriteString("\n")

		// Add description on hover (when cursor is on this item)
		if i == m.cursor && m.width >= 100 {
			descStyle := lipgloss.NewStyle().Foreground(styles.TextDim).Italic(true).MarginLeft(6)
			b.WriteString(descStyle.Render("↳ " + cat.Description))
			b.WriteString("\n")
		}
	}

	// Summary
	b.WriteString("\n")
	selectedCount := 0
	var selectedSize int64
	for _, cat := range m.categories {
		if cat.Selected {
			selectedCount += cat.Count
			selectedSize += cat.Size
		}
	}

	b.WriteString(styles.SubtitleStyle.Render(fmt.Sprintf("Selected: %d files, %s",
		selectedCount,
		utils.FormatBytes(selectedSize))))
	b.WriteString("\n\n")

	// Status bar
	statusBar := components.NewStatusBar()
	statusBar.SetView("Category Selection")

	// Count selected categories
	selectedCatCount := 0
	for _, cat := range m.categories {
		if cat.Selected {
			selectedCatCount++
		}
	}

	statusBar.SetSelection(selectedCatCount, len(m.categories), selectedSize)
	statusBar.SetShortcuts(map[string]string{
		"↑/↓":   "navigate",
		"space": "toggle",
		"x":     "toggle+down",
		"ctrl+a": "select all",
		"ctrl+d": "deselect all",
		"enter": "continue",
		"q":     "quit",
	})

	b.WriteString(statusBar.Render(m.width))

	return b.String()
}

// proceedToFileBrowser sends a message to proceed to file browser
func (m *CategoryViewModel) proceedToFileBrowser() tea.Cmd {
	var selected []string
	for _, cat := range m.categories {
		if cat.Selected {
			selected = append(selected, cat.Name)
		}
	}

	return func() tea.Msg {
		return CategoriesSelectedMsg{SelectedCategories: selected}
	}
}

// Helper functions

// getCategoryDefaults returns the safety level, recommended status, and default selection for a category
func getCategoryDefaults(category string) (SafetyLevel, bool, bool) {
	switch category {
	case "cache", "temp":
		// Very safe - always recommended and selected by default
		return SafetyHigh, true, true
	case "logs":
		// Safe but review age threshold
		return SafetyHigh, true, true
	case "package_managers":
		// Generally safe but can require reinstall
		return SafetyMedium, false, false
	case "duplicates":
		// Needs careful review to avoid deleting wrong files
		return SafetyMedium, false, false
	case "downloads":
		// Risky - might contain important files
		return SafetyLow, false, false
	default:
		return SafetyLow, false, false
	}
}

// getCategoryDescription returns a user-friendly description for a category
func getCategoryDescription(category string) string {
	descriptions := map[string]string{
		"cache":            "Temporary files that can be safely regenerated",
		"temp":             "Temporary files created by applications",
		"logs":             "Log files older than the configured threshold",
		"duplicates":       "Duplicate files identified by content hash",
		"downloads":        "Old files in your Downloads folder",
		"package_managers": "Cache from package managers (brew, npm, pip, etc.)",
	}

	if desc, ok := descriptions[category]; ok {
		return desc
	}
	return "No description available"
}

// getSafetyLevelString returns a string representation of the safety level
func (s SafetyLevel) String() string {
	switch s {
	case SafetyHigh:
		return "SAFE"
	case SafetyMedium:
		return "CAUTION"
	case SafetyLow:
		return "RISKY"
	default:
		return "UNKNOWN"
	}
}
