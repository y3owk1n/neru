package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	t.Run("Systray Defaults", func(t *testing.T) {
		if !cfg.Systray.Enabled {
			t.Error("Expected Systray.Enabled to be true by default")
		}
	})

	t.Run("General Keyboard Layout Defaults", func(t *testing.T) {
		if cfg.General.KBLayoutToUse != "" {
			t.Errorf("Expected General.KBLayoutToUse to be empty by default, got %q", cfg.General.KBLayoutToUse)
		}
	})
}
