package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fenilsonani/cleanup-cache/internal/config"
	"github.com/fenilsonani/cleanup-cache/internal/platform"
	"github.com/fenilsonani/cleanup-cache/internal/ui/models"
)

// RunInteractive starts the interactive TUI mode
func RunInteractive(cfg *config.Config) error {
	// Get platform info
	platformInfo, err := platform.GetInfo()
	if err != nil {
		return fmt.Errorf("failed to get platform info: %w", err)
	}

	// Create the app model
	m := models.NewAppModel(cfg, platformInfo)

	// Create the Bubble Tea program
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Run the program
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running interactive mode: %w", err)
	}

	return nil
}
