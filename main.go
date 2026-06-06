package main

import (
	"embed"
	"fmt"
	"log"

	"go.yaml.in/yaml/v3"
)

//go:embed default.yaml
var defaultCfg embed.FS

func main() {
	cfg := ReadDefaultConfig()
	for _, timer := range cfg.Timers {
		fmt.Printf("Name: %s\nFocus: %d\nBreak: %d\n",
			timer.Name, timer.Focus, timer.Break)
	}
}

type Config struct {
	Timers []struct {
		Name  string `yaml:"name"`
		Focus int    `yaml:"focus"`
		Break int    `yaml:"break"`
	} `yaml:"timers"`
}

func ReadDefaultConfig() Config {
	// read the file
	data, err := defaultCfg.ReadFile("default.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// unmarshal into struct
	var config Config

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("error unmarshaling: %v", err)
	}

	return config
}
