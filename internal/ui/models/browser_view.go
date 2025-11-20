package models

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fenilsonani/cleanup-cache/internal/scanner"
	"github.com/fenilsonani/cleanup-cache/internal/ui/components"
	"github.com/fenilsonani/cleanup-cache/internal/ui/styles"
	uiutils "github.com/fenilsonani/cleanup-cache/internal/ui/utils"
	"github.com/fenilsonani/cleanup-cache/pkg/utils"
)

// SortBy defines the sort field
type SortBy int

const (
	SortByName SortBy = iota
	SortBySize
	SortByDate
	SortByCategory
)

// BulkAction defines bulk operation types
type BulkAction int

const (
	BulkActionNone BulkAction = iota
	BulkActionSelectBySize
	BulkActionSelectByAge
	BulkActionSelectByPattern
	BulkActionInvertSelection
	BulkActionClearSelection
)

// BrowserViewModel handles file browsing and selection
type BrowserViewModel struct {
	files           []scanner.FileInfo
	filteredFiles   []scanner.FileInfo // filtered view of files
	selectedIndices map[int]bool
	cursor          int
	offset          int
	pageSize        int
	width           int
	height          int

	// Sorting
	sortBy   SortBy
	sortDesc bool

	// Filtering
	filterMode   bool
	filterText   string
	filterActive bool

	// Visual mode (range selection)
	visualMode     bool
	visualStartIdx int

	// Bulk operations
	bulkMenuOpen   bool
	bulkMenuCursor int
	bulkMenuItems  []string

	// Input mode for bulk operations
	inputMode      bool
	inputPrompt    string
	inputValue     string
	inputAction    BulkAction

	// 'gg' tracking
	lastKeyWasG bool
}

// NewBrowserViewModel creates a new browser view model
func NewBrowserViewModel(scanResult *scanner.ScanResult, selectedCategories []string, width, height int) *BrowserViewModel {
	// Filter files by selected categories
	var filteredFiles []scanner.FileInfo
	categoryMap := make(map[string]bool)
	for _, cat := range selectedCategories {
		categoryMap[cat] = true
	}

	for _, file := range scanResult.Files {
		if categoryMap[file.Category] {
			filteredFiles = append(filteredFiles, file)
		}
	}

	// Use default dimensions if not provided
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	// Calculate page size based on terminal height
	pageSize := uiutils.CalculatePageSize(height)

	m := &BrowserViewModel{
		files:           filteredFiles,
		filteredFiles:   filteredFiles,
		selectedIndices: make(map[int]bool),
		cursor:          0,
		offset:          0,
		pageSize:        pageSize,
		width:           width,
		height:          height,
		sortBy:          SortBySize, // Default sort by size
		sortDesc:        true,        // Descending by default
		bulkMenuItems: []string{
			"Select files > threshold size",
			"Select files older than N days",
			"Select by pattern (glob)",
			"Invert selection",
			"Clear all selection",
		},
	}

	// Sort initially
	m.sortFiles()
	m.applyFilter()

	return m
}

// Init initializes the browser view
func (m *BrowserViewModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *BrowserViewModel) Update(msg tea.Msg) (*BrowserViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Recalculate page size
		m.pageSize = uiutils.CalculatePageSize(msg.Height)

	case tea.KeyMsg:
		// Handle input mode (for bulk operations)
		if m.inputMode {
			return m.handleInputMode(msg)
		}

		// Handle bulk menu
		if m.bulkMenuOpen {
			return m.handleBulkMenu(msg)
		}

		// Handle filter mode
		if m.filterMode {
			return m.handleFilterMode(msg)
		}

		// Handle visual mode
		if m.visualMode {
			return m.handleVisualMode(msg)
		}

		// Normal mode
		return m.handleNormalMode(msg)
	}

	return m, nil
}

// handleNormalMode handles keys in normal mode
func (m *BrowserViewModel) handleNormalMode(msg tea.KeyMsg) (*BrowserViewModel, tea.Cmd) {
	key := msg.String()

	// Handle 'gg' for jump to top
	if m.lastKeyWasG && key == "g" {
		m.cursor = 0
		m.offset = 0
		m.lastKeyWasG = false
		return m, nil
	}

	// Reset 'gg' tracking
	if key != "g" {
		m.lastKeyWasG = false
	}

	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset--
			}
		}

	case "down", "j":
		if m.cursor < len(m.filteredFiles)-1 {
			m.cursor++
			if m.cursor >= m.offset+m.pageSize {
				m.offset++
			}
		}

	case "g":
		// Track first 'g' for 'gg'
		m.lastKeyWasG = true

	case "G", "shift+g":
		// Jump to last item
		m.cursor = len(m.filteredFiles) - 1
		m.offset = max(0, m.cursor-m.pageSize+1)

	case "ctrl+f", "pgdown":
		// Page down
		m.cursor = min(m.cursor+m.pageSize, len(m.filteredFiles)-1)
		m.offset = max(0, min(m.cursor, len(m.filteredFiles)-m.pageSize))

	case "ctrl+b", "pgup":
		// Page up
		m.cursor = max(m.cursor-m.pageSize, 0)
		m.offset = max(0, m.cursor)

	case "space":
		m.toggleSelection(m.cursor)

	case "x":
		// Toggle and move down
		m.toggleSelection(m.cursor)
		if m.cursor < len(m.filteredFiles)-1 {
			m.cursor++
			if m.cursor >= m.offset+m.pageSize {
				m.offset++
			}
		}

	case "s":
		// Cycle sort mode
		m.cycleSortMode()

	case "S", "shift+s":
		// Reverse sort order
		m.sortDesc = !m.sortDesc
		m.sortFiles()
		m.applyFilter()

	case "/":
		// Enter filter mode
		m.filterMode = true
		m.filterText = ""

	case "b":
		// Open bulk operations menu
		m.bulkMenuOpen = true
		m.bulkMenuCursor = 0

	case "v":
		// Enter visual mode
		m.visualMode = true
		m.visualStartIdx = m.cursor

	case "ctrl+a":
		// Select all visible files
		for i := range m.filteredFiles {
			m.selectedIndices[m.getOriginalIndex(i)] = true
		}

	case "ctrl+d":
		// Deselect all
		m.selectedIndices = make(map[int]bool)

	case "enter":
		return m, m.proceedToConfirmation()
	}

	return m, nil
}

// handleFilterMode handles keys in filter mode
func (m *BrowserViewModel) handleFilterMode(msg tea.KeyMsg) (*BrowserViewModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Clear filter and exit
		m.filterMode = false
		m.filterText = ""
		m.filterActive = false
		m.applyFilter()
		m.cursor = 0
		m.offset = 0

	case "enter":
		// Apply filter and exit
		m.filterMode = false
		m.filterActive = m.filterText != ""
		m.applyFilter()
		m.cursor = 0
		m.offset = 0

	case "backspace":
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
			m.filterActive = m.filterText != ""
			m.applyFilter()
		}

	default:
		// Add character to filter
		if len(msg.String()) == 1 {
			m.filterText += msg.String()
			m.filterActive = true
			m.applyFilter()
		}
	}

	return m, nil
}

// handleVisualMode handles keys in visual mode
func (m *BrowserViewModel) handleVisualMode(msg tea.KeyMsg) (*BrowserViewModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Exit visual mode
		m.visualMode = false

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset--
			}
		}

	case "down", "j":
		if m.cursor < len(m.filteredFiles)-1 {
			m.cursor++
			if m.cursor >= m.offset+m.pageSize {
				m.offset++
			}
		}

	case "space", "enter":
		// Toggle entire range
		start := min(m.visualStartIdx, m.cursor)
		end := max(m.visualStartIdx, m.cursor)

		// Check if all selected, if so deselect, otherwise select
		allSelected := true
		for i := start; i <= end; i++ {
			origIdx := m.getOriginalIndex(i)
			if !m.selectedIndices[origIdx] {
				allSelected = false
				break
			}
		}

		for i := start; i <= end; i++ {
			origIdx := m.getOriginalIndex(i)
			m.selectedIndices[origIdx] = !allSelected
		}

		m.visualMode = false
	}

	return m, nil
}

// handleBulkMenu handles keys in bulk menu
func (m *BrowserViewModel) handleBulkMenu(msg tea.KeyMsg) (*BrowserViewModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.bulkMenuOpen = false

	case "up", "k":
		if m.bulkMenuCursor > 0 {
			m.bulkMenuCursor--
		}

	case "down", "j":
		if m.bulkMenuCursor < len(m.bulkMenuItems)-1 {
			m.bulkMenuCursor++
		}

	case "enter":
		return m.executeBulkAction()
	}

	return m, nil
}

// handleInputMode handles keys in input mode
func (m *BrowserViewModel) handleInputMode(msg tea.KeyMsg) (*BrowserViewModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inputMode = false
		m.inputValue = ""

	case "enter":
		m.inputMode = false
		m.applyInputAction()

	case "backspace":
		if len(m.inputValue) > 0 {
			m.inputValue = m.inputValue[:len(m.inputValue)-1]
		}

	default:
		// Add character to input
		if len(msg.String()) == 1 {
			m.inputValue += msg.String()
		}
	}

	return m, nil
}

// executeBulkAction executes the selected bulk action
func (m *BrowserViewModel) executeBulkAction() (*BrowserViewModel, tea.Cmd) {
	m.bulkMenuOpen = false

	switch m.bulkMenuCursor {
	case 0: // Select files > threshold size
		m.inputMode = true
		m.inputPrompt = "Enter size threshold (e.g., 100MB): "
		m.inputValue = ""
		m.inputAction = BulkActionSelectBySize

	case 1: // Select files older than N days
		m.inputMode = true
		m.inputPrompt = "Enter days (e.g., 30): "
		m.inputValue = ""
		m.inputAction = BulkActionSelectByAge

	case 2: // Select by pattern
		m.inputMode = true
		m.inputPrompt = "Enter glob pattern (e.g., *.log): "
		m.inputValue = ""
		m.inputAction = BulkActionSelectByPattern

	case 3: // Invert selection
		for i := range m.filteredFiles {
			origIdx := m.getOriginalIndex(i)
			m.selectedIndices[origIdx] = !m.selectedIndices[origIdx]
		}

	case 4: // Clear all selection
		m.selectedIndices = make(map[int]bool)
	}

	return m, nil
}

// applyInputAction applies the input action
func (m *BrowserViewModel) applyInputAction() {
	switch m.inputAction {
	case BulkActionSelectBySize:
		// Parse size
		size, err := utils.ParseSize(m.inputValue)
		if err == nil {
			for i, file := range m.filteredFiles {
				if file.Size >= size {
					origIdx := m.getOriginalIndex(i)
					m.selectedIndices[origIdx] = true
				}
			}
		}

	case BulkActionSelectByAge:
		// Parse days
		var days int
		fmt.Sscanf(m.inputValue, "%d", &days)
		if days > 0 {
			threshold := time.Now().AddDate(0, 0, -days)
			for i, file := range m.filteredFiles {
				if file.ModTime.Before(threshold) {
					origIdx := m.getOriginalIndex(i)
					m.selectedIndices[origIdx] = true
				}
			}
		}

	case BulkActionSelectByPattern:
		// Match pattern
		pattern := m.inputValue
		for i, file := range m.filteredFiles {
			matched, _ := filepath.Match(pattern, filepath.Base(file.Path))
			if matched {
				origIdx := m.getOriginalIndex(i)
				m.selectedIndices[origIdx] = true
			}
		}
	}

	m.inputValue = ""
}

// cycleSortMode cycles through sort modes
func (m *BrowserViewModel) cycleSortMode() {
	switch m.sortBy {
	case SortByName:
		m.sortBy = SortBySize
	case SortBySize:
		m.sortBy = SortByDate
	case SortByDate:
		m.sortBy = SortByCategory
	case SortByCategory:
		m.sortBy = SortByName
	}
	m.sortFiles()
	m.applyFilter()
}

// sortFiles sorts the files based on current sort settings
func (m *BrowserViewModel) sortFiles() {
	sort.Slice(m.files, func(i, j int) bool {
		var less bool
		switch m.sortBy {
		case SortByName:
			less = strings.ToLower(filepath.Base(m.files[i].Path)) < strings.ToLower(filepath.Base(m.files[j].Path))
		case SortBySize:
			less = m.files[i].Size < m.files[j].Size
		case SortByDate:
			less = m.files[i].ModTime.Before(m.files[j].ModTime)
		case SortByCategory:
			less = m.files[i].Category < m.files[j].Category
		}

		if m.sortDesc {
			return !less
		}
		return less
	})
}

// applyFilter applies the filter to files
func (m *BrowserViewModel) applyFilter() {
	if m.filterText == "" {
		m.filteredFiles = m.files
		return
	}

	m.filteredFiles = []scanner.FileInfo{}
	filterLower := strings.ToLower(m.filterText)

	for _, file := range m.files {
		if m.fuzzyMatch(filepath.Base(file.Path), filterLower) {
			m.filteredFiles = append(m.filteredFiles, file)
		}
	}
}

// fuzzyMatch performs fuzzy matching
func (m *BrowserViewModel) fuzzyMatch(text, pattern string) bool {
	textLower := strings.ToLower(text)
	patternIdx := 0

	for _, ch := range textLower {
		if patternIdx < len(pattern) && ch == rune(pattern[patternIdx]) {
			patternIdx++
		}
	}

	return patternIdx == len(pattern)
}

// toggleSelection toggles selection for an index in filtered view
func (m *BrowserViewModel) toggleSelection(filteredIdx int) {
	if filteredIdx < 0 || filteredIdx >= len(m.filteredFiles) {
		return
	}
	origIdx := m.getOriginalIndex(filteredIdx)
	m.selectedIndices[origIdx] = !m.selectedIndices[origIdx]
}

// getOriginalIndex returns the original index in m.files for a filtered index
func (m *BrowserViewModel) getOriginalIndex(filteredIdx int) int {
	if filteredIdx < 0 || filteredIdx >= len(m.filteredFiles) {
		return -1
	}

	targetFile := m.filteredFiles[filteredIdx]
	for i, file := range m.files {
		if file.Path == targetFile.Path {
			return i
		}
	}
	return -1
}

// isSelected checks if a filtered index is selected
func (m *BrowserViewModel) isSelected(filteredIdx int) bool {
	origIdx := m.getOriginalIndex(filteredIdx)
	return m.selectedIndices[origIdx]
}

// View renders the browser view
func (m *BrowserViewModel) View() string {
	var b strings.Builder

	// Show warning if terminal is too small
	if warning := uiutils.GetSizeWarningBanner(m.width, m.height); warning != "" {
		b.WriteString(warning)
	}

	// Title
	b.WriteString(styles.TitleStyle.Render("📁 Select Files to Delete"))
	b.WriteString("\n\n")

	// Column headers
	b.WriteString(m.renderTableHeader())
	b.WriteString("\n")

	// Separator - adjust to terminal width
	separatorWidth := m.width
	if separatorWidth > 100 {
		separatorWidth = 100
	}
	if separatorWidth < 40 {
		separatorWidth = 40
	}
	headerStyle := lipgloss.NewStyle().Foreground(styles.Border)
	b.WriteString(headerStyle.Render(strings.Repeat("─", separatorWidth)))
	b.WriteString("\n")

	// File list
	end := m.offset + m.pageSize
	if end > len(m.filteredFiles) {
		end = len(m.filteredFiles)
	}

	for i := m.offset; i < end; i++ {
		b.WriteString(m.renderFileRow(i))
		b.WriteString("\n")
	}

	// Empty state
	if len(m.filteredFiles) == 0 {
		message := "No files found"
		suggestion := "Try adjusting your filter or selecting different categories"
		if m.filterActive {
			message = "No files match your filter"
			suggestion = "Try a different search term or clear the filter with ESC"
		}
		b.WriteString(styles.EmptyStateBox(message, suggestion))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Pagination info
	b.WriteString(m.renderPagination())
	b.WriteString("\n")

	// Summary
	b.WriteString(m.renderSummary())
	b.WriteString("\n\n")

	// Filter status
	if m.filterActive || m.filterMode {
		b.WriteString(m.renderFilterStatus())
		b.WriteString("\n")
	}

	// Status bar (only in normal mode)
	if !m.bulkMenuOpen && !m.inputMode {
		var selectedCount int
		var selectedSize int64

		for idx, isSelected := range m.selectedIndices {
			if isSelected && idx < len(m.files) {
				selectedCount++
				selectedSize += m.files[idx].Size
			}
		}

		statusBar := components.NewStatusBar()

		// Set workflow breadcrumbs
		workflowSteps := []string{"Scan", "Categories", "Files", "Confirm", "Clean"}
		statusBar.SetWorkflowStep(3, len(workflowSteps), workflowSteps)

		statusBar.SetSelection(selectedCount, len(m.files), selectedSize)

		// Show mode and shortcuts
		shortcuts := map[string]string{
			"↑/↓":   "nav",
			"space": "toggle",
			"s":     "sort",
			"/":     "filter",
			"enter": "next",
			"?":     "help",
		}

		if m.filterMode {
			shortcuts = map[string]string{
				"enter": "apply",
				"esc":   "cancel",
			}
		} else if m.visualMode {
			shortcuts = map[string]string{
				"↑/↓":  "nav",
				"space": "toggle",
				"esc":   "cancel",
			}
		}

		statusBar.SetShortcuts(shortcuts)
		b.WriteString(statusBar.Render(m.width))
	}

	// Overlays
	view := b.String()

	if m.inputMode {
		return m.renderInputOverlay(view)
	}

	if m.bulkMenuOpen {
		return m.renderBulkMenuOverlay(view)
	}

	return view
}

// renderTableHeader renders the table header with column names and sort indicator
func (m *BrowserViewModel) renderTableHeader() string {
	var b strings.Builder

	// Checkbox column
	headerStyle := lipgloss.NewStyle().Foreground(styles.Secondary).Bold(true)
	b.WriteString("  ")
	b.WriteString(headerStyle.Render("☐ "))

	// Name column with sort indicator
	nameLabel := "Name"
	if m.sortBy == SortByName {
		if m.sortDesc {
			nameLabel += " ↓"
		} else {
			nameLabel += " ↑"
		}
	}
	b.WriteString(headerStyle.Render(fmt.Sprintf("%-52s", nameLabel)))

	// Size column with sort indicator
	sizeLabel := "Size"
	if m.sortBy == SortBySize {
		if m.sortDesc {
			sizeLabel += " ↓"
		} else {
			sizeLabel += " ↑"
		}
	}
	b.WriteString(headerStyle.Render(fmt.Sprintf("%-12s", sizeLabel)))

	// Date column with sort indicator
	dateLabel := "Modified"
	if m.sortBy == SortByDate {
		if m.sortDesc {
			dateLabel += " ↓"
		} else {
			dateLabel += " ↑"
		}
	}
	b.WriteString(headerStyle.Render(fmt.Sprintf("%-16s", dateLabel)))

	// Category column with sort indicator
	categoryLabel := "Category"
	if m.sortBy == SortByCategory {
		if m.sortDesc {
			categoryLabel += " ↓"
		} else {
			categoryLabel += " ↑"
		}
	}
	b.WriteString(headerStyle.Render(categoryLabel))

	return b.String()
}

// renderFileRow renders a single file row
func (m *BrowserViewModel) renderFileRow(i int) string {
	var b strings.Builder
	file := m.filteredFiles[i]

	// Cursor
	cursor := "  "
	if i == m.cursor {
		cursor = styles.SelectedStyle.Render("→ ")
	}
	b.WriteString(cursor)

	// Checkbox
	checkbox := styles.UncheckedBox()
	if m.isSelected(i) {
		checkbox = styles.CheckedBox()
	}
	b.WriteString(checkbox)
	b.WriteString(" ")

	// Visual mode highlight
	inVisualRange := m.visualMode && i >= min(m.visualStartIdx, m.cursor) && i <= max(m.visualStartIdx, m.cursor)

	// Calculate available width for filename
	// Total width - cursor (2) - checkbox (4) - size (12) - date (16) - category (15) - spacing
	availableForPath := m.width - 55
	if availableForPath < 20 {
		availableForPath = 20
	}
	if availableForPath > 60 {
		availableForPath = 60
	}

	// Filename (truncate intelligently based on terminal width)
	filename := uiutils.TruncatePath(file.Path, availableForPath)

	filenameStyle := styles.FilePathStyle
	if inVisualRange {
		filenameStyle = lipgloss.NewStyle().Foreground(styles.Text).Background(styles.Primary)
	}
	b.WriteString(filenameStyle.Render(fmt.Sprintf("%-*s", availableForPath+2, filename)))

	// Size with color coding
	sizeStr := utils.FormatBytes(file.Size)
	sizeStyle := styles.GetFileSizeStyle(file.Size)
	b.WriteString(sizeStyle.Render(fmt.Sprintf("%-12s", sizeStr)))

	// Modified time (adjust for small terminals)
	modTime := formatRelativeTime(file.ModTime)
	if m.width < 100 {
		// Shorten time display for narrow terminals
		modTime = formatShortTime(file.ModTime)
	}
	b.WriteString(styles.DimStyle.Render(fmt.Sprintf("%-16s", modTime)))

	// Category
	b.WriteString(styles.CategoryStyle.Render(file.Category))

	return b.String()
}

// renderPagination renders pagination info
func (m *BrowserViewModel) renderPagination() string {
	if len(m.filteredFiles) == 0 {
		return ""
	}

	totalPages := (len(m.filteredFiles) + m.pageSize - 1) / m.pageSize
	currentPage := (m.cursor / m.pageSize) + 1

	startItem := m.offset + 1
	endItem := min(m.offset+m.pageSize, len(m.filteredFiles))

	info := fmt.Sprintf("Page %d/%d (Items %d-%d of %d)",
		currentPage, totalPages, startItem, endItem, len(m.filteredFiles))

	return styles.DimStyle.Render(info)
}

// renderSummary renders the selection summary
func (m *BrowserViewModel) renderSummary() string {
	var selectedCount int
	var selectedSize int64

	for idx, isSelected := range m.selectedIndices {
		if isSelected && idx < len(m.files) {
			selectedCount++
			selectedSize += m.files[idx].Size
		}
	}

	summary := fmt.Sprintf("Selected: %d/%d files, %s",
		selectedCount, len(m.files), utils.FormatBytes(selectedSize))

	return styles.SubtitleStyle.Render(summary)
}

// renderFilterStatus renders the filter status
func (m *BrowserViewModel) renderFilterStatus() string {
	if m.filterMode {
		return styles.InfoStyle.Render(fmt.Sprintf("Filter: %s_", m.filterText))
	}

	if m.filterActive {
		matchCount := len(m.filteredFiles)
		return styles.SuccessStyle.Render(fmt.Sprintf("Filter: %s (%d matches)", m.filterText, matchCount))
	}

	return ""
}

// renderHelp renders help text
func (m *BrowserViewModel) renderHelp() string {
	if m.visualMode {
		helpText := "Visual Mode | ↑/↓ move | space/enter toggle range | esc cancel"
		if m.width < 80 {
			helpText = "Visual | ↑/↓ | space sel | esc"
		}
		return styles.HelpStyle.Render(helpText)
	}

	if m.filterMode {
		helpText := "Filter Mode | type to filter | enter apply | esc cancel"
		if m.width < 80 {
			helpText = "Filter | type | enter | esc"
		}
		return styles.HelpStyle.Render(helpText)
	}

	// Adjust help text based on terminal width
	help := "↑/↓/j/k navigate | space toggle | x toggle+down | s sort | S reverse | / filter | b bulk | v visual | gg/G jump | enter continue"
	if m.width < 120 {
		help = "↑/↓ nav | space sel | s sort | / filter | b bulk | v visual | enter next"
	}
	if m.width < 80 {
		help = "↑/↓ | space | s sort | / filt | enter"
	}
	return styles.HelpStyle.Render(help)
}

// renderBulkMenuOverlay renders the bulk operations menu
func (m *BrowserViewModel) renderBulkMenuOverlay(content string) string {
	// Create menu
	var menu strings.Builder
	menuStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Padding(1, 2).
		Background(styles.BgDark)

	menu.WriteString(styles.TitleStyle.Render("Bulk Operations"))
	menu.WriteString("\n\n")

	for i, item := range m.bulkMenuItems {
		cursor := "  "
		if i == m.bulkMenuCursor {
			cursor = styles.SelectedStyle.Render("→ ")
		}
		menu.WriteString(cursor)
		menu.WriteString(item)
		menu.WriteString("\n")
	}

	menu.WriteString("\n")
	menu.WriteString(styles.HelpStyle.Render("↑/↓ navigate | enter select | esc cancel"))

	menuBox := menuStyle.Render(menu.String())

	// Place menu in center
	return lipgloss.Place(100, 30, lipgloss.Center, lipgloss.Center, menuBox, lipgloss.WithWhitespaceChars(""), lipgloss.WithWhitespaceForeground(lipgloss.Color("#333")))
}

// renderInputOverlay renders the input prompt
func (m *BrowserViewModel) renderInputOverlay(content string) string {
	var input strings.Builder
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Info).
		Padding(1, 2).
		Background(styles.BgDark)

	input.WriteString(styles.InfoStyle.Render("Input"))
	input.WriteString("\n\n")
	input.WriteString(m.inputPrompt)
	input.WriteString("\n")
	input.WriteString(styles.HighlightStyle.Render(m.inputValue + "_"))
	input.WriteString("\n\n")
	input.WriteString(styles.HelpStyle.Render("enter confirm | esc cancel"))

	inputBox := inputStyle.Render(input.String())

	// Place input in center
	return lipgloss.Place(100, 30, lipgloss.Center, lipgloss.Center, inputBox, lipgloss.WithWhitespaceChars(""), lipgloss.WithWhitespaceForeground(lipgloss.Color("#333")))
}

// proceedToConfirmation sends a message to proceed to confirmation
func (m *BrowserViewModel) proceedToConfirmation() tea.Cmd {
	var selected []scanner.FileInfo
	for idx, isSelected := range m.selectedIndices {
		if isSelected && idx < len(m.files) {
			selected = append(selected, m.files[idx])
		}
	}

	return func() tea.Msg {
		return FilesSelectedMsg{SelectedFiles: selected}
	}
}

// Helper functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// formatRelativeTime formats a time as relative (e.g., "2 days ago")
func formatRelativeTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	}
	if duration < time.Hour {
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	}
	if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	if duration < 30*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
	if duration < 365*24*time.Hour {
		months := int(duration.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}

	years := int(duration.Hours() / 24 / 365)
	if years == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", years)
}

// formatShortTime formats a time in short form for narrow terminals
func formatShortTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Hour {
		return "< 1h"
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	if duration < 30*24*time.Hour {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
	if duration < 365*24*time.Hour {
		return fmt.Sprintf("%dmo", int(duration.Hours()/24/30))
	}
	return fmt.Sprintf("%dy", int(duration.Hours()/24/365))
}
