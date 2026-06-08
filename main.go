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

	// TODO: create seperate files/functions to clean this mess up
	BlueBoldUnderline.Println("Welcome to toki!")

	var choice string
	for i, split := range config.Timers {
		fmt.Printf("%d %v (Work: %v mins; Focus: %v mins)\n", i+1, split.Name, split.Focus, split.Break)
	}

	RedBold.Println("Press q to quit")
	GreenBold.Print("Enter your option >> ")
	fmt.Scan(&choice)

	// TODO: maybe any other way to handle this better?
	if choice == "q" {
		fmt.Println("quitting...")
		os.Exit(0)
	}

	option, err := strconv.Atoi(choice)
	if err != nil {
		log.Fatalln("invalid choice")
	}

	switch {
	case option <= len(config.Timers):
		fmt.Printf("split selected: %v\n", config.Timers[option-1])
	default:
		fmt.Println("invalid option")
		os.Exit(1)
	}

	// TODO: create a new split - open $EDITOR to create custom split
	// then verify if the config is correct

	// TODO: after the split cycle is completed, give a option to exit
}
