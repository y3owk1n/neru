package config_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

const testHotkeyFieldName = "test_hotkey"

// TestValidateHotkey tests the ValidateHotkey function.
func TestValidateHotkey(t *testing.T) {
	tests := []struct {
		name      string
		hotkey    string
		fieldName string
		wantErr   bool
	}{
		{
			name:      "valid single modifier",
			hotkey:    KeyCmdSpace,
			fieldName: testHotkeyFieldName,
			wantErr:   false,
		},
		{
			name:      "valid multiple modifiers",
			hotkey:    "Cmd+Shift+Space",
			fieldName: testHotkeyFieldName,
			wantErr:   false,
		},
		{
			name:      "valid all modifiers",
			hotkey:    "Cmd+Ctrl+Alt+Shift+A",
			fieldName: testHotkeyFieldName,
			wantErr:   false,
		},
		{
			name:      "valid function key",
			hotkey:    "F1",
			fieldName: testHotkeyFieldName,
			wantErr:   false,
		},
		{
			// evdev and Wayland emit Insert, so a binding may name it.
			name:      "valid Insert key",
			hotkey:    "Insert",
			fieldName: testHotkeyFieldName,
			wantErr:   false,
		},
		{
			name:      "valid Insert in a modifier combo",
			hotkey:    "Cmd+Insert",
			fieldName: testHotkeyFieldName,
			wantErr:   false,
		},
		{
			// A modifier-shaped key with per-platform semantics, declined by
			// ADR 0008.
			name:      "CapsLock is not a named key",
			hotkey:    "CapsLock",
			fieldName: testHotkeyFieldName,
			wantErr:   true,
		},
		{
			name:      "valid Option modifier",
			hotkey:    "Option+D",
			fieldName: testHotkeyFieldName,
			wantErr:   false,
		},
		{
			name:      "valid Primary modifier",
			hotkey:    "Primary+D",
			fieldName: testHotkeyFieldName,
			wantErr:   false,
		},
		{
			name:      "valid RightCmd modifier",
			hotkey:    "RightCmd+Q",
			fieldName: testHotkeyFieldName,
			wantErr:   false,
		},
		{
			name:      "valid LeftShift modifier",
			hotkey:    "LeftShift+W",
			fieldName: testHotkeyFieldName,
			wantErr:   false,
		},
		{
			name:      "valid multiple right-prefixed modifiers",
			hotkey:    "RightCmd+RightCtrl+RightShift+RightOption+R",
			fieldName: testHotkeyFieldName,
			wantErr:   false,
		},
		{
			name:      "empty hotkey allowed",
			hotkey:    "",
			fieldName: testHotkeyFieldName,
			wantErr:   true,
		},
		{
			name:      "valid Super modifier",
			hotkey:    KeySuperSpace,
			fieldName: testHotkeyFieldName,
			wantErr:   false,
		},
		{
			name:      "valid Meta modifier",
			hotkey:    "Meta+Space",
			fieldName: testHotkeyFieldName,
			wantErr:   false,
		},
		{
			name:      "empty key",
			hotkey:    "Cmd+",
			fieldName: testHotkeyFieldName,
			wantErr:   true,
		},
		{
			name:      "just modifiers",
			hotkey:    "Cmd+Shift",
			fieldName: testHotkeyFieldName,
			wantErr:   true,
		},
		{
			name:      "duplicate modifiers",
			hotkey:    "Cmd+Cmd+Space",
			fieldName: testHotkeyFieldName,
			wantErr:   false, // Duplicates not checked
		},
		{
			name:      "any key allowed",
			hotkey:    "Cmd+InvalidKey",
			fieldName: testHotkeyFieldName,
			wantErr:   true,
		},
		{
			name:      "single key without modifiers",
			hotkey:    "Space",
			fieldName: testHotkeyFieldName,
			wantErr:   false, // Single keys allowed
		},
		{
			name:      "just plus",
			hotkey:    "+",
			fieldName: testHotkeyFieldName,
			wantErr:   true,
		},
		{
			name:      "multiple pluses",
			hotkey:    "Cmd++Space",
			fieldName: testHotkeyFieldName,
			wantErr:   true,
		},
		{
			name:      "leading plus",
			hotkey:    "+Space",
			fieldName: testHotkeyFieldName,
			wantErr:   true,
		},
		{
			name:      "trailing plus",
			hotkey:    "Space+",
			fieldName: testHotkeyFieldName,
			wantErr:   true,
		},
		{
			name:      "spaces in hotkey trimmed",
			hotkey:    "Cmd + Space",
			fieldName: testHotkeyFieldName,
			wantErr:   false, // TrimSpace handles spaces
		},
		{
			name:      "lowercase modifiers",
			hotkey:    "cmd+space",
			fieldName: testHotkeyFieldName,
			wantErr:   true, // Must be exact case
		},
		{
			name:      "mixed case modifiers",
			hotkey:    "CMD+shift+Space",
			fieldName: testHotkeyFieldName,
			wantErr:   true, // Must be exact case
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := config.ValidateHotkey(testCase.hotkey, testCase.fieldName)
			if (err != nil) != testCase.wantErr {
				t.Errorf("ValidateHotkey() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

// TestValidateHotkeyBindings_DuplicateNormalizedKeys tests that
// ValidateHotkeyBindings detects duplicate keys after normalization.
func TestValidateHotkeyBindings_DuplicateNormalizedKeys(t *testing.T) {
	tests := []struct {
		name    string
		cfg     func() *config.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "no duplicates",
			cfg: func() *config.Config {
				_config := config.DefaultConfig()
				_config.Hotkeys.Bindings = map[string][]string{
					"Cmd+Shift+S": {"scroll"},
					"Cmd+Shift+G": {config.ModeNameGrid},
				}

				return _config
			},
			wantErr: false,
		},
		{
			name: "duplicate named keys different casing",
			cfg: func() *config.Config {
				_config := config.DefaultConfig()
				// Both pass ValidateHotkey (IsValidNamedKey is case-insensitive)
				// but normalize to the same key.
				_config.Hotkeys.Bindings = map[string][]string{
					config.KeyDisplayEscape: {config.CmdIdle},
					config.KeyNameEscape:    {config.ModeNameHints},
				}

				return _config
			},
			wantErr: true,
			errMsg:  "duplicate bindings",
		},
		{
			name: "duplicate via alias Enter and Return",
			cfg: func() *config.Config {
				_config := config.DefaultConfig()
				// "Enter" and "Return" are both valid named keys that
				// normalize to the same canonical form "return".
				_config.Hotkeys.Bindings = map[string][]string{
					"Enter":          {"hints"},
					config.KeyReturn: {config.ModeNameGrid},
				}

				return _config
			},
			wantErr: true,
			errMsg:  "duplicate bindings",
		},
		{
			name: "empty bindings valid",
			cfg: func() *config.Config {
				_config := config.DefaultConfig()
				_config.Hotkeys.Bindings = map[string][]string{}

				return _config
			},
			wantErr: false,
		},
		{
			name: "single binding valid",
			cfg: func() *config.Config {
				c := config.DefaultConfig()
				c.Hotkeys.Bindings = map[string][]string{
					"Cmd+Shift+Space": {"hints"},
				}

				return c
			},
			wantErr: false,
		},
		{
			name:    "default config valid",
			cfg:     config.DefaultConfig,
			wantErr: false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.cfg().ValidateHotkeyBindings()

			if (err != nil) != testCase.wantErr {
				t.Errorf("ValidateHotkeyBindings() error = %v, wantErr %v", err, testCase.wantErr)
			}

			if testCase.wantErr && testCase.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), testCase.errMsg) {
					t.Errorf(
						"ValidateHotkeyBindings() error = %v, want error containing %q",
						err,
						testCase.errMsg,
					)
				}
			}
		})
	}
}

// TestValidateHotkeyBindings_ActionChains tests that comma-separated action
// chains (e.g. "action left_click,left_click") are accepted for mouse button
// actions and rejected otherwise, matching the runtime chain rules.
func TestValidateHotkeyBindings_ActionChains(t *testing.T) {
	tests := []struct {
		name    string
		action  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "double click chain",
			action:  "action left_click,left_click",
			wantErr: false,
		},
		{
			name:    "triple click chain",
			action:  "action left_click,left_click,left_click",
			wantErr: false,
		},
		{
			name:    "mixed button chain",
			action:  "action left_click,right_click",
			wantErr: false,
		},
		{
			name:    "chain with flags",
			action:  "action left_click,left_click --modifier shift",
			wantErr: false,
		},
		{
			name:    "chain with unknown action",
			action:  "action left_click,not_an_action",
			wantErr: true,
			errMsg:  "unknown action subcommand: not_an_action",
		},
		{
			name:    "chain with non mouse button action",
			action:  "action left_click,scroll_down",
			wantErr: true,
			errMsg:  "only mouse button actions are allowed",
		},
		{
			// sleep is a known action but has no executable type, so it must be
			// reported as a chain violation rather than an unknown name.
			name:    "chain with known but non chainable action",
			action:  "action left_click,sleep",
			wantErr: true,
			errMsg:  "sleep cannot be used in an action chain",
		},
		{
			name:    "chain of button hold actions",
			action:  "action left_mouse_down,left_mouse_up",
			wantErr: false,
		},
		{
			name:    "chain of legacy button spellings",
			action:  "action mouse_down,mouse_up",
			wantErr: false,
		},
		{
			name:    "chain with empty element",
			action:  "action left_click,",
			wantErr: true,
			errMsg:  "empty action in comma-separated list",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Hotkeys.Bindings = map[string][]string{
				"Ctrl+1": {testCase.action, config.CmdIdle},
			}

			err := cfg.ValidateHotkeyBindings()

			if (err != nil) != testCase.wantErr {
				t.Errorf("ValidateHotkeyBindings() error = %v, wantErr %v", err, testCase.wantErr)
			}

			if testCase.wantErr && testCase.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), testCase.errMsg) {
					t.Errorf(
						"ValidateHotkeyBindings() error = %v, want error containing %q",
						err,
						testCase.errMsg,
					)
				}
			}
		})
	}
}

func TestValidateHotkeys_MoveCell(t *testing.T) {
	tests := []struct {
		name    string
		action  string
		wantErr bool
		errMsg  string
	}{
		{
			name:   "equals form",
			action: "action move_cell --direction=right",
		},
		{
			name:   "space form",
			action: "action move_cell --direction left",
		},
		{
			name:   "with count",
			action: "action move_cell --direction=up --count=2",
		},
		{
			// Direction values are validated by the daemon when the action
			// runs, not by config validation, which only knows action names.
			name:   "unknown direction passes config validation",
			action: "action move_cell --direction=sideways",
		},
		{
			name:    "typo in the action name",
			action:  "action move_cel --direction=right",
			wantErr: true,
			errMsg:  "unknown action subcommand: move_cel",
		},
		{
			name:    "not chainable",
			action:  "action left_click,move_cell",
			wantErr: true,
			errMsg:  "move_cell cannot be used in an action chain",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.RecursiveGrid.Hotkeys = map[string]config.StringOrStringArray{
				"Right": {testCase.action},
			}

			err := cfg.ValidateHotkeys()

			if (err != nil) != testCase.wantErr {
				t.Errorf("ValidateHotkeys() error = %v, wantErr %v", err, testCase.wantErr)
			}

			if testCase.wantErr && testCase.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), testCase.errMsg) {
					t.Errorf(
						"ValidateHotkeys() error = %v, want error containing %q",
						err,
						testCase.errMsg,
					)
				}
			}
		})
	}
}
