package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

func main() {
	for {
		err, config := ReadConfig()
		if err != nil {
			log.Fatalf("Error reading config: %v", err)
		}

		PrintBanner()
		PrintAvailableSplits(config)

		var choice string

		RedBold.Println("Press q to quit")
		GreenBold.Print("Enter your option >> ")
		fmt.Scan(&choice)
		ClearConsole()

		if choice == "q" || choice == "Q" {
			PrintBanner()
			MauveBold.Println("goodbye👋🏻...")
			os.Exit(0)
		}

		option, err := strconv.Atoi(choice)
		if err != nil {
			log.Fatalln("invalid choice")
		}

		selectedSplit := config.Timers[option-1]

		PeachBold.Println("============================================================")
		BlueBold.Printf("SELECTED SPLIT: %s || FOCUS: %d min(s) || BREAK: %d min(s)\n",
			selectedSplit.Name, selectedSplit.Focus, selectedSplit.Break)
		PeachBold.Println("============================================================")

		switch {
		case option <= len(config.Timers):
			focusDuration := selectedSplit.Focus * 60
			StartSession("FOCUS", focusDuration)

			breakDuration := selectedSplit.Break * 60
			StartSession("BREAK", breakDuration)

		default:
			fmt.Println("invalid option")
			os.Exit(1)
		}
	}
	// TODO: save the successfull work splits into badger db
}
