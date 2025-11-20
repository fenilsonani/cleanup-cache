package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/fenilsonani/cleanup-cache/internal/ui/styles"
)

// InfoPanel represents a contextual information panel
type InfoPanel struct {
	title   string
	content []InfoItem
	visible bool
	width   int
}

// InfoItem represents a single piece of information
type InfoItem struct {
	Label string
	Value string
	Icon  string
}

// NewInfoPanel creates a new info panel
func NewInfoPanel(title string, width int) *InfoPanel {
	return &InfoPanel{
		title:   title,
		content: []InfoItem{},
		visible: false,
		width:   width,
	}
}

// AddItem adds an information item to the panel
func (p *InfoPanel) AddItem(label, value, icon string) {
	p.content = append(p.content, InfoItem{
		Label: label,
		Value: value,
		Icon:  icon,
	})
}

// SetVisible sets the visibility of the panel
func (p *InfoPanel) SetVisible(visible bool) {
	p.visible = visible
}

// IsVisible returns whether the panel is visible
func (p *InfoPanel) IsVisible() bool {
	return p.visible
}

// Toggle toggles the visibility of the panel
func (p *InfoPanel) Toggle() {
	p.visible = !p.visible
}

// Clear clears all content from the panel
func (p *InfoPanel) Clear() {
	p.content = []InfoItem{}
}

// SetWidth sets the width of the panel
func (p *InfoPanel) SetWidth(width int) {
	p.width = width
}

// Render renders the info panel
func (p *InfoPanel) Render() string {
	if !p.visible || len(p.content) == 0 {
		return ""
	}

	var b strings.Builder

	// Calculate panel width (use half of terminal width, min 40)
	panelWidth := p.width / 2
	if panelWidth < 40 {
		panelWidth = 40
	}
	if panelWidth > 80 {
		panelWidth = 80
	}

	// Create panel style
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(styles.FocusBorder).
		Padding(1, 2).
		Width(panelWidth).
		Background(styles.BgDark)

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true).
		Underline(true)

	var content strings.Builder
	content.WriteString(titleStyle.Render(p.title))
	content.WriteString("\n\n")

	// Content items
	for i, item := range p.content {
		// Icon and label
		labelStyle := lipgloss.NewStyle().
			Foreground(styles.Secondary).
			Bold(true)

		if item.Icon != "" {
			content.WriteString(item.Icon + " ")
		}
		content.WriteString(labelStyle.Render(item.Label) + ": ")

		// Value
		valueStyle := lipgloss.NewStyle().
			Foreground(styles.Text)

		content.WriteString(valueStyle.Render(item.Value))

		if i < len(p.content)-1 {
			content.WriteString("\n")
		}
	}

	// Footer hint
	content.WriteString("\n\n")
	footerStyle := lipgloss.NewStyle().
		Foreground(styles.TextDim).
		Italic(true)
	content.WriteString(footerStyle.Render("Press 'i' or 'esc' to close"))

	b.WriteString(panelStyle.Render(content.String()))

	return b.String()
}

// RenderAsOverlay renders the panel as an overlay (centered on screen)
func (p *InfoPanel) RenderAsOverlay(terminalWidth, terminalHeight int) string {
	if !p.visible || len(p.content) == 0 {
		return ""
	}

	panelContent := p.Render()

	// Calculate positioning to center the panel
	lines := strings.Split(panelContent, "\n")
	panelHeight := len(lines)
	panelWidth := 0
	for _, line := range lines {
		if len(line) > panelWidth {
			panelWidth = len(line)
		}
	}

	// Center vertically and horizontally
	topPadding := (terminalHeight - panelHeight) / 2
	if topPadding < 0 {
		topPadding = 0
	}

	leftPadding := (terminalWidth - panelWidth) / 2
	if leftPadding < 0 {
		leftPadding = 0
	}

	var b strings.Builder

	// Add top padding
	for i := 0; i < topPadding; i++ {
		b.WriteString("\n")
	}

	// Add left padding to each line
	for _, line := range lines {
		b.WriteString(strings.Repeat(" ", leftPadding))
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// CategoryInfoPanel creates an info panel for a category
func CategoryInfoPanel(categoryName string, count int, size int64, safetyLevel string, width int) *InfoPanel {
	panel := NewInfoPanel("Category Information", width)

	panel.AddItem("Category", categoryName, styles.GetCategoryIcon(categoryName))
	panel.AddItem("Files", fmt.Sprintf("%d", count), "📊")
	panel.AddItem("Total Size", formatBytes(size), "💾")
	panel.AddItem("Safety Level", safetyLevel, styles.GetSafetyIcon(safetyLevel))

	// Add category-specific information
	description := getCategoryDescription(categoryName)
	panel.AddItem("Description", description, "📝")

	return panel
}

// FileInfoPanel creates an info panel for a file
func FileInfoPanel(filePath string, size int64, modTime string, category string, width int) *InfoPanel {
	panel := NewInfoPanel("File Information", width)

	// Truncate path if too long
	displayPath := filePath
	if len(filePath) > 60 {
		displayPath = filePath[:30] + "..." + filePath[len(filePath)-30:]
	}

	panel.AddItem("Path", displayPath, "📁")
	panel.AddItem("Size", formatBytes(size), "💾")
	panel.AddItem("Modified", modTime, "🕒")
	panel.AddItem("Category", category, styles.GetCategoryIcon(category))

	// Add file type information
	fileType := getFileType(filePath)
	panel.AddItem("Type", fileType, "📄")

	return panel
}

// Helper functions

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGT"[exp])
}

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

func getFileType(path string) string {
	// Simple file type detection based on extension
	if strings.HasSuffix(path, ".log") {
		return "Log File"
	} else if strings.HasSuffix(path, ".cache") || strings.HasSuffix(path, ".tmp") {
		return "Cache/Temp File"
	} else if strings.Contains(path, "/cache/") || strings.Contains(path, "/Cache/") {
		return "Cache File"
	} else if strings.Contains(path, "/logs/") || strings.Contains(path, "/Logs/") {
		return "Log File"
	}
	return "File"
}
