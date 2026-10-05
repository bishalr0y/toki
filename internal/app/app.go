package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/bishalr0y/toki/internal/config"
	"github.com/bishalr0y/toki/internal/history"
)

// Run starts the interface and blocks until the user quits.
//
// The caller owns configuration loading so that a broken config is reported as
// a plain error on stderr, rather than from inside a full screen interface that
// has already taken over the terminal.
//
// History is not fatal. A file that cannot be read or written is reported on the
// summary, because losing the record of a session is worth mentioning but not
// worth refusing to run the timer over.
func Run(cfg config.Config, hist *history.Store) error {
	_, err := tea.NewProgram(New(cfg, hist)).Run()
	return err
}
