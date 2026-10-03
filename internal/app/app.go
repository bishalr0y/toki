package app

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/bishalr0y/toki/internal/config"
	"github.com/bishalr0y/toki/internal/session"
	"github.com/bishalr0y/toki/internal/timer"
	"github.com/bishalr0y/toki/internal/ui"
)

func Run() {
	for {
		cfg, err := config.ReadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading config: %v\n", err)
			os.Exit(1)
		}

		ui.PrintBanner()
		ui.PrintAvailableSplits(cfg)

		var choice string

		ui.RedBold.Println("Press q to quit")
		ui.GreenBold.Print("Enter your option >> ")
		if _, err := fmt.Scan(&choice); err != nil {
			// EOF or unreadable input (piped or closed stdin). Exit cleanly instead
			// of looping forever on an empty choice.
			fmt.Fprintln(os.Stderr, "\nno input available, exiting")
			return
		}
		ui.ClearConsole()

		if choice == "q" || choice == "Q" {
			ui.PrintBanner()
			ui.MauveBold.Println("goodbye👋🏻...")
			os.Exit(0)
		}

		option, err := strconv.Atoi(choice)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid choice: %s\n", choice)
			continue
		}

		if option < 1 || option > len(cfg.Timers) {
			fmt.Fprintf(os.Stderr, "invalid option: %d\n", option)
			continue
		}

		selectedSplit := cfg.Timers[option-1]

		ui.PeachBold.Println("============================================================")
		ui.BlueBold.Printf("SELECTED SPLIT: %s || FOCUS: %d min(s) || BREAK: %d min(s)\n",
			selectedSplit.Name, selectedSplit.FocusMins, selectedSplit.BreakMins)
		ui.PeachBold.Println("============================================================")

		split := timer.Split{
			Focus: time.Duration(selectedSplit.FocusMins) * time.Minute,
			Break: time.Duration(selectedSplit.BreakMins) * time.Minute,
		}
		if err := session.Run(split); err != nil {
			fmt.Fprintf(os.Stderr, "session error: %v\n", err)
		}
	}
}
