package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// testModifierBackspaceKey is a shared test constant for modifier-combo backspace key scenarios.
const testModifierBackspaceKey = "Ctrl+H"

// testActionConflictKey is a shared test constant for backspace vs action key binding conflict scenarios.
const testActionConflictKey = "Shift+L"

// testResetKeyConflictBinding is a shared test constant for reset key vs action key binding conflict scenarios.
const testResetKeyConflictBinding = "Space"

// TestConfig_ValidateHints tests the Config.ValidateHints method.
func TestConfig_ValidateHints(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		wantErr bool
	}{
		{
			name: "valid hints config",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters: "abcd",
					UI: config.HintsUI{
						BackgroundColorLight:  "#FFFFFF",
						BackgroundColorDark:   "#FFFFFF",
						TextColorLight:        "#000000",
						TextColorDark:         "#000000",
						MatchedTextColorLight: "#FF0000",
						MatchedTextColorDark:  "#FF0000",
						BorderColorLight:      "#000000",
						BorderColorDark:       "#000000",
						FontSize:              12,
						BorderRadius:          4,
						PaddingX:              4,
						PaddingY:              4,
						BorderWidth:           1,
					},
					ClickableRoles:    []string{"AXButton"},
					ParallelThreshold: 20,
				},
			},
			wantErr: false,
		},
		{
			name: "hints with empty characters - invalid",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters: "", // Invalid
				},
			},
			wantErr: true,
		},
		{
			name: "hints enabled but invalid",
			config: config.Config{
				Hints: config.HintsConfig{
					Enabled:        true,
					HintCharacters: "", // Invalid
				},
			},
			wantErr: true,
		},
		{
			name: "hints with unicode characters - invalid",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters: "aé😀", // Invalid - contains Unicode
				},
			},
			wantErr: true,
		},

		{
			name: "hints backspace_key conflicts with hint_characters - invalid",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters: "abcd",
					BackspaceKey:   "a", // Conflicts with hint_characters
				},
			},
			wantErr: true,
		},
		{
			name: "hints backspace_key case-insensitive conflict - invalid",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters: "ABCD",
					BackspaceKey:   "a", // Conflicts (case-insensitive)
				},
			},
			wantErr: true,
		},
		{
			name: "hints backspace_key named key 'space' conflicts with space in hint_characters - invalid",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters: "ab cd", // Contains space character
					BackspaceKey:   "space", // Named key resolves to space
				},
			},
			wantErr: true,
		},
		{
			name: "hints backspace_key no conflict",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters: "abcd",
					BackspaceKey:   "x", // No conflict
					UI: config.HintsUI{
						BackgroundColorLight:  "#FFFFFF",
						BackgroundColorDark:   "#FFFFFF",
						TextColorLight:        "#000000",
						TextColorDark:         "#000000",
						MatchedTextColorLight: "#FF0000",
						MatchedTextColorDark:  "#FF0000",
						BorderColorLight:      "#000000",
						BorderColorDark:       "#000000",
						FontSize:              12,
						BorderRadius:          4,
						PaddingX:              4,
						PaddingY:              4,
						BorderWidth:           1,
					},
					ClickableRoles:    []string{"AXButton"},
					ParallelThreshold: 20,
				},
			},
			wantErr: false,
		},
		{
			name: "hints backspace_key modifier combo no conflict with characters",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters: "abcd",
					BackspaceKey:   testModifierBackspaceKey, // Modifier combo, no conflict
					UI: config.HintsUI{
						BackgroundColorLight:  "#FFFFFF",
						BackgroundColorDark:   "#FFFFFF",
						TextColorLight:        "#000000",
						TextColorDark:         "#000000",
						MatchedTextColorLight: "#FF0000",
						MatchedTextColorDark:  "#FF0000",
						BorderColorLight:      "#000000",
						BorderColorDark:       "#000000",
						FontSize:              12,
						BorderRadius:          4,
						PaddingX:              4,
						PaddingY:              4,
						BorderWidth:           1,
					},
					ClickableRoles:    []string{"AXButton"},
					ParallelThreshold: 20,
				},
			},
			wantErr: false,
		},
		{
			name: "hints backspace_key empty string uses default - valid",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters: "abcd",
					BackspaceKey:   "", // Empty = default backspace/delete
					UI: config.HintsUI{
						BackgroundColorLight:  "#FFFFFF",
						BackgroundColorDark:   "#FFFFFF",
						TextColorLight:        "#000000",
						TextColorDark:         "#000000",
						MatchedTextColorLight: "#FF0000",
						MatchedTextColorDark:  "#FF0000",
						BorderColorLight:      "#000000",
						BorderColorDark:       "#000000",
						FontSize:              12,
						BorderRadius:          4,
						PaddingX:              4,
						PaddingY:              4,
						BorderWidth:           1,
					},
					ClickableRoles:    []string{"AXButton"},
					ParallelThreshold: 20,
				},
			},
			wantErr: false,
		},
		{
			name: "hints with valid ASCII digits and symbols",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters: "123!@#", // Valid - ASCII digits and symbols
					UI: config.HintsUI{
						BackgroundColorLight:  "#FFFFFF",
						BackgroundColorDark:   "#FFFFFF",
						TextColorLight:        "#000000",
						TextColorDark:         "#000000",
						MatchedTextColorLight: "#FF0000",
						MatchedTextColorDark:  "#FF0000",
						BorderColorLight:      "#000000",
						BorderColorDark:       "#000000",
						FontSize:              12,
						BorderRadius:          4,
						PaddingX:              4,
						PaddingY:              4,
						BorderWidth:           1,
					},
					ClickableRoles:    []string{"AXButton"},
					ParallelThreshold: 20,
				},
			},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.config.ValidateHints()
			if (err != nil) != testCase.wantErr {
				t.Errorf("Config.ValidateHints() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

// TestConfig_ValidateAppConfigs tests the Config.ValidateAppConfigs method.
func TestConfig_ValidateAppConfigs(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		wantErr bool
	}{
		{
			name: "valid app configs",
			config: config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:             "com.example.app",
							AdditionalClickable:  []string{"AXButton", "AXLink"},
							IgnoreClickableCheck: false,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty bundle ID",
			config: config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:            "",
							AdditionalClickable: []string{"AXButton"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty clickable roles in slice",
			config: config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:            "com.example.app",
							AdditionalClickable: []string{"AXButton", ""},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate bundle IDs allowed",
			config: config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:            "com.example.app",
							AdditionalClickable: []string{"AXButton"},
						},
						{
							BundleID:            "com.example.app", // Duplicate allowed
							AdditionalClickable: []string{"AXLink"},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.config.ValidateAppConfigs()
			if (err != nil) != testCase.wantErr {
				t.Errorf(
					"Config.ValidateAppConfigs() error = %v, wantErr %v",
					err,
					testCase.wantErr,
				)
			}
		})
	}
}

// TestConfig_ValidateGrid tests the Config.ValidateGrid method.
func TestConfig_ValidateGrid(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		wantErr bool
	}{
		{
			name: "valid grid config",
			config: config.Config{
				Grid: config.GridConfig{
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					UI: config.GridUI{
						BackgroundColorLight:        "#FF0000",
						BackgroundColorDark:         "#FF0000",
						TextColorLight:              "#FFFFFF",
						TextColorDark:               "#FFFFFF",
						MatchedTextColorLight:       "#000000",
						MatchedTextColorDark:        "#000000",
						MatchedBackgroundColorLight: "#333333",
						MatchedBackgroundColorDark:  "#333333",
						MatchedBorderColorLight:     "#FF0000",
						MatchedBorderColorDark:      "#FF0000",
						BorderColorLight:            "#666666",
						BorderColorDark:             "#666666",
						FontSize:                    14,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "grid with empty characters - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Characters: "", // Invalid
				},
			},
			wantErr: true,
		},
		{
			name: "grid enabled but invalid characters",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:    true,
					Characters: "A", // Too short
				},
			},
			wantErr: true,
		},
		{
			name: "invalid color",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:    true,
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					UI: config.GridUI{
						BackgroundColorLight: "invalid",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "grid with reserved reset character",
			config: config.Config{
				Grid: config.GridConfig{
					Characters: "ABC DEF", // Contains ' ' (space, the default reset key)
				},
			},
			wantErr: true,
		},
		{
			name: "grid with unicode characters - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Characters: "abcé😀", // Invalid - contains Unicode
				},
			},
			wantErr: true,
		},
		{
			name: "grid with valid row_labels",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:    true,
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					RowLabels:  "123456789",
					UI: config.GridUI{
						FontSize:                    12,
						BackgroundColorLight:        "#ffffff",
						BackgroundColorDark:         "#ffffff",
						TextColorLight:              "#000000",
						TextColorDark:               "#000000",
						MatchedTextColorLight:       "#000000",
						MatchedTextColorDark:        "#000000",
						MatchedBackgroundColorLight: "#ffffff",
						MatchedBackgroundColorDark:  "#ffffff",
						MatchedBorderColorLight:     "#000000",
						MatchedBorderColorDark:      "#000000",
						BorderColorLight:            "#000000",
						BorderColorDark:             "#000000",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "grid with valid col_labels",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:    true,
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					ColLabels:  "abcdefghij",
					UI: config.GridUI{
						FontSize:                    12,
						BackgroundColorLight:        "#ffffff",
						BackgroundColorDark:         "#ffffff",
						TextColorLight:              "#000000",
						TextColorDark:               "#000000",
						MatchedTextColorLight:       "#000000",
						MatchedTextColorDark:        "#000000",
						MatchedBackgroundColorLight: "#ffffff",
						MatchedBackgroundColorDark:  "#ffffff",
						MatchedBorderColorLight:     "#000000",
						MatchedBorderColorDark:      "#000000",
						BorderColorLight:            "#000000",
						BorderColorDark:             "#000000",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "grid with too short row_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:    true,
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					RowLabels:  "1", // Too short
					UI: config.GridUI{
						FontSize:                    12,
						BackgroundColorLight:        "#ffffff",
						TextColorLight:              "#000000",
						MatchedTextColorLight:       "#000000",
						MatchedBackgroundColorLight: "#ffffff",
						MatchedBorderColorLight:     "#000000",
						BorderColorLight:            "#000000",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "grid with too short col_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:    true,
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					ColLabels:  "a", // Too short
					UI: config.GridUI{
						FontSize:                    12,
						BackgroundColorLight:        "#ffffff",
						TextColorLight:              "#000000",
						MatchedTextColorLight:       "#000000",
						MatchedBackgroundColorLight: "#ffffff",
						MatchedBorderColorLight:     "#000000",
						BorderColorLight:            "#000000",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "grid with reserved character in row_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:    true,
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					RowLabels:  "123 456", // Contains ' '
					UI: config.GridUI{
						FontSize:                    12,
						BackgroundColorLight:        "#ffffff",
						BackgroundColorDark:         "#ffffff",
						TextColorLight:              "#000000",
						TextColorDark:               "#000000",
						MatchedTextColorLight:       "#000000",
						MatchedTextColorDark:        "#000000",
						MatchedBackgroundColorLight: "#ffffff",
						MatchedBackgroundColorDark:  "#ffffff",
						MatchedBorderColorLight:     "#000000",
						MatchedBorderColorDark:      "#000000",
						BorderColorLight:            "#ffffff",
						BorderColorDark:             "#ffffff",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "grid with reserved character in col_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:    true,
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					ColLabels:  "abc def", // Contains ' '
					UI: config.GridUI{
						FontSize:                    12,
						BackgroundColorLight:        "#ffffff",
						TextColorLight:              "#000000",
						MatchedTextColorLight:       "#000000",
						MatchedBackgroundColorLight: "#ffffff",
						MatchedBorderColorLight:     "#000000",
						BorderColorLight:            "#ffffff",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "grid with unicode in row_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:    true,
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					RowLabels:  "123é456", // Contains Unicode
					UI: config.GridUI{
						FontSize:                    12,
						BackgroundColorLight:        "#ffffff",
						BackgroundColorDark:         "#ffffff",
						TextColorLight:              "#000000",
						TextColorDark:               "#000000",
						MatchedTextColorLight:       "#000000",
						MatchedTextColorDark:        "#000000",
						MatchedBackgroundColorLight: "#ffffff",
						MatchedBackgroundColorDark:  "#ffffff",
						MatchedBorderColorLight:     "#000000",
						MatchedBorderColorDark:      "#000000",
						BorderColorLight:            "#000000",
						BorderColorDark:             "#000000",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "grid with unicode in col_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:    true,
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					ColLabels:  "abc😀def", // Contains Unicode
					UI: config.GridUI{
						FontSize:                    12,
						BackgroundColorLight:        "#ffffff",
						BackgroundColorDark:         "#ffffff",
						TextColorLight:              "#000000",
						TextColorDark:               "#000000",
						MatchedTextColorLight:       "#000000",
						MatchedTextColorDark:        "#000000",
						MatchedBackgroundColorLight: "#ffffff",
						MatchedBackgroundColorDark:  "#ffffff",
						MatchedBorderColorLight:     "#000000",
						MatchedBorderColorDark:      "#000000",
						BorderColorLight:            "#000000",
						BorderColorDark:             "#000000",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "grid with duplicate characters in sublayer_keys - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:      true,
					Characters:   "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					SublayerKeys: "aaabbbccc", // Contains duplicate 'a', 'b', 'c'
					UI: config.GridUI{
						FontSize:                    12,
						BackgroundColorLight:        "#ffffff",
						TextColorLight:              "#000000",
						MatchedTextColorLight:       "#000000",
						MatchedBackgroundColorLight: "#ffffff",
						MatchedBorderColorLight:     "#000000",
						BorderColorLight:            "#000000",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "grid with duplicate characters - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:    true,
					Characters: "AaaBCDEF", // Contains duplicate 'A' and 'a'
					UI: config.GridUI{
						FontSize:                    12,
						BackgroundColorLight:        "#ffffff",
						TextColorLight:              "#000000",
						MatchedTextColorLight:       "#000000",
						MatchedBackgroundColorLight: "#ffffff",
						MatchedBorderColorLight:     "#000000",
						BorderColorLight:            "#000000",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "grid with duplicate characters in row_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:    true,
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					RowLabels:  "1123456789", // Contains duplicate '1'
					UI: config.GridUI{
						FontSize:                    12,
						BackgroundColorLight:        "#ffffff",
						TextColorLight:              "#000000",
						MatchedTextColorLight:       "#000000",
						MatchedBackgroundColorLight: "#ffffff",
						MatchedBorderColorLight:     "#000000",
						BorderColorLight:            "#000000",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "grid with duplicate characters in col_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:    true,
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					ColLabels:  "aabbcdef", // Contains duplicate 'a' and 'b'
					UI: config.GridUI{
						FontSize:                    12,
						BackgroundColorLight:        "#ffffff",
						TextColorLight:              "#000000",
						MatchedTextColorLight:       "#000000",
						MatchedBackgroundColorLight: "#ffffff",
						MatchedBorderColorLight:     "#000000",
						BorderColorLight:            "#000000",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "grid with valid ASCII digits and symbols",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:   "123!@#",    // Valid - ASCII digits and symbols
					SublayerKeys: "abcdefghi", // Required for subgrid
					UI: config.GridUI{
						BackgroundColorLight:        "#FF0000",
						BackgroundColorDark:         "#FF0000",
						TextColorLight:              "#FFFFFF",
						TextColorDark:               "#FFFFFF",
						MatchedTextColorLight:       "#000000",
						MatchedTextColorDark:        "#000000",
						MatchedBackgroundColorLight: "#333333",
						MatchedBackgroundColorDark:  "#333333",
						MatchedBorderColorLight:     "#FF0000",
						MatchedBorderColorDark:      "#FF0000",
						BorderColorLight:            "#666666",
						BorderColorDark:             "#666666",
						FontSize:                    14,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "grid with sublayer_keys containing unicode - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:   "ABC",
					SublayerKeys: "abcdefg-é", // Invalid - contains Unicode
				},
			},
			wantErr: true,
		},
		{
			name: "grid with sublayer_keys containing reserved character - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:   "ABC",
					SublayerKeys: "abcdefg h", // Invalid - contains ' ' (space, the default reset key)
				},
			},
			wantErr: true,
		},
		{
			name: "grid backspace_key conflicts with characters - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:   "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					BackspaceKey: "a", // Conflicts with characters
				},
			},
			wantErr: true,
		},
		{
			name: "grid backspace_key named key 'tab' conflicts with tab in characters - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:   "AB\tCDEFGHIJKLMNOPQRSTUVWXYZ",
					BackspaceKey: "tab", // Named key resolves to \t
				},
			},
			wantErr: true,
		},
		{
			name: "grid backspace_key conflicts with row_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:   "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					RowLabels:    "123456789",
					BackspaceKey: "1", // Conflicts with row_labels
					UI: config.GridUI{
						FontSize: 12,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "grid backspace_key conflicts with col_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:   "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					ColLabels:    "123456789x",
					BackspaceKey: "x", // Conflicts with col_labels
					UI: config.GridUI{
						FontSize: 12,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "grid backspace_key conflicts with sublayer_keys - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:   "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					SublayerKeys: "123456789",
					BackspaceKey: "1", // Conflicts with sublayer_keys
					UI: config.GridUI{
						FontSize: 12,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "grid backspace_key no conflict",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:   "ABCDEFGHIJKLMNOPQRSTUVWXY",
					BackspaceKey: "z", // Not in characters
					UI: config.GridUI{
						BackgroundColorLight:        "#FF0000",
						BackgroundColorDark:         "#FF0000",
						TextColorLight:              "#FFFFFF",
						TextColorDark:               "#FFFFFF",
						MatchedTextColorLight:       "#000000",
						MatchedTextColorDark:        "#000000",
						MatchedBackgroundColorLight: "#333333",
						MatchedBackgroundColorDark:  "#333333",
						MatchedBorderColorLight:     "#FF0000",
						MatchedBorderColorDark:      "#FF0000",
						BorderColorLight:            "#666666",
						BorderColorDark:             "#666666",
						FontSize:                    14,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "grid backspace_key modifier combo no conflict",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:   "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					BackspaceKey: testModifierBackspaceKey, // Modifier combo, no conflict
					UI: config.GridUI{
						BackgroundColorLight:        "#FF0000",
						BackgroundColorDark:         "#FF0000",
						TextColorLight:              "#FFFFFF",
						TextColorDark:               "#FFFFFF",
						MatchedTextColorLight:       "#000000",
						MatchedTextColorDark:        "#000000",
						MatchedBackgroundColorLight: "#333333",
						MatchedBackgroundColorDark:  "#333333",
						MatchedBorderColorLight:     "#FF0000",
						MatchedBorderColorDark:      "#FF0000",
						BorderColorLight:            "#666666",
						BorderColorDark:             "#666666",
						FontSize:                    14,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "grid modifier combo reset_key conflicts with same modifier combo backspace_key - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:   "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					ResetKey:     testModifierBackspaceKey,
					BackspaceKey: testModifierBackspaceKey, // Same as reset_key
				},
			},
			wantErr: true,
		},
		{
			name: "negative font size",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:    true,
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					UI: config.GridUI{
						FontSize: -1,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid grid config with auto_exit_actions",
			config: config.Config{
				Grid: config.GridConfig{
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					UI: config.GridUI{
						BackgroundColorLight:        "#FF0000",
						BackgroundColorDark:         "#FF0000",
						TextColorLight:              "#FFFFFF",
						TextColorDark:               "#FFFFFF",
						MatchedTextColorLight:       "#000000",
						MatchedTextColorDark:        "#000000",
						MatchedBackgroundColorLight: "#333333",
						MatchedBackgroundColorDark:  "#333333",
						MatchedBorderColorLight:     "#FF0000",
						MatchedBorderColorDark:      "#FF0000",
						BorderColorLight:            "#666666",
						BorderColorDark:             "#666666",
						FontSize:                    14,
					},
					AutoExitActions: []string{"left_click", "right_click"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid grid config with empty auto_exit_actions",
			config: config.Config{
				Grid: config.GridConfig{
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					UI: config.GridUI{
						BackgroundColorLight:        "#FF0000",
						BackgroundColorDark:         "#FF0000",
						TextColorLight:              "#FFFFFF",
						TextColorDark:               "#FFFFFF",
						MatchedTextColorLight:       "#000000",
						MatchedTextColorDark:        "#000000",
						MatchedBackgroundColorLight: "#333333",
						MatchedBackgroundColorDark:  "#333333",
						MatchedBorderColorLight:     "#FF0000",
						MatchedBorderColorDark:      "#FF0000",
						BorderColorLight:            "#666666",
						BorderColorDark:             "#666666",
						FontSize:                    14,
					},
					AutoExitActions: []string{},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid grid auto_exit_actions with unknown action",
			config: config.Config{
				Grid: config.GridConfig{
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					UI: config.GridUI{
						BackgroundColorLight:        "#FF0000",
						BackgroundColorDark:         "#FF0000",
						TextColorLight:              "#FFFFFF",
						TextColorDark:               "#FFFFFF",
						MatchedTextColorLight:       "#000000",
						MatchedTextColorDark:        "#000000",
						MatchedBackgroundColorLight: "#333333",
						MatchedBackgroundColorDark:  "#333333",
						MatchedBorderColorLight:     "#FF0000",
						MatchedBorderColorDark:      "#FF0000",
						BorderColorLight:            "#666666",
						BorderColorDark:             "#666666",
						FontSize:                    14,
					},
					AutoExitActions: []string{"unknown_action"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid grid auto_exit_actions with scroll (IPC-only)",
			config: config.Config{
				Grid: config.GridConfig{
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					UI: config.GridUI{
						BackgroundColorLight:        "#FF0000",
						BackgroundColorDark:         "#FF0000",
						TextColorLight:              "#FFFFFF",
						TextColorDark:               "#FFFFFF",
						MatchedTextColorLight:       "#000000",
						MatchedTextColorDark:        "#000000",
						MatchedBackgroundColorLight: "#333333",
						MatchedBackgroundColorDark:  "#333333",
						MatchedBorderColorLight:     "#FF0000",
						MatchedBorderColorDark:      "#FF0000",
						BorderColorLight:            "#666666",
						BorderColorDark:             "#666666",
						FontSize:                    14,
					},
					AutoExitActions: []string{"scroll"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid grid auto_exit_actions with move_mouse (IPC-only)",
			config: config.Config{
				Grid: config.GridConfig{
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					UI: config.GridUI{
						BackgroundColorLight:        "#FF0000",
						BackgroundColorDark:         "#FF0000",
						TextColorLight:              "#FFFFFF",
						TextColorDark:               "#FFFFFF",
						MatchedTextColorLight:       "#000000",
						MatchedTextColorDark:        "#000000",
						MatchedBackgroundColorLight: "#333333",
						MatchedBackgroundColorDark:  "#333333",
						MatchedBorderColorLight:     "#FF0000",
						MatchedBorderColorDark:      "#FF0000",
						BorderColorLight:            "#666666",
						BorderColorDark:             "#666666",
						FontSize:                    14,
					},
					AutoExitActions: []string{"move_mouse"},
				},
			},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.config.ValidateGrid()
			if (err != nil) != testCase.wantErr {
				t.Errorf("Config.ValidateGrid() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

// TestConfig_ValidateModeExitKeys_ResetKeyConflicts tests that mode exit keys
// cannot conflict with grid or recursive-grid reset keys.
func TestConfig_ValidateModeExitKeys_ResetKeyConflicts(t *testing.T) {
	tests := []struct {
		name    string
		config  func() config.Config
		wantErr bool
	}{
		{
			name: "space exit key conflicts with default grid reset key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.General.ModeExitKeys = []string{"escape", "space"}
				// Grid.ResetKey defaults to " " (space)
				return cfg
			},
			wantErr: true,
		},
		{
			name: "literal space exit key rejected as empty after trim",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.General.ModeExitKeys = []string{"escape", " "}

				return cfg
			},
			wantErr: true,
		},
		{
			name: "space exit key conflicts with default recursive-grid reset key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.General.ModeExitKeys = []string{"escape", "space"}
				cfg.Grid.ResetKey = "," // Avoid grid conflict to test recursive-grid

				return cfg
			},
			wantErr: true,
		},
		{
			name: "no conflict when grid mode is disabled",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.General.ModeExitKeys = []string{"escape", "space"}
				cfg.Grid.Enabled = false
				cfg.RecursiveGrid.ResetKey = "," // Avoid recursive-grid conflict

				return cfg
			},
			wantErr: false,
		},
		{
			name: "no conflict when recursive-grid mode is disabled",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.General.ModeExitKeys = []string{"escape", "space"}
				cfg.Grid.ResetKey = ","           // Avoid grid conflict
				cfg.RecursiveGrid.Enabled = false // Disabled, so no conflict

				return cfg
			},
			wantErr: false,
		},
		{
			name: "comma exit key conflicts with custom grid reset key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.General.ModeExitKeys = []string{"escape", ","}
				cfg.Grid.ResetKey = ","
				cfg.RecursiveGrid.ResetKey = "."

				return cfg
			},
			wantErr: true,
		},
		{
			name: "no conflict with modifier combo reset key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.General.ModeExitKeys = []string{"escape", "space"}
				cfg.Grid.ResetKey = "Ctrl+R"
				cfg.RecursiveGrid.ResetKey = "Ctrl+R"

				return cfg
			},
			wantErr: false,
		},
		{
			name: "no conflict when exit keys do not match reset key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.General.ModeExitKeys = []string{"escape"}
				// Default reset key is space, no space in exit keys
				return cfg
			},
			wantErr: false,
		},
		{
			name: "exit key conflicts with custom hints backspace_key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.General.ModeExitKeys = []string{"escape", "x"}
				cfg.Hints.BackspaceKey = "x"

				return cfg
			},
			wantErr: true,
		},
		{
			name: "exit key conflicts with custom grid backspace_key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.General.ModeExitKeys = []string{"escape", "x"}
				cfg.Grid.BackspaceKey = "x"

				return cfg
			},
			wantErr: true,
		},
		{
			name: "exit key conflicts with custom recursive_grid backspace_key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.General.ModeExitKeys = []string{"escape", "x"}
				cfg.RecursiveGrid.BackspaceKey = "x"

				return cfg
			},
			wantErr: true,
		},
		{
			name: "exit key 'backspace' conflicts with default backspace_key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.General.ModeExitKeys = []string{"escape", "backspace"}
				cfg.Hints.BackspaceKey = "" // default

				return cfg
			},
			wantErr: true,
		},
		{
			name: "exit key 'delete' conflicts with default backspace_key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.General.ModeExitKeys = []string{"escape", "delete"}
				cfg.Hints.BackspaceKey = "" // default

				return cfg
			},
			wantErr: true,
		},
		{
			name: "no conflict when backspace_key is modifier combo",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.General.ModeExitKeys = []string{"escape", "Ctrl+C"}
				cfg.Hints.BackspaceKey = testModifierBackspaceKey // modifier combo won't conflict
				cfg.Grid.BackspaceKey = testModifierBackspaceKey
				cfg.RecursiveGrid.BackspaceKey = testModifierBackspaceKey

				return cfg
			},
			wantErr: false,
		},
		{
			name: "no conflict when mode with conflicting backspace_key is disabled",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.General.ModeExitKeys = []string{"escape", "x"}
				cfg.Hints.Enabled = false
				cfg.Hints.BackspaceKey = "x" // would conflict, but hints is disabled
				// Remove "x" from grid characters/sublayer_keys to avoid exit key vs characters conflict
				cfg.Grid.Characters = "abcdefghijklmnpqrstuvwy"
				cfg.Grid.SublayerKeys = "abcdefghijklmnpqrstuvwy"
				cfg.Grid.BackspaceKey = "backspace"
				cfg.RecursiveGrid.BackspaceKey = "backspace"

				return cfg
			},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := testCase.config()

			err := cfg.ValidateModeExitKeys()
			if (err != nil) != testCase.wantErr {
				t.Errorf(
					"Config.ValidateModeExitKeys() error = %v, wantErr %v",
					err,
					testCase.wantErr,
				)
			}
		})
	}
}

// TestConfig_ValidateAction tests the Config.ValidateAction method.
func TestConfig_ValidateAction(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		wantErr bool
	}{
		{
			name: "valid action config",
			config: config.Config{
				Action: config.ActionConfig{
					MoveMouseStep: 10,
					KeyBindings: config.ActionKeyBindingsCfg{
						LeftClick: "Cmd+L",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid empty action config",
			config: config.Config{
				Action: config.ActionConfig{
					MoveMouseStep: 10,
				},
			},
			wantErr: false,
		},
		{
			name: "valid action config with positive step",
			config: config.Config{
				Action: config.ActionConfig{
					MoveMouseStep: 10,
					KeyBindings: config.ActionKeyBindingsCfg{
						LeftClick: "Cmd+L",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "zero step is invalid",
			config: config.Config{
				Action: config.ActionConfig{
					MoveMouseStep: 0,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid negative step",
			config: config.Config{
				Action: config.ActionConfig{
					MoveMouseStep: -5,
				},
			},
			wantErr: true,
		},
		{
			name: "valid single lowercase key binding",
			config: config.Config{
				Action: config.ActionConfig{
					MoveMouseStep: 10,
					KeyBindings: config.ActionKeyBindingsCfg{
						MoveMouseUp: "w", // lowercase single char is valid
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid multi-char key binding",
			config: config.Config{
				Action: config.ActionConfig{
					MoveMouseStep: 10,
					KeyBindings: config.ActionKeyBindingsCfg{
						LeftClick: "abc", // multi-char string is not valid
					},
				},
			},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.config.ValidateAction()
			if (err != nil) != testCase.wantErr {
				t.Errorf("Config.ValidateAction() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

// TestConfig_ValidateSmoothCursor tests the Config.ValidateSmoothCursor method.
func TestConfig_ValidateSmoothCursor(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		wantErr bool
	}{
		{
			name: "valid smooth cursor config",
			config: config.Config{
				SmoothCursor: config.SmoothCursorConfig{
					MoveMouseEnabled: true,
					Steps:            10,
					MaxDuration:      200,
					DurationPerPixel: 0.1,
				},
			},
			wantErr: false,
		},
		{
			name: "negative steps",
			config: config.Config{
				SmoothCursor: config.SmoothCursorConfig{
					MoveMouseEnabled: true,
					Steps:            -1,
				},
			},
			wantErr: true,
		},
		{
			name: "zero steps",
			config: config.Config{
				SmoothCursor: config.SmoothCursorConfig{
					MoveMouseEnabled: true,
					Steps:            0,
				},
			},
			wantErr: true,
		},
		{
			name: "negative max_duration",
			config: config.Config{
				SmoothCursor: config.SmoothCursorConfig{
					MoveMouseEnabled: true,
					Steps:            10,
					MaxDuration:      -1,
				},
			},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.config.ValidateSmoothCursor()
			if (err != nil) != testCase.wantErr {
				t.Errorf(
					"Config.ValidateSmoothCursor() error = %v, wantErr %v",
					err,
					testCase.wantErr,
				)
			}
		})
	}
}

// TestConfig_ValidateScrollKeyBindings tests the scroll key bindings validation.
func TestConfig_ValidateScrollKeyBindings(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		wantErr bool
	}{
		{
			name: "valid key bindings",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = map[string][]string{
					"scroll_up":   {"k", "Up"},
					"scroll_down": {"j", "Down"},
					"go_top":      {"gg"},
					"go_bottom":   {"G"},
					"page_up":     {"Ctrl+U", "PageUp"},
					"page_down":   {"Ctrl+D", "PageDown"},
				}

				return *cfg
			}(),
			wantErr: false,
		},
		{
			name: "empty key bindings - valid",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = map[string][]string{}

				return *cfg
			}(),
			wantErr: false,
		},
		{
			name: "nil key bindings - valid",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = nil

				return *cfg
			}(),
			wantErr: false,
		},
		{
			name: "unknown action",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = map[string][]string{
					"unknown_action": {"k"},
				}

				return *cfg
			}(),
			wantErr: true,
		},
		{
			name: "empty keys array",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = map[string][]string{
					"scroll_up": {},
				}

				return *cfg
			}(),
			wantErr: true,
		},
		{
			name: "empty key in array",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = map[string][]string{
					"scroll_up": {"k", ""},
				}

				return *cfg
			}(),
			wantErr: true,
		},
		{
			name: "invalid modifier",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = map[string][]string{
					"scroll_up": {"Super+D"},
				}

				return *cfg
			}(),
			wantErr: true,
		},
		{
			name: "invalid key name",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = map[string][]string{
					"scroll_up": {"InvalidKeyName"},
				}

				return *cfg
			}(),
			wantErr: true,
		},
		{
			name: "invalid function key out of supported range",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = map[string][]string{
					"scroll_up": {"F21"},
				}

				return *cfg
			}(),
			wantErr: true,
		},
		{
			name: "valid single-letter keys",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = map[string][]string{
					"scroll_up": {"g"},
				}

				return *cfg
			}(),
			wantErr: false,
		},
		{
			name: "invalid sequence - too long",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = map[string][]string{
					"go_top": {"ggg"},
				}

				return *cfg
			}(),
			wantErr: true,
		},
		{
			name: "invalid sequence - non-letter",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = map[string][]string{
					"go_top": {"g1"},
				}

				return *cfg
			}(),
			wantErr: true,
		},
		{
			name: "valid special keys",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = map[string][]string{
					"scroll_up": {
						"Space",
						"Return",
						"Enter",
						"Escape",
						"Tab",
						"Delete",
						"Backspace",
					},
					"scroll_down": {"Home", "End", "PageUp", "PageDown"},
					"scroll_left": {"Up", "Down", "Left", "Right"},
					"scroll_right": {
						"F1",
						"F2",
						"F3",
						"F4",
						"F5",
						"F6",
						"F7",
						"F8",
						"F9",
						"F10",
						"F11",
						"F12",
						"F13",
						"F14",
						"F15",
						"F16",
						"F17",
						"F18",
						"F19",
						"F20",
					},
					"page_up": {
						"Cmd+Up",
						"Cmd+Down",
						"Ctrl+Z",
						"Ctrl+U",
					},
				}

				return *cfg
			}(),
			wantErr: false,
		},
		{
			name: "valid mixed modifiers",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = map[string][]string{
					"scroll_up":   {"Cmd+K", "Ctrl+Shift+Up", "Alt+Option+Down"},
					"scroll_down": {"Cmd+Ctrl+Alt+Shift+X"},
				}

				return *cfg
			}(),
			wantErr: false,
		},
		{
			name: "empty key with modifier",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = map[string][]string{
					"scroll_up": {"Ctrl+"},
				}

				return *cfg
			}(),
			wantErr: true,
		},
		{
			name: "whitespace in key",
			config: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Scroll.KeyBindings = map[string][]string{
					"scroll_up": {" k "},
				}

				return *cfg
			}(),
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.config.ValidateScrollKeyBindings()
			if (err != nil) != testCase.wantErr {
				t.Errorf(
					"Config.ValidateScrollKeyBindings() error = %v, wantErr %v",
					err,
					testCase.wantErr,
				)
			}
		})
	}
}

// TestConfig_Validate_BackspaceKeyActionKeyConflicts tests that backspace keys
// cannot conflict with action key bindings (checked via full Validate()).
func TestConfig_Validate_BackspaceKeyActionKeyConflicts(t *testing.T) {
	tests := []struct {
		name    string
		config  func() config.Config
		wantErr bool
	}{
		{
			name: "hints backspace_key conflicts with left_click binding",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.BackspaceKey = testActionConflictKey
				cfg.Action.KeyBindings.LeftClick = testActionConflictKey

				return cfg
			},
			wantErr: true,
		},
		{
			name: "grid backspace_key conflicts with right_click binding",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Grid.BackspaceKey = "Cmd+R"
				cfg.Action.KeyBindings.RightClick = "Cmd+R"

				return cfg
			},
			wantErr: true,
		},
		{
			name: "recursive_grid backspace_key conflicts with move_mouse_up binding",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.RecursiveGrid.BackspaceKey = "Shift+K"
				cfg.Action.KeyBindings.MoveMouseUp = "Shift+K"

				return cfg
			},
			wantErr: true,
		},
		{
			name: "backspace_key conflicts case-insensitive with action binding",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.BackspaceKey = "shift+l"
				cfg.Action.KeyBindings.LeftClick = testActionConflictKey

				return cfg
			},
			wantErr: true,
		},
		{
			name: "no conflict when backspace_key differs from all action bindings",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.BackspaceKey = "Ctrl+Z"

				// Default action bindings don't include Ctrl+Z
				return cfg
			},
			wantErr: false,
		},
		{
			name: "no conflict when backspace_key is empty (default) and no action binding is delete",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.BackspaceKey = ""

				return cfg
			},
			wantErr: false,
		},
		{
			name: "empty backspace_key (default) no conflict with non-delete action bindings",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.BackspaceKey = "" // default backspace/delete
				cfg.Grid.BackspaceKey = ""
				cfg.RecursiveGrid.BackspaceKey = ""

				return cfg
			},
			wantErr: false,
		},
		{
			name: "no conflict when mode is disabled",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.Enabled = false
				cfg.Hints.BackspaceKey = testActionConflictKey
				cfg.Action.KeyBindings.LeftClick = testActionConflictKey

				return cfg
			},
			wantErr: false,
		},
		{
			name: "no conflict when action binding is empty",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.BackspaceKey = testActionConflictKey
				cfg.Action.KeyBindings.LeftClick = ""

				return cfg
			},
			wantErr: false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := testCase.config()
			err := cfg.Validate()

			if (err != nil) != testCase.wantErr {
				t.Errorf(
					"Config.Validate() error = %v, wantErr %v",
					err,
					testCase.wantErr,
				)
			}
		})
	}
}

// TestConfig_Validate_ResetKeyActionKeyConflicts tests that reset keys
// cannot conflict with action key bindings (checked via full Validate()).
func TestConfig_Validate_ResetKeyActionKeyConflicts(t *testing.T) {
	tests := []struct {
		name    string
		config  func() config.Config
		wantErr bool
	}{
		{
			name: "action binding 'Space' conflicts with default grid reset_key (space)",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Action.KeyBindings.LeftClick = testResetKeyConflictBinding
				// Grid.ResetKey defaults to " " (space)

				return cfg
			},
			wantErr: true,
		},
		{
			name: "action binding 'Space' conflicts with default recursive_grid reset_key (space)",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Action.KeyBindings.MiddleClick = testResetKeyConflictBinding
				cfg.Grid.ResetKey = "," // Avoid grid conflict

				return cfg
			},
			wantErr: true,
		},
		{
			name: "no conflict when grid mode is disabled",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Action.KeyBindings.LeftClick = testResetKeyConflictBinding
				cfg.Grid.Enabled = false
				cfg.RecursiveGrid.ResetKey = "," // Avoid recursive-grid conflict

				return cfg
			},
			wantErr: false,
		},
		{
			name: "no conflict when action binding differs from reset key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				// Default action bindings (Shift+L, etc.) don't conflict with space reset key
				return cfg
			},
			wantErr: false,
		},
		{
			name: "action binding conflicts with custom grid reset_key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Grid.ResetKey = "F1"
				cfg.Action.KeyBindings.RightClick = "F1"

				return cfg
			},
			wantErr: true,
		},
		{
			name: "no conflict when action binding is empty",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Action.KeyBindings.LeftClick = ""

				return cfg
			},
			wantErr: false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := testCase.config()

			err := cfg.Validate()
			if (err != nil) != testCase.wantErr {
				t.Errorf(
					"Config.Validate() error = %v, wantErr %v",
					err,
					testCase.wantErr,
				)
			}
		})
	}
}

// TestValidateActionKeyBinding tests the ValidateActionKeyBinding function.
func TestValidateActionKeyBinding(t *testing.T) {
	tests := []struct {
		name       string
		keybinding string
		wantErr    bool
	}{
		{
			name:       "valid modifier plus alphabet",
			keybinding: "Cmd+L",
			wantErr:    false,
		},
		{
			name:       "valid modifier plus Return",
			keybinding: "Shift+Return",
			wantErr:    false,
		},
		{
			name:       "valid modifier plus Enter",
			keybinding: "Cmd+Enter",
			wantErr:    false,
		},
		{
			name:       "valid multiple modifiers",
			keybinding: "Cmd+Shift+L",
			wantErr:    false,
		},
		{
			name:       "valid single Return",
			keybinding: "Return",
			wantErr:    false,
		},
		{
			name:       "valid single Enter",
			keybinding: "Enter",
			wantErr:    false,
		},
		{
			name:       "valid_modifier_plus_Return#01",
			keybinding: "Cmd+Return",
			wantErr:    false,
		},
		{
			name:       "valid all modifiers",
			keybinding: "Cmd+Ctrl+Alt+Shift+Option+L",
			wantErr:    false,
		},
		{
			name:       "empty is valid (uses default)",
			keybinding: "",
			wantErr:    false,
		},
		{
			name:       "valid single lowercase alphabet",
			keybinding: "l",
			wantErr:    false,
		},
		{
			name:       "valid single uppercase alphabet",
			keybinding: "L",
			wantErr:    false,
		},
		{
			name:       "valid single number",
			keybinding: "1",
			wantErr:    false,
		},
		{
			name:       "valid single symbol",
			keybinding: "!",
			wantErr:    false,
		},
		{
			name:       "invalid multi-char non-named key",
			keybinding: "abc",
			wantErr:    true,
		},
		{
			name:       "invalid modifier only no key",
			keybinding: "Cmd+",
			wantErr:    true,
		},
		{
			name:       "invalid no plus sign",
			keybinding: "CmdL",
			wantErr:    true,
		},
		{
			name:       "valid lowercase key with modifier",
			keybinding: "Cmd+l",
			wantErr:    false,
		},
		{
			name:       "invalid modifier name",
			keybinding: "Super+L",
			wantErr:    true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Action.KeyBindings.LeftClick = testCase.keybinding

			err := cfg.ValidateActionKeyBindings()
			if (err != nil) != testCase.wantErr {
				t.Errorf(
					"Config.ValidateActionKeyBindings() error = %v, wantErr %v",
					err,
					testCase.wantErr,
				)
			}
		})
	}
}

// TestConfig_ValidatePerModeExitKeys tests per-mode mode_exit_keys validation.
func TestConfig_ValidatePerModeExitKeys(t *testing.T) {
	tests := []struct {
		name    string
		config  func() config.Config
		wantErr bool
	}{
		{
			name: "hints per-mode exit key valid - no conflict",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"q"}

				return cfg
			},
			wantErr: false,
		},
		{
			name: "hints per-mode exit key conflicts with hint_characters",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"a"}
				// Default hint_characters = "asdfghjkl"
				return cfg
			},
			wantErr: true,
		},
		{
			name: "hints per-mode exit key conflicts with backspace_key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"x"}
				cfg.Hints.BackspaceKey = "x"

				return cfg
			},
			wantErr: true,
		},
		{
			name: "hints per-mode exit key 'delete' conflicts with default backspace",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"delete"}
				cfg.Hints.BackspaceKey = "" // default

				return cfg
			},
			wantErr: true,
		},
		{
			name: "hints per-mode exit key modifier combo valid",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"Ctrl+X"}

				return cfg
			},
			wantErr: false,
		},
		{
			name: "hints per-mode exit key invalid format",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"invalidkey"}

				return cfg
			},
			wantErr: true,
		},
		{
			name: "hints per-mode exit key empty string in slice",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"q", ""}

				return cfg
			},
			wantErr: true,
		},
		{
			name: "grid per-mode exit key conflicts with characters",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Grid.ModeExitKeys = []string{"a"}
				// Default grid characters include 'a'
				return cfg
			},
			wantErr: true,
		},
		{
			name: "grid per-mode exit key conflicts with reset_key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Grid.ModeExitKeys = []string{"space"}
				// Default grid reset_key = " " (space)
				return cfg
			},
			wantErr: true,
		},
		{
			name: "grid per-mode exit key valid - no conflict",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Grid.ModeExitKeys = []string{"Ctrl+Q"}

				return cfg
			},
			wantErr: false,
		},
		{
			name: "recursive_grid per-mode exit key conflicts with keys",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.RecursiveGrid.ModeExitKeys = []string{"u"}
				// Default recursive_grid keys = "uijk"
				return cfg
			},
			wantErr: true,
		},
		{
			name: "recursive_grid per-mode exit key conflicts with reset_key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.RecursiveGrid.ModeExitKeys = []string{"space"}
				// Default recursive_grid reset_key = " " (space)
				return cfg
			},
			wantErr: true,
		},
		{
			name: "recursive_grid per-mode exit key valid",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.RecursiveGrid.ModeExitKeys = []string{"q"}

				return cfg
			},
			wantErr: false,
		},
		{
			name: "scroll per-mode exit key conflicts with scroll binding",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Scroll.ModeExitKeys = []string{"j"}
				// Default scroll_down = ["j", "Down"]
				return cfg
			},
			wantErr: true,
		},
		{
			name: "scroll per-mode exit key conflicts with modifier scroll binding",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Scroll.ModeExitKeys = []string{"Ctrl+D"}
				// Default page_down = ["Ctrl+D", "PageDown"]
				return cfg
			},
			wantErr: true,
		},
		{
			name: "scroll per-mode exit key valid - no conflict",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Scroll.ModeExitKeys = []string{"q"}

				return cfg
			},
			wantErr: false,
		},
		{
			name: "empty per-mode exit keys uses global fallback - valid",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				// All ModeExitKeys nil/empty = use global only
				return cfg
			},
			wantErr: false,
		},
		{
			name: "multiple modes with per-mode exit keys - all valid",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"q"}
				cfg.Grid.ModeExitKeys = []string{"Ctrl+Q"}
				cfg.RecursiveGrid.ModeExitKeys = []string{"q"}
				cfg.Scroll.ModeExitKeys = []string{"q"}

				return cfg
			},
			wantErr: false,
		},
		{
			name: "per-mode exit key duplicate of global is accepted (deduplicated at runtime)",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"escape"} // same as global

				return cfg
			},
			wantErr: false,
		},
		{
			name: "grid per-mode exit key conflicts with sublayer_keys",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Grid.ModeExitKeys = []string{"b"}
				// Default sublayer_keys = "abcdefghijklmnpqrstuvwxyz"
				return cfg
			},
			wantErr: true,
		},
		{
			name: "grid per-mode exit key conflicts with row_labels",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Grid.ModeExitKeys = []string{"1"}
				cfg.Grid.RowLabels = "123456789"

				return cfg
			},
			wantErr: true,
		},
		{
			name: "grid per-mode exit key conflicts with col_labels",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Grid.ModeExitKeys = []string{"x"}
				cfg.Grid.ColLabels = "wxyz123456"

				return cfg
			},
			wantErr: true,
		},
		{
			name: "grid per-mode exit key conflicts with backspace_key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Grid.ModeExitKeys = []string{"x"}
				cfg.Grid.BackspaceKey = "x"

				return cfg
			},
			wantErr: true,
		},
		{
			name: "recursive_grid per-mode exit key conflicts with backspace_key",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.RecursiveGrid.ModeExitKeys = []string{"x"}
				cfg.RecursiveGrid.BackspaceKey = "x"

				return cfg
			},
			wantErr: true,
		},
		{
			name: "scroll per-mode exit key named key conflicts with arrow binding",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Scroll.ModeExitKeys = []string{"Up"}
				// Default scroll_up = ["k", "Up"]
				return cfg
			},
			wantErr: true,
		},
		{
			name: "scroll per-mode exit key prefix conflicts with multi-key sequence",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Scroll.ModeExitKeys = []string{"g"}
				// Default go_top = ["gg", "Cmd+Up"] — "g" is a prefix of "gg"
				return cfg
			},
			wantErr: true,
		},
		{
			name: "scroll per-mode exit key no prefix conflict with non-matching sequence",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Scroll.ModeExitKeys = []string{"q"}
				// Default go_top = ["gg", "Cmd+Up"] — "q" is not a prefix of "gg"
				return cfg
			},
			wantErr: false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := testCase.config()

			err := cfg.Validate()
			if (err != nil) != testCase.wantErr {
				t.Errorf(
					"Config.Validate() error = %v, wantErr %v",
					err,
					testCase.wantErr,
				)
			}
		})
	}
}

// TestConfig_Validate_PerModeExitKeysActionKeyConflicts tests that per-mode exit keys
// cannot conflict with action key bindings (checked via full Validate()).
func TestConfig_Validate_PerModeExitKeysActionKeyConflicts(t *testing.T) {
	tests := []struct {
		name    string
		config  func() config.Config
		wantErr bool
	}{
		{
			name: "hints per-mode exit key conflicts with move_mouse_up binding",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"Up"}
				// Default move_mouse_up = "Up"
				return cfg
			},
			wantErr: true,
		},
		{
			name: "hints per-mode exit key conflicts with left_click binding",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"Shift+L"}
				// Default left_click = "Shift+L"
				return cfg
			},
			wantErr: true,
		},
		{
			name: "grid per-mode exit key conflicts with right_click binding",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Grid.ModeExitKeys = []string{"Shift+R"}
				// Default right_click = "Shift+R"
				return cfg
			},
			wantErr: true,
		},
		{
			name: "recursive_grid per-mode exit key conflicts with mouse_down binding",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.RecursiveGrid.ModeExitKeys = []string{"Shift+I"}
				// Default mouse_down = "Shift+I"
				return cfg
			},
			wantErr: true,
		},
		{
			name: "scroll per-mode exit key does NOT check action bindings (scroll has no action keys)",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Scroll.ModeExitKeys = []string{"Shift+L"}
				// Even though left_click = "Shift+L", scroll mode doesn't use action keys
				return cfg
			},
			wantErr: false,
		},
		{
			name: "no conflict when per-mode exit key differs from all action bindings",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"q"}

				return cfg
			},
			wantErr: false,
		},
		{
			name: "no conflict when mode is disabled",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.Enabled = false
				cfg.Hints.ModeExitKeys = []string{"Up"} // would conflict, but hints disabled

				return cfg
			},
			wantErr: false,
		},
		{
			name: "no conflict when action binding is empty",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"Shift+L"}
				cfg.Action.KeyBindings.LeftClick = ""

				return cfg
			},
			wantErr: false,
		},
		{
			name: "case-insensitive conflict detection",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"shift+l"}
				cfg.Action.KeyBindings.LeftClick = "Shift+L"

				return cfg
			},
			wantErr: true,
		},
		{
			name: "no conflict with Ctrl+Q exit key and default action bindings",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Grid.ModeExitKeys = []string{"Ctrl+Q"}

				return cfg
			},
			wantErr: false,
		},
		{
			name: "per-mode exit key conflicts with custom single-char action binding",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"w"}
				cfg.Action.KeyBindings.MoveMouseUp = "w"

				return cfg
			},
			wantErr: true,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := testCase.config()

			err := cfg.Validate()
			if (err != nil) != testCase.wantErr {
				t.Errorf(
					"Config.Validate() error = %v, wantErr %v",
					err,
					testCase.wantErr,
				)
			}
		})
	}
}

// TestMergeExitKeys tests the MergeExitKeys helper function.
func TestMergeExitKeys(t *testing.T) {
	tests := []struct {
		name       string
		globalKeys []string
		modeKeys   []string
		wantLen    int
	}{
		{
			name:       "no mode keys returns global only",
			globalKeys: []string{"escape"},
			modeKeys:   nil,
			wantLen:    1,
		},
		{
			name:       "empty mode keys returns global only",
			globalKeys: []string{"escape"},
			modeKeys:   []string{},
			wantLen:    1,
		},
		{
			name:       "mode keys merged with global",
			globalKeys: []string{"escape"},
			modeKeys:   []string{"q"},
			wantLen:    2,
		},
		{
			name:       "duplicate mode key is deduplicated",
			globalKeys: []string{"escape"},
			modeKeys:   []string{"escape"},
			wantLen:    1,
		},
		{
			name:       "case-insensitive deduplication",
			globalKeys: []string{"Escape"},
			modeKeys:   []string{"escape"},
			wantLen:    1,
		},
		{
			name:       "multiple mode keys merged",
			globalKeys: []string{"escape"},
			modeKeys:   []string{"q", "Ctrl+C"},
			wantLen:    3,
		},
		{
			name:       "multiple global keys with one mode key",
			globalKeys: []string{"escape", "Ctrl+C"},
			modeKeys:   []string{"q"},
			wantLen:    3,
		},
		{
			name:       "partial overlap deduplicated",
			globalKeys: []string{"escape", "Ctrl+C"},
			modeKeys:   []string{"Ctrl+C", "q"},
			wantLen:    3,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := config.MergeExitKeys(testCase.globalKeys, testCase.modeKeys)
			if len(result) != testCase.wantLen {
				t.Errorf(
					"MergeExitKeys() returned %d keys %v, want %d",
					len(result),
					result,
					testCase.wantLen,
				)
			}
		})
	}
}

// TestConfig_ResolvedExitKeys tests the Config.ResolvedExitKeys method.
func TestConfig_ResolvedExitKeys(t *testing.T) {
	tests := []struct {
		name     string
		config   func() config.Config
		modeName string
		wantLen  int
	}{
		{
			name: "hints with no per-mode keys returns global",
			config: func() config.Config {
				cfg := *config.DefaultConfig()

				return cfg
			},
			modeName: "hints",
			wantLen:  1, // just "Escape"
		},
		{
			name: "hints with per-mode key returns merged",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Hints.ModeExitKeys = []string{"q"}

				return cfg
			},
			modeName: "hints",
			wantLen:  2, // "Escape" + "q"
		},
		{
			name: "scroll with per-mode key returns merged",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Scroll.ModeExitKeys = []string{"q", "Ctrl+X"}

				return cfg
			},
			modeName: "scroll",
			wantLen:  3, // "Escape" + "q" + "Ctrl+X"
		},
		{
			name: "unknown mode name returns global only",
			config: func() config.Config {
				cfg := *config.DefaultConfig()

				return cfg
			},
			modeName: "unknown",
			wantLen:  1,
		},
		{
			name: "grid with duplicate of global is deduplicated",
			config: func() config.Config {
				cfg := *config.DefaultConfig()
				cfg.Grid.ModeExitKeys = []string{"escape", "q"}

				return cfg
			},
			modeName: "grid",
			wantLen:  2, // "Escape" + "q" (duplicate "escape" removed)
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := testCase.config()

			result := cfg.ResolvedExitKeys(testCase.modeName)
			if len(result) != testCase.wantLen {
				t.Errorf(
					"ResolvedExitKeys(%q) returned %d keys %v, want %d",
					testCase.modeName,
					len(result),
					result,
					testCase.wantLen,
				)
			}
		})
	}
}
