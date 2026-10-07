package config

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bishalr0y/toki/internal/sound"

	"go.yaml.in/yaml/v3"
)

//go:embed default.yaml
var defaultCfg embed.FS

const (
	// DefaultCycles is how many focus rounds a split runs when the config does
	// not say, which is the usual Pomodoro rhythm.
	DefaultCycles = 4
	// DefaultLongBreakMins is how long the rest after the final round lasts when
	// the config does not say.
	DefaultLongBreakMins = 15
)

type TimerSplit struct {
	Name      string `yaml:"name"`
	FocusMins int    `yaml:"focus"`
	BreakMins int    `yaml:"break"`
	// Cycles is how many focus rounds this split runs before its long break.
	// Zero means unset and picks up DefaultCycles, so a config file written
	// before this key existed still works.
	Cycles int `yaml:"cycles"`
	// LongBreakMins is how long the break after the final round lasts. Zero
	// means unset and picks up DefaultLongBreakMins.
	LongBreakMins int `yaml:"long_break"`
}

type Config struct {
	Timers []TimerSplit `yaml:"timers"`
}

// withDefaults fills in whatever the file left unset.
//
// It exists because these keys were added after the first release: a config
// written before them unmarshals them as zero, and treating zero as invalid
// would break every existing setup on upgrade. Negative values are left alone
// so Validate can report them as the mistakes they are.
func (c Config) withDefaults() Config {
	for i, split := range c.Timers {
		if split.Cycles == 0 {
			split.Cycles = DefaultCycles
		}
		if split.LongBreakMins == 0 {
			split.LongBreakMins = DefaultLongBreakMins
		}
		c.Timers[i] = split
	}
	return c
}

// Dir reports where toki keeps its files.
func Dir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home dir: %w", err)
	}
	return filepath.Join(homeDir, ".config/toki"), nil
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
		// Zero is tolerated here because it means "unset" and is filled in by
		// withDefaults, but a negative value can only be a mistake.
		if split.Cycles < 0 {
			return fmt.Errorf("timers[%d] %q: cycles must not be negative, got %d "+
				"(omit it for the default of %d)", i, split.Name, split.Cycles, DefaultCycles)
		}
		if split.LongBreakMins < 0 {
			return fmt.Errorf("timers[%d] %q: long_break must not be negative, got %d "+
				"(omit it for the default of %d)",
				i, split.Name, split.LongBreakMins, DefaultLongBreakMins)
		}
	}

	return nil
}

func ReadConfig() (Config, error) {
	configPath, err := Dir()
	if err != nil {
		return Config{}, err
	}

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

		writeDefaultSound(configPath)

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

	return config.withDefaults(), nil
}

// writeDefaultSound drops the built-in completion sound beside a config that was
// just created, so a fresh install is not silent.
//
// It is only called on that first run, and never overwrites: an existing file
// wins, which is what makes deleting the sound a way to turn it off for good
// instead of having it reappear on the next launch.
func writeDefaultSound(dir string) {
	path := filepath.Join(dir, sound.DefaultFile)

	// O_EXCL is the "never overwrite" part: if the user already has a sound
	// there, this fails and we leave theirs alone.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	_, _ = file.Write(sound.Default())
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

	return config.withDefaults(), nil
}
