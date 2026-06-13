package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

func main() {
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

	switch {
	case option <= len(config.Timers):
		focusDuration := config.Timers[option-1].Focus * 60
		StartSession("FOCUS", focusDuration)

		breakDuration := config.Timers[option-1].Break * 60
		StartSession("BREAK", breakDuration)

	default:
		fmt.Println("invalid option")
		os.Exit(1)
	}

	// TODO: dont let the user spam keys when the timer is running (only allow 'q' to quit)
	// TODO: create a new split - open $EDITOR to create custom split
	// then verify if the config is correct

	// TODO: after the split cycle is completed, give a option to exit
}
