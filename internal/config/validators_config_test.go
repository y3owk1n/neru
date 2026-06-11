package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func TestConfigValidateHotkeys_Valid(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.Hotkeys["PageUp"] = config.StringOrStringArray{"action page_up", config.CmdIdle}
	cfg.Scroll.Hotkeys["gg"] = config.StringOrStringArray{config.CmdGoTop}
	cfg.RecursiveGrid.Hotkeys["`"] = config.StringOrStringArray{
		config.CmdToggleCursorFollowSelection,
	}
	cfg.Grid.Hotkeys["Enter"] = config.StringOrStringArray{
		"action save_cursor_pos",
		config.CmdIdle,
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
	cfg.Hints.Hotkeys["gg"] = config.StringOrStringArray{config.CmdLeftClick}
	cfg.Hints.AppConfigs = []config.AppConfig{
		{
			BundleID: config.TestBundleIDSafari,
			Hotkeys: map[string]config.StringOrStringArray{
				"g": {config.CmdLeftClick},
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
	cfg.Scroll.Hotkeys["gg"] = config.StringOrStringArray{config.CmdGoTop}

	err := cfg.ValidateHotkeys()
	if err == nil {
		t.Fatal("ValidateHotkeys() expected prefix conflict error, got nil")
	}
}

func TestConfigValidateHotkeys_SequenceWithoutPrefixConflict(t *testing.T) {
	cfg := config.DefaultConfig()
	// "gg" sequence with no single-key "g" binding should be fine
	cfg.Scroll.Hotkeys["gg"] = config.StringOrStringArray{config.CmdGoTop}
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

func TestConfigValidateScroll_AppConfigs(t *testing.T) {
	cfg := config.DefaultConfig()

	// Valid scroll app configs
	cfg.Scroll.AppConfigs = []config.AppConfig{
		{
			BundleID: config.TestBundleIDSafari,
			Hotkeys: map[string]config.StringOrStringArray{
				"k": {"action scroll_up"},
			},
		},
	}

	err := cfg.ValidateScroll()
	if err != nil {
		t.Fatalf("ValidateScroll() unexpected error: %v", err)
	}

	// Invalid: Empty BundleID
	cfg.Scroll.AppConfigs = []config.AppConfig{
		{
			BundleID: "",
		},
	}

	err = cfg.ValidateScroll()
	if err == nil {
		t.Fatal("ValidateScroll() expected error for empty bundle_id, got nil")
	}

	// Invalid: Duplicate BundleID
	cfg.Scroll.AppConfigs = []config.AppConfig{
		{
			BundleID: config.TestBundleIDSafari,
		},
		{
			BundleID: config.TestBundleIDSafari,
		},
	}

	err = cfg.ValidateScroll()
	if err == nil {
		t.Fatal("ValidateScroll() expected error for duplicate bundle_id, got nil")
	}

	// Invalid: scroll_step = 0 in app config
	zero := 0
	cfg.Scroll.AppConfigs = []config.AppConfig{
		{
			BundleID:   config.TestBundleIDSafari,
			ScrollStep: &zero,
		},
	}

	err = cfg.ValidateScroll()
	if err == nil {
		t.Fatal("ValidateScroll() expected error for scroll_step = 0 in app config, got nil")
	}
}

func TestConfigValidateHints_DuplicateHintChars(t *testing.T) {
	cfg := config.DefaultConfig()

	cfg.Hints.HintCharacters = "ab"

	err := cfg.ValidateHints()
	if err != nil {
		t.Fatalf("ValidateHints() unexpected error for unique chars: %v", err)
	}

	cfg.Hints.HintCharacters = "aab"

	err = cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error for duplicate hint_characters")
	}

	cfg.Hints.HintCharacters = "aba"

	err = cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error for duplicate hint_characters (non-adjacent)")
	}

	cfg.Hints.HintCharacters = "aA"

	err = cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error for case-insensitive duplicate hint_characters")
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

func TestConfigValidateVirtualPointer(t *testing.T) {
	cfg := config.DefaultConfig()

	err := cfg.ValidateVirtualPointer()
	if err != nil {
		t.Fatalf("ValidateVirtualPointer() unexpected error: %v", err)
	}

	cfg.VirtualPointer.UI.Size = 0

	err = cfg.ValidateVirtualPointer()
	if err == nil {
		t.Fatal("ValidateVirtualPointer() expected error for size=0")
	}
}
