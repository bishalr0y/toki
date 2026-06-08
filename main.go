package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	err, config := ReadConfig()
	if err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	fmt.Println("Welcome to toki!")
	var choice int
	for i, split := range config.Timers {
		fmt.Printf("%d %v (Work: %v mins; Focus: %v mins)\n", i+1, split.Name, split.Focus, split.Break)
	}

	fmt.Println("Press 0 to quit")
	fmt.Print("Enter your option >> ")
	fmt.Scan(&choice)

	switch {
	case choice == 0:
		fmt.Println("quiting...")
		os.Exit(0)
	case choice <= len(config.Timers):
		fmt.Printf("split selected: %v\n", config.Timers[choice-1])
	default:
		fmt.Println("invalid option")
		os.Exit(1)
	}

	// TODO: create a new split - open $EDITOR to create custom split
	// then verify if the config is correct

	// TODO: after the split cycle is completed, give a option to exit
}
