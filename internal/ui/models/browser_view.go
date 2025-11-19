package models

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fenilsonani/cleanup-cache/internal/scanner"
	"github.com/fenilsonani/cleanup-cache/internal/ui/styles"
	"github.com/fenilsonani/cleanup-cache/pkg/utils"
)

// BrowserViewModel handles file browsing and selection
type BrowserViewModel struct {
	files           []scanner.FileInfo
	selectedIndices map[int]bool
	cursor          int
	offset          int
	pageSize        int
}

// NewBrowserViewModel creates a new browser view model
func NewBrowserViewModel(scanResult *scanner.ScanResult, selectedCategories []string) *BrowserViewModel {
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

	return &BrowserViewModel{
		files:           filteredFiles,
		selectedIndices: make(map[int]bool),
		cursor:          0,
		offset:          0,
		pageSize:        20,
	}
}

// Init initializes the browser view
func (m *BrowserViewModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m *BrowserViewModel) Update(msg tea.Msg) (*BrowserViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset--
				}
			}
		case "down", "j":
			if m.cursor < len(m.files)-1 {
				m.cursor++
				if m.cursor >= m.offset+m.pageSize {
					m.offset++
				}
			}
		case "space":
			m.selectedIndices[m.cursor] = !m.selectedIndices[m.cursor]
		case "ctrl+a":
			for i := range m.files {
				m.selectedIndices[i] = true
			}
		case "ctrl+d":
			m.selectedIndices = make(map[int]bool)
		case "enter":
			return m, m.proceedToConfirmation()
		}
	}

	return m, nil
}

// View renders the browser view
func (m *BrowserViewModel) View() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("📁 Select Files to Delete"))
	b.WriteString("\n\n")

	// File list
	end := m.offset + m.pageSize
	if end > len(m.files) {
		end = len(m.files)
	}

	for i := m.offset; i < end; i++ {
		file := m.files[i]

		cursor := "  "
		if i == m.cursor {
			cursor = styles.SelectedStyle.Render("→ ")
		}

		checkbox := styles.UncheckedBox()
		if m.selectedIndices[i] {
			checkbox = styles.CheckedBox()
		}

		// Truncate filename
		filename := file.Path
		if len(filename) > 50 {
			filename = "..." + filename[len(filename)-47:]
		}

		line := fmt.Sprintf("%s%s %s %s",
			cursor,
			checkbox,
			styles.FilePathStyle.Render(filename),
			styles.FileSizeStyle.Render(utils.FormatBytes(file.Size)),
		)

		b.WriteString(line)
		b.WriteString("\n")
	}

	// Summary
	b.WriteString("\n")
	var selectedCount int
	var selectedSize int64
	for idx, isSelected := range m.selectedIndices {
		if isSelected && idx < len(m.files) {
			selectedCount++
			selectedSize += m.files[idx].Size
		}
	}

	b.WriteString(styles.SubtitleStyle.Render(fmt.Sprintf("Selected: %d/%d files, %s",
		selectedCount, len(m.files), utils.FormatBytes(selectedSize))))
	b.WriteString("\n\n")

	b.WriteString(styles.HelpStyle.Render("↑/↓ navigate | space toggle | ctrl+a select all | ctrl+d deselect all | enter continue"))

	return b.String()
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
