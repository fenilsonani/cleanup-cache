package models

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/fenilsonani/cleanup-cache/internal/config"
	"github.com/fenilsonani/cleanup-cache/internal/platform"
	"github.com/fenilsonani/cleanup-cache/internal/scanner"
)

// ViewState represents the current view in the app
type ViewState int

const (
	ViewScanning ViewState = iota
	ViewCategorySelection
	ViewFileBrowser
	ViewConfirmation
	ViewCleaning
	ViewSummary
	ViewHelp
)

// AppModel is the root model for the interactive TUI
type AppModel struct {
	// Current state
	state         ViewState
	previousState ViewState // For back navigation

	// Shared data
	config       *config.Config
	platformInfo *platform.Info
	scanResult   *scanner.ScanResult

	// View models
	scanView       *ScanViewModel
	categoryView   *CategoryViewModel
	browserView    *BrowserViewModel
	confirmView    *ConfirmViewModel
	cleanupView    *CleanupViewModel
	summaryView    *SummaryViewModel

	// UI state
	width  int
	height int
	err    error
}

// NewAppModel creates a new app model
func NewAppModel(cfg *config.Config, platformInfo *platform.Info) *AppModel {
	return &AppModel{
		state:        ViewScanning,
		config:       cfg,
		platformInfo: platformInfo,
		scanResult:   &scanner.ScanResult{Files: []scanner.FileInfo{}},
	}
}

// Init initializes the model
func (m *AppModel) Init() tea.Cmd {
	// Start scanning immediately
	m.scanView = NewScanViewModel(m.config, m.platformInfo)
	return m.scanView.Init()
}

// Update handles messages
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.state == ViewScanning || m.state == ViewSummary {
				return m, tea.Quit
			}
		case "?":
			m.previousState = m.state
			m.state = ViewHelp
			return m, nil
		case "esc":
			// Handle back navigation
			switch m.state {
			case ViewHelp:
				// Return to previous view
				m.state = m.previousState
				return m, nil
			case ViewFileBrowser:
				// Go back to category selection
				m.state = ViewCategorySelection
				return m, nil
			case ViewConfirmation:
				// Go back to file browser
				m.state = ViewFileBrowser
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case ScanCompleteMsg:
		// Scanning complete, move to category selection
		m.scanResult = msg.Result
		m.categoryView = NewCategoryViewModel(m.scanResult)
		m.state = ViewCategorySelection
		return m, nil

	case CategoriesSelectedMsg:
		// Categories selected, move to file browser
		m.browserView = NewBrowserViewModel(m.scanResult, msg.SelectedCategories)
		m.state = ViewFileBrowser
		return m, nil

	case FilesSelectedMsg:
		// Files selected, move to confirmation
		m.confirmView = NewConfirmViewModel(msg.SelectedFiles)
		m.state = ViewConfirmation
		return m, nil

	case ConfirmedMsg:
		// User confirmed, start cleanup
		m.cleanupView = NewCleanupViewModel(m.confirmView.files, m.config)
		m.state = ViewCleaning
		return m, m.cleanupView.Init()

	case CleanupCompleteMsg:
		// Cleanup complete, show summary
		m.summaryView = NewSummaryViewModel(msg.Result)
		m.state = ViewSummary
		return m, nil
	}

	// Delegate to current view
	return m.delegateUpdate(msg)
}

// delegateUpdate delegates the update to the current view
func (m *AppModel) delegateUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.state {
	case ViewScanning:
		if m.scanView != nil {
			m.scanView, cmd = m.scanView.Update(msg)
		}
	case ViewCategorySelection:
		if m.categoryView != nil {
			m.categoryView, cmd = m.categoryView.Update(msg)
		}
	case ViewFileBrowser:
		if m.browserView != nil {
			m.browserView, cmd = m.browserView.Update(msg)
		}
	case ViewConfirmation:
		if m.confirmView != nil {
			m.confirmView, cmd = m.confirmView.Update(msg)
		}
	case ViewCleaning:
		if m.cleanupView != nil {
			m.cleanupView, cmd = m.cleanupView.Update(msg)
		}
	case ViewSummary:
		if m.summaryView != nil {
			m.summaryView, cmd = m.summaryView.Update(msg)
		}
	}

	return m, cmd
}

// View renders the current view
func (m *AppModel) View() string {
	if m.err != nil {
		return "Error: " + m.err.Error() + "\n\nPress q to quit."
	}

	switch m.state {
	case ViewScanning:
		if m.scanView != nil {
			return m.scanView.View()
		}
	case ViewCategorySelection:
		if m.categoryView != nil {
			return m.categoryView.View()
		}
	case ViewFileBrowser:
		if m.browserView != nil {
			return m.browserView.View()
		}
	case ViewConfirmation:
		if m.confirmView != nil {
			return m.confirmView.View()
		}
	case ViewCleaning:
		if m.cleanupView != nil {
			return m.cleanupView.View()
		}
	case ViewSummary:
		if m.summaryView != nil {
			return m.summaryView.View()
		}
	case ViewHelp:
		return m.renderHelp()
	}

	return "Loading..."
}

// renderHelp renders the help view
func (m *AppModel) renderHelp() string {
	help := `
CleanupCache - Interactive Mode Help

Global Shortcuts:
  q       - Quit (from most views)
  ctrl+c  - Force quit
  ?       - Toggle this help
  esc     - Go back / Close help

Category Selection:
  ↑/↓     - Navigate categories
  space   - Toggle category selection
  enter   - Proceed to file browser
  a       - Select all categories
  d       - Deselect all categories

File Browser:
  ↑/↓     - Navigate files
  space   - Toggle file selection
  enter   - View file details
  /       - Open filter
  s       - Open sort menu
  b       - Open bulk actions
  tab     - Switch between panels
  ctrl+a  - Select all files
  ctrl+d  - Deselect all files
  ctrl+i  - Invert selection

Cleanup:
  p       - Pause/Resume
  ctrl+c  - Cancel (safely)

Press ESC to close this help.
`
	return help
}

// Custom messages
type ScanCompleteMsg struct {
	Result *scanner.ScanResult
}

type CategoriesSelectedMsg struct {
	SelectedCategories []string
}

type FilesSelectedMsg struct {
	SelectedFiles []scanner.FileInfo
}

type ConfirmedMsg struct{}

type CleanupCompleteMsg struct {
	Result interface{} // Will be CleanResult
}
