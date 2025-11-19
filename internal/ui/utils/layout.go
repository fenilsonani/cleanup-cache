package utils

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fenilsonani/cleanup-cache/internal/ui/styles"
)

const (
	// MinTerminalWidth is the minimum recommended terminal width
	MinTerminalWidth = 80
	// MinTerminalHeight is the minimum recommended terminal height
	MinTerminalHeight = 24
)

// TruncatePath intelligently truncates a file path to fit within maxWidth
// It preserves the beginning and end of the path, truncating the middle
func TruncatePath(path string, maxWidth int) string {
	if len(path) <= maxWidth {
		return path
	}

	if maxWidth < 10 {
		// Too small to show anything meaningful
		return "..."
	}

	// Split into directory and filename
	dir, file := filepath.Split(path)

	// If filename alone is too long, truncate it
	if len(file) > maxWidth-4 {
		return "..." + file[len(file)-(maxWidth-4):]
	}

	// Calculate available space for directory
	availableForDir := maxWidth - len(file) - 3 // 3 for "..."

	if availableForDir <= 0 {
		return "..." + file
	}

	// Clean up directory path
	dir = filepath.Clean(dir)
	if len(dir) <= availableForDir {
		return filepath.Join(dir, file)
	}

	// Truncate directory from the middle
	// Show start and end: /very/long/.../end/
	if availableForDir < 10 {
		return ".../" + file
	}

	// Split directory into parts
	parts := strings.Split(dir, string(filepath.Separator))
	if len(parts) <= 2 {
		// Just truncate from start
		truncatedDir := "..." + dir[len(dir)-availableForDir:]
		return truncatedDir + string(filepath.Separator) + file
	}

	// Show first part, "...", and last part
	firstPart := parts[0]
	if firstPart == "" && len(parts) > 1 {
		firstPart = string(filepath.Separator) + parts[1]
	}

	lastPart := parts[len(parts)-1]
	middlePart := "..."

	// Try to fit: firstPart/.../ lastPart/file
	estimatedLen := len(firstPart) + 1 + len(middlePart) + 1 + len(lastPart)
	if estimatedLen <= availableForDir {
		truncatedDir := firstPart + string(filepath.Separator) + middlePart + string(filepath.Separator) + lastPart
		return truncatedDir + string(filepath.Separator) + file
	}

	// If still too long, just show ...lastPart/file
	return "..." + string(filepath.Separator) + lastPart + string(filepath.Separator) + file
}

// CalculatePageSize calculates the number of items that can fit on a page
// given the terminal height and reserved space for headers/footers
func CalculatePageSize(terminalHeight int) int {
	// Reserve space for:
	// - Title (2 lines)
	// - Header/instructions (2-3 lines)
	// - Footer/help (2-3 lines)
	// - Status/summary (2-3 lines)
	// Total reserved: ~10 lines
	const reservedLines = 10

	pageSize := terminalHeight - reservedLines
	if pageSize < 5 {
		pageSize = 5 // Minimum page size
	}

	return pageSize
}

// IsTerminalTooSmall checks if the terminal is below minimum recommended size
func IsTerminalTooSmall(width, height int) bool {
	return width < MinTerminalWidth || height < MinTerminalHeight
}

// GetSizeWarningBanner returns a warning banner if terminal is too small
func GetSizeWarningBanner(width, height int) string {
	if !IsTerminalTooSmall(width, height) {
		return ""
	}

	warning := "⚠️  Terminal too small! Recommended: 80x24 or larger"
	if width > 0 && height > 0 {
		warning += styles.DimStyle.Render(" (current: ") +
			styles.WarningStyle.Render(fmt.Sprintf("%dx%d", width, height)) +
			styles.DimStyle.Render(")")
	}

	return styles.WarningStyle.Render(warning) + "\n\n"
}

// TruncateString truncates a string to maxLen, adding ellipsis if needed
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return "..."
	}
	return s[:maxLen-3] + "..."
}

// TruncateMiddle truncates a string from the middle, preserving start and end
func TruncateMiddle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	if maxLen < 10 {
		return TruncateString(s, maxLen)
	}

	// Show equal parts from start and end
	sideLen := (maxLen - 3) / 2
	return s[:sideLen] + "..." + s[len(s)-sideLen:]
}

// AdjustContentHeight limits content to fit within available height
// Returns the number of lines that can be displayed
func AdjustContentHeight(totalLines, terminalHeight, reservedLines int) int {
	availableLines := terminalHeight - reservedLines
	if availableLines < 1 {
		availableLines = 1
	}

	if totalLines <= availableLines {
		return totalLines
	}

	return availableLines
}
