package config

import (
	"os"
	"path/filepath"
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
		if timer.Focus <= 0 {
			t.Errorf("timer[%d].Focus = %d, want > 0", i, timer.Focus)
		}
		if timer.Break <= 0 {
			t.Errorf("timer[%d].Break = %d, want > 0", i, timer.Break)
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
	if cfg.Timers[0].Focus != 15 {
		t.Errorf("expected focus 15, got %d", cfg.Timers[0].Focus)
	}
	if cfg.Timers[0].Break != 5 {
		t.Errorf("expected break 5, got %d", cfg.Timers[0].Break)
	}
}
