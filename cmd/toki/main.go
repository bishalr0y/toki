// Command toki is a small Pomodoro timer for the terminal.
package main

import (
	"fmt"
	"os"

	"github.com/bishalr0y/toki/internal/app"
	"github.com/bishalr0y/toki/internal/config"
	"github.com/bishalr0y/toki/internal/history"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "toki: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Read once at startup. The old menu re-read the file on every iteration,
	// so edits made mid-session silently changed what was on screen.
	cfg, err := config.ReadConfig()
	if err != nil {
		return err
	}

	histPath, err := config.HistoryPath()
	if err != nil {
		return err
	}

	return app.Run(cfg, history.NewStore(histPath))
}
