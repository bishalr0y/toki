package main

import (
	"embed"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

//go:embed default.yaml
var defaultCfg embed.FS

func main() {
	err, config := ReadConfig()
	if err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	fmt.Println(config)
}

type Config struct {
	Timers []struct {
		Name  string `yaml:"name"`
		Focus int    `yaml:"focus"`
		Break int    `yaml:"break"`
	} `yaml:"timers"`
}

// ReadConfig() checks for config file in ~/.config/toki/config.yaml
func ReadConfig() (error, Config) {
	// get the home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.New("Failed to get the home dir: " + err.Error()), Config{}
	}

	// create the config path
	configPath := filepath.Join(homeDir, ".config/toki")
	configFile := "config.yaml"

	data, err := os.ReadFile(filepath.Join(configPath, configFile))

	// if the config doesnt exist - create one from the default config
	if errors.Is(err, os.ErrNotExist) {
		defaultConfig := ReadDefaultConfig()
		configBytes, _ := yaml.Marshal(defaultConfig)

		// create the reqd directories
		if err := os.MkdirAll(configPath, 0o755); err != nil {
			log.Fatalf("Error while creating the default config file")
		}

		if err := os.WriteFile(filepath.Join(configPath, configFile), configBytes, 0o644); err != nil {
			return errors.New("Failed to write the default config " + err.Error()), Config{}
		}

		return nil, defaultConfig
	}

	var config Config
	yaml.Unmarshal(data, &config)
	return nil, config
}

// ReadDefaultConfig() reads the embedded default config file
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
