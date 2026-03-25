package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func TestConfigValidateCustomHotkeys_Valid(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.CustomHotkeys["PageUp"] = config.StringOrStringArray{"action page_up", "idle"}
	cfg.Scroll.CustomHotkeys["gg"] = config.StringOrStringArray{"action go_top"}
	cfg.Grid.CustomHotkeys["Enter"] = config.StringOrStringArray{
		"action save_cursor_pos",
		"idle",
		"action wait_for_mode_exit",
		"action restore_cursor_pos",
	}

	err := cfg.ValidateCustomHotkeys()
	if err != nil {
		t.Fatalf("ValidateCustomHotkeys() unexpected error: %v", err)
	}
}

func TestConfigValidateCustomHotkeys_InvalidAction(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.CustomHotkeys["x"] = config.StringOrStringArray{"action nope"}

	err := cfg.ValidateCustomHotkeys()
	if err == nil {
		t.Fatal("ValidateCustomHotkeys() expected error, got nil")
	}
}

func TestConfigValidateCustomHotkeys_PrefixConflict(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Scroll.CustomHotkeys["g"] = config.StringOrStringArray{"action scroll_up"}
	cfg.Scroll.CustomHotkeys["gg"] = config.StringOrStringArray{"action go_top"}

	err := cfg.ValidateCustomHotkeys()
	if err == nil {
		t.Fatal("ValidateCustomHotkeys() expected prefix conflict error, got nil")
	}
}

func TestConfigValidateCustomHotkeys_SequenceWithoutPrefixConflict(t *testing.T) {
	cfg := config.DefaultConfig()
	// "gg" sequence with no single-key "g" binding should be fine
	cfg.Scroll.CustomHotkeys["gg"] = config.StringOrStringArray{"action go_top"}
	cfg.Scroll.CustomHotkeys["j"] = config.StringOrStringArray{"action scroll_down"}

	err := cfg.ValidateCustomHotkeys()
	if err != nil {
		t.Fatalf("ValidateCustomHotkeys() unexpected error: %v", err)
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
