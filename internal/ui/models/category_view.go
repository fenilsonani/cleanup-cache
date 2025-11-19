package models

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fenilsonani/cleanup-cache/internal/scanner"
	"github.com/fenilsonani/cleanup-cache/internal/ui/styles"
	"github.com/fenilsonani/cleanup-cache/pkg/utils"
)

// CategoryItem represents a selectable category
type CategoryItem struct {
	Name     string
	Count    int
	Size     int64
	Selected bool
}

// CategoryViewModel handles category selection
type CategoryViewModel struct {
	scanResult *scanner.ScanResult
	categories []CategoryItem
	cursor     int
}

// NewCategoryViewModel creates a new category view model
func NewCategoryViewModel(scanResult *scanner.ScanResult) *CategoryViewModel {
	// Group scan results by category
	grouped := scanResult.GroupByCategory()

	// Create category items
	var categories []CategoryItem
	for _, catName := range []string{"cache", "temp", "logs", "duplicates", "downloads", "package_managers"} {
		if catResult, ok := grouped[catName]; ok && catResult.TotalCount > 0 {
			categories = append(categories, CategoryItem{
				Name:     catName,
				Count:    catResult.TotalCount,
				Size:     catResult.TotalSize,
				Selected: true, // Default to selected
			})
		}
	}

	// Also add any other categories
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
				Name:     catName,
				Count:    catResult.TotalCount,
				Size:     catResult.TotalSize,
				Selected: true,
			})
		}
	}

	return &CategoryViewModel{
		scanResult: scanResult,
		categories: categories,
		cursor:     0,
	}
}

// Init initializes the category view
func (m *CategoryViewModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *CategoryViewModel) Update(msg tea.Msg) (*CategoryViewModel, tea.Cmd) {
	switch msg := msg.(type) {
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
		case "space":
			// Toggle selection
			if m.cursor < len(m.categories) {
				m.categories[m.cursor].Selected = !m.categories[m.cursor].Selected
			}
		case "a":
			// Select all
			for i := range m.categories {
				m.categories[i].Selected = true
			}
		case "d":
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

	// Title
	b.WriteString(styles.TitleStyle.Render("📦 Select Categories to Clean"))
	b.WriteString("\n\n")

	// Instructions
	b.WriteString(styles.HelpStyle.Render("Use ↑/↓ to navigate, space to toggle, enter to continue"))
	b.WriteString("\n\n")

	// Category list
	for i, cat := range m.categories {
		cursor := "  "
		if i == m.cursor {
			cursor = styles.SelectedStyle.Render("→ ")
		}

		checkbox := styles.UncheckedBox()
		if cat.Selected {
			checkbox = styles.CheckedBox()
		}

		line := fmt.Sprintf("%s%s %s",
			cursor,
			checkbox,
			styles.CategoryStyle.Render(cat.Name),
		)

		// Add count and size
		line += fmt.Sprintf(" (%s files, %s)",
			styles.DimStyle.Render(fmt.Sprintf("%d", cat.Count)),
			styles.FileSizeStyle.Render(utils.FormatBytes(cat.Size)),
		)

		b.WriteString(line)
		b.WriteString("\n")
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

	// Shortcuts
	b.WriteString(styles.HelpStyle.Render("Shortcuts: [a] select all  [d] deselect all  [enter] continue  [q] quit"))

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
