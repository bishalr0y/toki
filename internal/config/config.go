package config

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

//go:embed default.yaml
var defaultCfg embed.FS

type TimerSplit struct {
	Name      string `yaml:"name"`
	FocusMins int    `yaml:"focus"`
	BreakMins int    `yaml:"break"`
}

type Config struct {
	Timers []TimerSplit `yaml:"timers"`
}

// Validate reports the first problem that would make the config unusable.
//
// Without this, a zero duration yields an instant "session completed!" and a
// negative one makes the countdown loop iterate zero times, so the session is
// silently skipped. Errors name the offending entry by position and label so
// they can be acted on directly.
func (c Config) Validate() error {
	if len(c.Timers) == 0 {
		return errors.New("no timers configured: expected at least one entry under `timers`")
	}

	for i, split := range c.Timers {
		if strings.TrimSpace(split.Name) == "" {
			return fmt.Errorf("timers[%d]: name must not be empty", i)
		}
		if split.FocusMins <= 0 {
			return fmt.Errorf("timers[%d] %q: focus must be a positive number of minutes, got %d",
				i, split.Name, split.FocusMins)
		}
		if split.BreakMins <= 0 {
			return fmt.Errorf("timers[%d] %q: break must be a positive number of minutes, got %d",
				i, split.Name, split.BreakMins)
		}
	}

	return nil
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

		return defaultConfig, defaultConfig.Validate()
	}

	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config at %s: %w", filepath.Join(configPath, configFile), err)
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
