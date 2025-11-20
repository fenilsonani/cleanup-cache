package styles

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Theme colors - Base palette
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

// Extended color palette for enhanced UI
var (
	// Gradient colors for depth
	PrimaryLight   = lipgloss.Color("#9F7AEA")
	PrimaryDark    = lipgloss.Color("#6B21A8")
	SuccessLight   = lipgloss.Color("#34D399")
	SuccessDark    = lipgloss.Color("#059669")
	WarningLight   = lipgloss.Color("#FCD34D")
	WarningDark    = lipgloss.Color("#D97706")
	DangerLight    = lipgloss.Color("#F87171")
	DangerDark     = lipgloss.Color("#DC2626")

	// Interactive states
	ActiveBg    = lipgloss.Color("#312E81")
	HoverBg     = lipgloss.Color("#4C1D95")
	FocusBorder = lipgloss.Color("#8B5CF6")
	SelectedBg  = lipgloss.Color("#2E1065")

	// File size color coding
	FileSizeTiny   = lipgloss.Color("#6EE7B7") // < 1MB - green
	FileSizeSmall  = lipgloss.Color("#60A5FA") // < 100MB - blue
	FileSizeMedium = lipgloss.Color("#FBBF24") // < 1GB - yellow
	FileSizeLarge  = lipgloss.Color("#FB923C") // < 10GB - orange
	FileSizeHuge   = lipgloss.Color("#F87171") // >= 10GB - red

	// Safety levels
	SafetyHigh   = lipgloss.Color("#10B981") // Green - safe to delete
	SafetyMedium = lipgloss.Color("#F59E0B") // Yellow - review carefully
	SafetyLow    = lipgloss.Color("#EF4444") // Red - risky

	// Category icons colors
	CacheColor    = lipgloss.Color("#60A5FA")
	TempColor     = lipgloss.Color("#34D399")
	LogsColor     = lipgloss.Color("#A78BFA")
	DuplicateColor = lipgloss.Color("#FB923C")
	DownloadsColor = lipgloss.Color("#FBBF24")
	PackagesColor  = lipgloss.Color("#F472B6")
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

	// New enhanced styles
	ActiveRowStyle = lipgloss.NewStyle().
			Background(ActiveBg).
			Foreground(Text).
			Bold(true)

	SelectedRowStyle = lipgloss.NewStyle().
			Background(SelectedBg).
			Foreground(Text)

	HoverRowStyle = lipgloss.NewStyle().
			Background(HoverBg).
			Foreground(Text)

	ZebraRowStyle = lipgloss.NewStyle().
			Background(BgDark)

	BadgeStyle = lipgloss.NewStyle().
			Foreground(Text).
			Background(Primary).
			Padding(0, 1).
			Bold(true)

	RecommendedBadgeStyle = lipgloss.NewStyle().
			Foreground(Text).
			Background(Success).
			Padding(0, 1).
			Bold(true)

	RiskyBadgeStyle = lipgloss.NewStyle().
			Foreground(Text).
			Background(Danger).
			Padding(0, 1).
			Bold(true)

	ModalStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(FocusBorder).
			Padding(1, 2).
			Background(BgDark)

	TooltipStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Info).
			Padding(0, 1).
			Foreground(TextDim).
			Background(BgLight)

	SkeletonStyle = lipgloss.NewStyle().
			Foreground(Muted)
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

// GetFileSizeColor returns the appropriate color for a file size
func GetFileSizeColor(size int64) lipgloss.Color {
	const (
		MB = 1024 * 1024
		GB = 1024 * MB
	)

	switch {
	case size < MB:
		return FileSizeTiny // < 1MB - green
	case size < 100*MB:
		return FileSizeSmall // < 100MB - blue
	case size < GB:
		return FileSizeMedium // < 1GB - yellow
	case size < 10*GB:
		return FileSizeLarge // < 10GB - orange
	default:
		return FileSizeHuge // >= 10GB - red
	}
}

// GetFileSizeStyle returns a styled file size string with color coding
func GetFileSizeStyle(size int64) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(GetFileSizeColor(size)).Bold(true)
}

// GetSafetyColor returns the color for a safety level
func GetSafetyColor(level string) lipgloss.Color {
	switch level {
	case "HIGH", "SAFE":
		return SafetyHigh
	case "MEDIUM", "CAUTION":
		return SafetyMedium
	case "LOW", "RISKY", "DANGEROUS":
		return SafetyLow
	default:
		return Muted
	}
}

// GetSafetyIcon returns an emoji icon for a safety level
func GetSafetyIcon(level string) string {
	switch level {
	case "HIGH", "SAFE":
		return "🟢"
	case "MEDIUM", "CAUTION":
		return "🟡"
	case "LOW", "RISKY", "DANGEROUS":
		return "🔴"
	default:
		return "⚪"
	}
}

// GetCategoryIcon returns an emoji icon for a category
func GetCategoryIcon(category string) string {
	switch category {
	case "cache":
		return "📦"
	case "temp":
		return "🗂️"
	case "logs":
		return "📝"
	case "duplicates":
		return "📑"
	case "downloads":
		return "📥"
	case "package_managers":
		return "📦"
	default:
		return "📁"
	}
}

// GetCategoryColor returns the color for a category
func GetCategoryColor(category string) lipgloss.Color {
	switch category {
	case "cache":
		return CacheColor
	case "temp":
		return TempColor
	case "logs":
		return LogsColor
	case "duplicates":
		return DuplicateColor
	case "downloads":
		return DownloadsColor
	case "package_managers":
		return PackagesColor
	default:
		return Secondary
	}
}

// RenderFileSize formats and colors a file size
func RenderFileSize(size int64) string {
	// Use the existing FormatBytes from utils package
	// This is a placeholder - will integrate with existing formatter
	return GetFileSizeStyle(size).Render(formatBytes(size))
}

// formatBytes is a helper (will use utils.FormatBytes in actual code)
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

// EmptyStateBox creates a styled box for empty states
func EmptyStateBox(message, suggestion string) string {
	emptyBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Muted).
		Padding(2, 4).
		Align(lipgloss.Center)

	content := DimStyle.Render(message) + "\n\n" +
		HelpStyle.Render(suggestion)

	return emptyBox.Render(content)
}
