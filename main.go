package main

import (
	"fmt"
	"log"
)

func main() {
	err, config := ReadConfig()
	if err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	fmt.Println(config)
	// TODO: let the user choose the split

	// TODO: create a new split - open $EDITOR to create custom split
	// then verify if the config is correct

	// TODO: after the split cycle is completed, give a option to exit
}
