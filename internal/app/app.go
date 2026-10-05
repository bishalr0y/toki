package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/bishalr0y/toki/internal/config"
)

// Run starts the interface and blocks until the user quits.
//
// The caller owns configuration loading so that a broken config is reported as
// a plain error on stderr, rather than from inside a full screen interface that
// has already taken over the terminal.
func Run(cfg config.Config) error {
	_, err := tea.NewProgram(New(cfg)).Run()
	return err
}
