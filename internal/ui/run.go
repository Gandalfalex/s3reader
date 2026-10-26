// Package ui is the Bubble Tea terminal interface.
package ui

import (
	tea "charm.land/bubbletea/v2"

	"s3reader/internal/config"
)

// Run starts the TUI and blocks until the user quits.
func Run(s *config.Settings, logPath string) error {
	_, err := tea.NewProgram(newModel(s, logPath)).Run()
	return err
}
