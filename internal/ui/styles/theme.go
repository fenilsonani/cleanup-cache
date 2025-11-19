package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme colors
var (
	Primary   = lipgloss.Color("#7C3AED")
	Secondary = lipgloss.Color("#A78BFA")
	Success   = lipgloss.Color("#10B981")
	Warning   = lipgloss.Color("#F59E0B")
	Danger    = lipgloss.Color("#EF4444")
	Info      = lipgloss.Color("#3B82F6")
	Muted     = lipgloss.Color("#6B7280")
	Text      = lipgloss.Color("#F3F4F6")
	TextDim   = lipgloss.Color("#9CA3AF")
	Border    = lipgloss.Color("#4B5563")
	BgDark    = lipgloss.Color("#1F2937")
	BgLight   = lipgloss.Color("#374151")
)

// Common styles
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(Secondary).
			MarginBottom(1)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Border).
			Padding(1, 2)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true)

	CheckboxStyle = lipgloss.NewStyle().
			Foreground(Success)

	CheckboxUncheckedStyle = lipgloss.NewStyle().
				Foreground(Muted)

	FilePathStyle = lipgloss.NewStyle().
			Foreground(Info)

	FileSizeStyle = lipgloss.NewStyle().
			Foreground(Warning)

	CategoryStyle = lipgloss.NewStyle().
			Foreground(Secondary).
			Italic(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Danger).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success).
			Bold(true)

	HelpStyle = lipgloss.NewStyle().
			Foreground(TextDim).
			Italic(true)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(Text).
			Background(BgDark).
			Padding(0, 1)

	HighlightStyle = lipgloss.NewStyle().
			Foreground(Text).
			Background(Primary).
			Bold(true)

	DimStyle = lipgloss.NewStyle().
			Foreground(TextDim)

	BoldStyle = lipgloss.NewStyle().
			Bold(true)

	InfoStyle = lipgloss.NewStyle().
			Foreground(Info).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning).
			Bold(true)
)

// Helper functions
func CheckedBox() string {
	return CheckboxStyle.Render("☑")
}

func UncheckedBox() string {
	return CheckboxUncheckedStyle.Render("☐")
}

func ProgressBar(current, total int, width int) string {
	if total == 0 {
		return ""
	}

	percent := float64(current) / float64(total)
	filled := int(percent * float64(width))

	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	style := lipgloss.NewStyle().Foreground(Primary)
	return style.Render(bar)
}
