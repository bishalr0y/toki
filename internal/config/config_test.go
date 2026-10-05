package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadDefaultConfig(t *testing.T) {
	cfg, err := ReadDefaultConfig()
	if err != nil {
		t.Fatalf("ReadDefaultConfig() returned error: %v", err)
	}

	if len(cfg.Timers) == 0 {
		t.Fatal("expected at least one timer in default config")
	}

	for i, timer := range cfg.Timers {
		if timer.Name == "" {
			t.Errorf("timer[%d].Name is empty", i)
		}
		if timer.FocusMins <= 0 {
			t.Errorf("timer[%d].FocusMins = %d, want > 0", i, timer.FocusMins)
		}
		if timer.BreakMins <= 0 {
			t.Errorf("timer[%d].BreakMins = %d, want > 0", i, timer.BreakMins)
		}
	}
}

func TestReadConfig_CreatesDefault(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig() returned error: %v", err)
	}

	if len(cfg.Timers) == 0 {
		t.Fatal("expected at least one timer after creating default config")
	}

	configPath := filepath.Join(tmpHome, ".config/toki/config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("expected config file to be created at %s: %v", configPath, err)
	}
}

func TestReadConfig_ExistingFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, ".config/toki")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configContent := `timers:
  - name: Test Split
    focus: 15
    break: 5
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig() returned error: %v", err)
	}

	if len(cfg.Timers) != 1 {
		t.Fatalf("expected 1 timer, got %d", len(cfg.Timers))
	}

	if cfg.Timers[0].Name != "Test Split" {
		t.Errorf("expected name 'Test Split', got %q", cfg.Timers[0].Name)
	}
	if cfg.Timers[0].FocusMins != 15 {
		t.Errorf("expected focus 15, got %d", cfg.Timers[0].FocusMins)
	}
	if cfg.Timers[0].BreakMins != 5 {
		t.Errorf("expected break 5, got %d", cfg.Timers[0].BreakMins)
	}
}

// B4: zero and negative durations used to be accepted silently. `focus: 0`
// produced an instant "session completed!", and a negative value made the range
// loop iterate zero times, so the session was skipped without a word.
// Validation has to happen at load time, not when a split is picked. Otherwise
// the user only discovers their config is broken after typing a choice.
func TestReadConfigRejectsAnInvalidFile(t *testing.T) {
	steps := []struct {
		name    string
		content string
		wantSay string
	}{
		{
			name:    "zero focus",
			content: "timers:\n  - name: broken\n    focus: 0\n    break: 5\n",
			wantSay: "focus",
		},
		{
			name:    "negative break",
			content: "timers:\n  - name: broken\n    focus: 25\n    break: -5\n",
			wantSay: "break",
		},
		{
			name:    "no timers",
			content: "timers: []\n",
			wantSay: "at least one",
		},
	}

	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			tmpHome := t.TempDir()
			t.Setenv("HOME", tmpHome)

			configDir := filepath.Join(tmpHome, ".config/toki")
			if err := os.MkdirAll(configDir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(configDir, "config.yaml")
			if err := os.WriteFile(path, []byte(s.content), 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := ReadConfig()
			if err == nil {
				t.Fatal("ReadConfig() = nil error, want a validation failure")
			}
			if !strings.Contains(err.Error(), s.wantSay) {
				t.Errorf("error %q does not mention %q", err, s.wantSay)
			}
		})
	}
}

// Cycles and the long break were added after the first release, so every config
// written before them unmarshals them as zero. That has to mean "use the
// default" rather than "reject this file", or the new keys would break every
// existing setup on upgrade.
func TestAConfigWrittenBeforeCyclesExistedStillLoads(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, ".config/toki")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	old := "timers:\n  - name: Classic\n    focus: 25\n    break: 5\n"
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig() = %v, want an existing config to keep working", err)
	}

	got := cfg.Timers[0]
	if got.Cycles != DefaultCycles {
		t.Errorf("Cycles = %d, want the default %d", got.Cycles, DefaultCycles)
	}
	if got.LongBreakMins != DefaultLongBreakMins {
		t.Errorf("LongBreakMins = %d, want the default %d", got.LongBreakMins, DefaultLongBreakMins)
	}
}

// Each split carries its own rhythm, so one split can be four rounds while
// another is two.
func TestCyclesAndTheLongBreakAreSetPerSplit(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	configDir := filepath.Join(tmpHome, ".config/toki")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `timers:
  - name: Standard
    focus: 25
    break: 5
    cycles: 4
    long_break: 15
  - name: Double
    focus: 50
    break: 10
    cycles: 2
    long_break: 30
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig() = %v", err)
	}

	if len(cfg.Timers) != 2 {
		t.Fatalf("got %d timers, want 2", len(cfg.Timers))
	}
	if got, want := cfg.Timers[0], (TimerSplit{
		Name: "Standard", FocusMins: 25, BreakMins: 5, Cycles: 4, LongBreakMins: 15,
	}); got != want {
		t.Errorf("Timers[0] = %+v, want %+v", got, want)
	}
	if got, want := cfg.Timers[1].Cycles, 2; got != want {
		t.Errorf("Timers[1].Cycles = %d, want %d; each split keeps its own rhythm", got, want)
	}
	if got, want := cfg.Timers[1].LongBreakMins, 30; got != want {
		t.Errorf("Timers[1].LongBreakMins = %d, want %d", got, want)
	}
}

// Zero has to mean "unset" so that old files load, but a negative value is
// plainly a mistake and must not be quietly replaced by the default: a user who
// typed -1 would see four rounds and no explanation.
func TestNegativeCyclesIsRejectedRatherThanDefaulted(t *testing.T) {
	cfg := Config{Timers: []TimerSplit{
		{Name: "Standard", FocusMins: 25, BreakMins: 5, Cycles: -1, LongBreakMins: 15},
	}}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() = nil for cycles: -1, want an error")
	}
	for _, want := range []string{"timers[0]", "Standard", "cycles", "-1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestNegativeLongBreakIsRejectedRatherThanDefaulted(t *testing.T) {
	cfg := Config{Timers: []TimerSplit{
		{Name: "Standard", FocusMins: 25, BreakMins: 5, Cycles: 4, LongBreakMins: -15},
	}}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() = nil for long_break: -15, want an error")
	}
	for _, want := range []string{"timers[0]", "Standard", "long_break", "-15"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestValidateAcceptsAWellFormedConfig(t *testing.T) {
	cfg := Config{Timers: []TimerSplit{
		{Name: "Standard", FocusMins: 25, BreakMins: 5},
		{Name: "Long Focus", FocusMins: 50, BreakMins: 10},
	}}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidateRejectsUnusableTimers(t *testing.T) {
	steps := []struct {
		name    string
		cfg     Config
		wantSay []string
	}{
		{
			name:    "no timers at all",
			cfg:     Config{},
			wantSay: []string{"at least one"},
		},
		{
			name:    "empty name",
			cfg:     Config{Timers: []TimerSplit{{Name: "   ", FocusMins: 25, BreakMins: 5}}},
			wantSay: []string{"timers[0]", "name"},
		},
		{
			name:    "zero focus",
			cfg:     Config{Timers: []TimerSplit{{Name: "Standard", FocusMins: 0, BreakMins: 5}}},
			wantSay: []string{"timers[0]", "Standard", "focus", "0"},
		},
		{
			name:    "negative focus",
			cfg:     Config{Timers: []TimerSplit{{Name: "Standard", FocusMins: -25, BreakMins: 5}}},
			wantSay: []string{"Standard", "focus", "-25"},
		},
		{
			name:    "zero break",
			cfg:     Config{Timers: []TimerSplit{{Name: "Standard", FocusMins: 25, BreakMins: 0}}},
			wantSay: []string{"Standard", "break", "0"},
		},
		{
			name:    "negative break",
			cfg:     Config{Timers: []TimerSplit{{Name: "Standard", FocusMins: 25, BreakMins: -5}}},
			wantSay: []string{"Standard", "break", "-5"},
		},
		{
			name: "offending entry is identified by position",
			cfg: Config{Timers: []TimerSplit{
				{Name: "ok", FocusMins: 25, BreakMins: 5},
				{Name: "also ok", FocusMins: 25, BreakMins: 5},
				{Name: "broken", FocusMins: 0, BreakMins: 5},
			}},
			wantSay: []string{"timers[2]", "broken"},
		},
	}

	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			err := s.cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error")
			}
			for _, want := range s.wantSay {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}
