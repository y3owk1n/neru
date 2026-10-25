package config_test

import (
	"maps"
	"math"
	"strings"
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

// "run" carries its own steps as quoted arguments, so a binding that uses it
// has to validate like any other command rather than being rejected as
// unknown.
func TestConfigValidateHotkeys_RunSequence(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.Hotkeys["Enter"] = config.StringOrStringArray{
		"run 'action left_click' 'action sleep 0.2' 'action restore_cursor_pos'",
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
			BundleID: TestBundleIDSafari,
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

func TestConfigValidateHotkeys_BisectAppOverridePrefixConflict(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Bisect.AppConfigs = []config.AppConfig{
		{
			BundleID: TestBundleIDSafari,
			Hotkeys: map[string]config.StringOrStringArray{
				// Shadowed by the inherited single-key "y" binding.
				"yu": {config.CmdLeftClick},
			},
		},
	}

	err := cfg.ValidateHotkeys()
	if err == nil {
		t.Fatal("ValidateHotkeys() expected merged bisect app override prefix conflict, got nil")
	}
}

func TestConfigValidateHotkeys_PrefixConflictInEveryTable(t *testing.T) {
	conflict := func() map[string]config.StringOrStringArray {
		return map[string]config.StringOrStringArray{
			"g":  {config.CmdIdle},
			"gg": {config.CmdIdle},
		}
	}
	appConfigs := func() []config.AppConfig {
		return []config.AppConfig{{BundleID: TestBundleIDSafari, Hotkeys: conflict()}}
	}
	appOverride := func(key string) []config.AppConfig {
		return []config.AppConfig{{
			BundleID: TestBundleIDSafari,
			Hotkeys:  map[string]config.StringOrStringArray{key: {config.CmdIdle}},
		}}
	}

	tests := []struct {
		name   string
		field  string
		mutate func(cfg *config.Config)
	}{
		{
			name:   "grid app_configs",
			field:  "grid.app_configs[0]",
			mutate: func(cfg *config.Config) { cfg.Grid.AppConfigs = appConfigs() },
		},
		{
			name:   "recursive_grid app_configs",
			field:  "recursive_grid.app_configs[0]",
			mutate: func(cfg *config.Config) { cfg.RecursiveGrid.AppConfigs = appConfigs() },
		},
		{
			name:  "grid base sequence and app single key",
			field: "grid.hotkeys merged with grid.app_configs[0]",
			mutate: func(cfg *config.Config) {
				cfg.Grid.Hotkeys["gg"] = config.StringOrStringArray{config.CmdIdle}
				cfg.Grid.AppConfigs = appOverride("g")
			},
		},
		{
			name:  "grid base single key and app sequence",
			field: "grid.hotkeys merged with grid.app_configs[0]",
			mutate: func(cfg *config.Config) {
				cfg.Grid.Hotkeys["g"] = config.StringOrStringArray{config.CmdIdle}
				cfg.Grid.AppConfigs = appOverride("gg")
			},
		},
		{
			name:  "recursive_grid base sequence and app single key",
			field: "recursive_grid.hotkeys merged with recursive_grid.app_configs[0]",
			mutate: func(cfg *config.Config) {
				cfg.RecursiveGrid.Hotkeys["gg"] = config.StringOrStringArray{config.CmdIdle}
				cfg.RecursiveGrid.AppConfigs = appOverride("g")
			},
		},
		{
			name:  "recursive_grid base single key and app sequence",
			field: "recursive_grid.hotkeys merged with recursive_grid.app_configs[0]",
			mutate: func(cfg *config.Config) {
				cfg.RecursiveGrid.Hotkeys["g"] = config.StringOrStringArray{config.CmdIdle}
				cfg.RecursiveGrid.AppConfigs = appOverride("gg")
			},
		},
		{
			name:   "monitor_select hotkeys",
			field:  "monitor_select.hotkeys",
			mutate: func(cfg *config.Config) { maps.Copy(cfg.MonitorSelect.Hotkeys, conflict()) },
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			testCase.mutate(cfg)

			err := cfg.ValidateHotkeys()
			if err == nil || !strings.Contains(err.Error(), testCase.field) ||
				!strings.Contains(err.Error(), "prefix conflict") {
				t.Fatalf(
					"ValidateHotkeys() = %v, want a prefix conflict in %s",
					err,
					testCase.field,
				)
			}
		})
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
			BundleID: TestBundleIDSafari,
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
			BundleID: TestBundleIDSafari,
		},
		{
			BundleID: TestBundleIDSafari,
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
			BundleID:   TestBundleIDSafari,
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

	err := cfg.ValidateHints(nil)
	if err != nil {
		t.Fatalf("ValidateHints() unexpected error for unique chars: %v", err)
	}

	cfg.Hints.HintCharacters = "aab"

	err = cfg.ValidateHints(nil)
	if err == nil {
		t.Fatal("ValidateHints() expected error for duplicate hint_characters")
	}

	cfg.Hints.HintCharacters = "aba"

	err = cfg.ValidateHints(nil)
	if err == nil {
		t.Fatal("ValidateHints() expected error for duplicate hint_characters (non-adjacent)")
	}

	cfg.Hints.HintCharacters = "aA"

	err = cfg.ValidateHints(nil)
	if err == nil {
		t.Fatal("ValidateHints() expected error for case-insensitive duplicate hint_characters")
	}
}

// TestConfigValidateVirtualPointer pins the refusals. The default char is a
// multi-byte rune, so validBase already proves the validator counts one rune
// as one character, not by its bytes.
func TestConfigValidateVirtualPointer(t *testing.T) {
	for _, char := range []string{"", "ab"} {
		cfg := validBase(t)
		cfg.VirtualPointer.UI.Char = char

		assertRejected(t, cfg.ValidateVirtualPointer(), "virtual_pointer.ui.char", char)
	}

	runIntBounds(t, []intBound{{
		name:     "virtual_pointer.ui.font_size",
		set:      func(c *config.Config, v int) { c.VirtualPointer.UI.FontSize = v },
		validate: (*config.Config).ValidateVirtualPointer,
		minValid: 0,
		maxValid: math.MaxInt32,
	}})
}
