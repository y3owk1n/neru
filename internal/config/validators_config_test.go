package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func TestConfigValidateHotkeys_Valid(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.Hotkeys["PageUp"] = config.StringOrStringArray{"action page_up", "idle"}
	cfg.Scroll.Hotkeys["gg"] = config.StringOrStringArray{"action go_top"}
	cfg.RecursiveGrid.Hotkeys["`"] = config.StringOrStringArray{"toggle-cursor-follow-selection"}
	cfg.Grid.Hotkeys["Enter"] = config.StringOrStringArray{
		"action save_cursor_pos",
		"idle",
		"action wait_for_mode_exit",
		"action restore_cursor_pos",
	}

	err := cfg.ValidateHotkeys()
	if err != nil {
		t.Fatalf("ValidateHotkeys() unexpected error: %v", err)
	}
}

func TestConfigValidateHotkeys_AppOverridePrefixConflict(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.Hotkeys["gg"] = config.StringOrStringArray{"action left_click"}
	cfg.Hints.AppConfigs = []config.AppConfig{
		{
			BundleID: "com.apple.Safari",
			Hotkeys: map[string]config.StringOrStringArray{
				"g": {"action left_click"},
			},
		},
	}

	err := cfg.ValidateHotkeys()
	if err == nil {
		t.Fatal("ValidateHotkeys() expected merged app override prefix conflict, got nil")
	}
}

func TestConfigValidateHotkeys_InvalidAction(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.Hotkeys["x"] = config.StringOrStringArray{"action nope"}

	err := cfg.ValidateHotkeys()
	if err == nil {
		t.Fatal("ValidateHotkeys() expected error, got nil")
	}
}

func TestConfigValidateHotkeys_PrefixConflict(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Scroll.Hotkeys["g"] = config.StringOrStringArray{"action scroll_up"}
	cfg.Scroll.Hotkeys["gg"] = config.StringOrStringArray{"action go_top"}

	err := cfg.ValidateHotkeys()
	if err == nil {
		t.Fatal("ValidateHotkeys() expected prefix conflict error, got nil")
	}
}

func TestConfigValidateHotkeys_SequenceWithoutPrefixConflict(t *testing.T) {
	cfg := config.DefaultConfig()
	// "gg" sequence with no single-key "g" binding should be fine
	cfg.Scroll.Hotkeys["gg"] = config.StringOrStringArray{"action go_top"}
	cfg.Scroll.Hotkeys["j"] = config.StringOrStringArray{"action scroll_down"}

	err := cfg.ValidateHotkeys()
	if err != nil {
		t.Fatalf("ValidateHotkeys() unexpected error: %v", err)
	}
}

func TestConfigValidateScroll_OnlyStepValidation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Scroll.ScrollStep = 0

	err := cfg.ValidateScroll()
	if err == nil {
		t.Fatal("ValidateScroll() expected error for scroll_step=0, got nil")
	}

	cfg.Scroll.ScrollStep = 100

	err = cfg.ValidateScroll()
	if err != nil {
		t.Fatalf("ValidateScroll() unexpected error for valid step: %v", err)
	}
}

func TestConfigValidateHints_AsciiHintChars(t *testing.T) {
	cfg := config.DefaultConfig()

	cfg.Hints.HintCharacters = "ab"

	err := cfg.ValidateHints()
	if err != nil {
		t.Fatalf("ValidateHints() unexpected error: %v", err)
	}

	cfg.Hints.HintCharacters = "aé"

	err = cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error for non-ASCII hint_characters")
	}
}
