package config_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

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
			hotkey:    "Cmd+Space",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "valid multiple modifiers",
			hotkey:    "Cmd+Shift+Space",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "valid all modifiers",
			hotkey:    "Cmd+Ctrl+Alt+Shift+A",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "valid function key",
			hotkey:    "F1",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "valid Option modifier",
			hotkey:    "Option+D",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "valid Primary modifier",
			hotkey:    "Primary+D",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "valid RightCmd modifier",
			hotkey:    "RightCmd+Q",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "valid LeftShift modifier",
			hotkey:    "LeftShift+W",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "valid multiple right-prefixed modifiers",
			hotkey:    "RightCmd+RightCtrl+RightShift+RightOption+R",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "empty hotkey allowed",
			hotkey:    "",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "valid Super modifier",
			hotkey:    "Super+Space",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "valid Meta modifier",
			hotkey:    "Meta+Space",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "empty key",
			hotkey:    "Cmd+",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "just modifiers",
			hotkey:    "Cmd+Shift",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "duplicate modifiers",
			hotkey:    "Cmd+Cmd+Space",
			fieldName: "test_hotkey",
			wantErr:   false, // Duplicates not checked
		},
		{
			name:      "any key allowed",
			hotkey:    "Cmd+InvalidKey",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "single key without modifiers",
			hotkey:    "Space",
			fieldName: "test_hotkey",
			wantErr:   false, // Single keys allowed
		},
		{
			name:      "just plus",
			hotkey:    "+",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "multiple pluses",
			hotkey:    "Cmd++Space",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "leading plus",
			hotkey:    "+Space",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "trailing plus",
			hotkey:    "Space+",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "spaces in hotkey trimmed",
			hotkey:    "Cmd + Space",
			fieldName: "test_hotkey",
			wantErr:   false, // TrimSpace handles spaces
		},
		{
			name:      "lowercase modifiers",
			hotkey:    "cmd+space",
			fieldName: "test_hotkey",
			wantErr:   true, // Must be exact case
		},
		{
			name:      "mixed case modifiers",
			hotkey:    "CMD+shift+Space",
			fieldName: "test_hotkey",
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
					"Cmd+Shift+G": {"grid"},
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
					"Escape": {"idle"},
					"escape": {"hints"},
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
					"Enter":  {"hints"},
					"Return": {"grid"},
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
