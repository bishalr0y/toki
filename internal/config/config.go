package config

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

//go:embed default.yaml
var defaultCfg embed.FS

type TimerSplit struct {
	Name  string `yaml:"name"`
	Focus int    `yaml:"focus"`
	Break int    `yaml:"break"`
}

type Config struct {
	Timers []TimerSplit `yaml:"timers"`
}

func ReadConfig() (Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("failed to get home dir: %w", err)
	}

	configPath := filepath.Join(homeDir, ".config/toki")
	configFile := "config.yaml"

	data, err := os.ReadFile(filepath.Join(configPath, configFile))

	if errors.Is(err, os.ErrNotExist) {
		defaultConfig, err := ReadDefaultConfig()
		if err != nil {
			return Config{}, fmt.Errorf("failed to read default config: %w", err)
		}

		configBytes, err := yaml.Marshal(defaultConfig)
		if err != nil {
			return Config{}, fmt.Errorf("failed to marshal default config: %w", err)
		}

		if err := os.MkdirAll(configPath, 0o755); err != nil {
			return Config{}, fmt.Errorf("failed to create config directory: %w", err)
		}

		if err := os.WriteFile(filepath.Join(configPath, configFile), configBytes, 0o644); err != nil {
			return Config{}, fmt.Errorf("failed to write default config: %w", err)
		}

		return defaultConfig, nil
	}

	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}

func ReadDefaultConfig() (Config, error) {
	data, err := defaultCfg.ReadFile("default.yaml")
	if err != nil {
		return Config{}, fmt.Errorf("failed to read embedded default config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal default config: %w", err)
	}

	return config, nil
}
