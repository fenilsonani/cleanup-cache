package models

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/fenilsonani/cleanup-cache/internal/config"
	"github.com/fenilsonani/cleanup-cache/internal/platform"
	"github.com/fenilsonani/cleanup-cache/internal/scanner"
	"github.com/fenilsonani/cleanup-cache/internal/ui/styles"
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
	m.scanView = NewScanViewModel(m.config, m.platformInfo, m.width, m.height)
	return m.scanView.Init()
}

// Update handles messages
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// Allow quitting from most views except during active cleanup
			if m.state != ViewCleaning {
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
		m.categoryView = NewCategoryViewModel(m.scanResult, m.width, m.height)
		m.state = ViewCategorySelection
		return m, nil

	case CategoriesSelectedMsg:
		// Categories selected, move to file browser
		m.browserView = NewBrowserViewModel(m.scanResult, msg.SelectedCategories, m.width, m.height)
		m.state = ViewFileBrowser
		return m, nil

	case FilesSelectedMsg:
		// Files selected, move to confirmation
		m.confirmView = NewConfirmViewModel(msg.SelectedFiles, m.width, m.height)
		m.state = ViewConfirmation
		return m, nil

	case ConfirmedMsg:
		// User confirmed, start cleanup
		m.cleanupView = NewCleanupViewModel(m.confirmView.files, m.config, m.width, m.height)
		m.state = ViewCleaning
		return m, m.cleanupView.Init()

	case ReviewSelectionMsg:
		// User wants to review/edit selection, go back to file browser
		m.state = ViewFileBrowser
		return m, nil

	case CleanupCompleteMsg:
		// Cleanup complete, show summary
		m.summaryView = NewSummaryViewModel(msg.Result, m.width, m.height)
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

// renderHelp renders the help view with context-aware content
func (m *AppModel) renderHelp() string {
	var b strings.Builder

	// Get help content based on current view
	var viewName string
	var helpContent string

	switch m.previousState {
	case ViewScanning:
		viewName = "Scan View"
		helpContent = m.getHelpForScan()
	case ViewCategorySelection:
		viewName = "Category Selection"
		helpContent = m.getHelpForCategory()
	case ViewFileBrowser:
		viewName = "File Browser"
		helpContent = m.getHelpForBrowser()
	case ViewConfirmation:
		viewName = "Confirmation"
		helpContent = m.getHelpForConfirm()
	case ViewCleaning:
		viewName = "Cleanup"
		helpContent = m.getHelpForCleanup()
	case ViewSummary:
		viewName = "Summary"
		helpContent = m.getHelpForSummary()
	default:
		viewName = "General"
		helpContent = m.getHelpForGeneral()
	}

	// Build help screen
	// Title
	title := fmt.Sprintf("Help - %s", viewName)
	b.WriteString(styles.TitleStyle.Render(title))
	b.WriteString("\n\n")

	// Content
	b.WriteString(helpContent)

	b.WriteString("\n\n")
	b.WriteString(styles.HelpStyle.Render("Press any key to close"))

	return b.String()
}

func (m *AppModel) getHelpForScan() string {
	return `Scanning your system for cache files and cleanable items.

Actions:
  ctrl+c  - Cancel scan and exit
  q       - Cancel scan and exit

The scan will automatically proceed to category selection when complete.`
}

func (m *AppModel) getHelpForCategory() string {
	return `Select which categories of files you want to clean.

Navigation:
  ↑/k     - Move up
  ↓/j     - Move down
  gg      - Go to top
  G       - Go to bottom

Selection:
  space   - Toggle category
  a       - Select all
  d       - Deselect all

Actions:
  enter   - Proceed to file browser
  esc     - Go back
  q       - Quit`
}

func (m *AppModel) getHelpForBrowser() string {
	return `Browse and select individual files to delete.

Navigation               Selection
  ↑/k     Move up          space    Toggle item
  ↓/j     Move down        x        Toggle + down
  gg      Top              v        Visual mode
  G       Bottom           ctrl+a   Select all
  ctrl+f  Page down        ctrl+d   Deselect all
  ctrl+b  Page up          ctrl+i   Invert selection

Actions                  Other
  enter   Proceed          /        Search/filter
  s       Sort menu        tab      Switch panel
  b       Bulk actions     ?        Toggle help
  esc     Back             q        Quit`
}

func (m *AppModel) getHelpForConfirm() string {
	return `Review and confirm your deletion choices.

Navigation:
  ←/→/h/l - Switch between buttons

Actions:
  enter   - Confirm selection
  y       - Yes, proceed
  n       - No, cancel
  e       - Edit selection (go back)
  esc     - Go back
  q       - Quit

Warning: Deleted files cannot be recovered!`
}

func (m *AppModel) getHelpForCleanup() string {
	return `Deleting selected files. This may take a moment.

Actions:
  p       - Pause/Resume (if available)
  ctrl+c  - Cancel cleanup (safely)

Progress will be shown in real-time.
The operation will proceed to the summary when complete.`
}

func (m *AppModel) getHelpForSummary() string {
	return `Cleanup operation complete. Review the results.

Actions:
  enter   - Exit application
  q       - Exit application

Results show:
  - Files successfully deleted
  - Space freed
  - Any errors or skipped files`
}

func (m *AppModel) getHelpForGeneral() string {
	return `CleanupCache - Interactive Mode Help

Global Shortcuts:
  ?       - Toggle this help
  esc     - Go back / Close help
  q       - Quit (from most views)
  ctrl+c  - Force quit

This interactive mode guides you through:
  1. Scanning - Find cleanable files
  2. Category Selection - Choose file types
  3. File Browser - Select specific files
  4. Confirmation - Review your choices
  5. Cleanup - Delete selected files
  6. Summary - View results

Press ? at any time to see context-specific help.`
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

type ReviewSelectionMsg struct{}

type CleanupCompleteMsg struct {
	Result interface{} // Will be CleanResult
}
