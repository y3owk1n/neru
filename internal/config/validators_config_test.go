package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

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
					HintCharacters:    "abcd",
					BackgroundColor:   "#FFFFFF",
					TextColor:         "#000000",
					MatchedTextColor:  "#FF0000",
					BorderColor:       "#000000",
					FontSize:          12,
					BorderRadius:      4,
					Padding:           4,
					BorderWidth:       1,
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
			name: "hints with valid ASCII digits and symbols",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:    "123!@#", // Valid - ASCII digits and symbols
					BackgroundColor:   "#FFFFFF",
					TextColor:         "#000000",
					MatchedTextColor:  "#FF0000",
					BorderColor:       "#000000",
					FontSize:          12,
					BorderRadius:      4,
					Padding:           4,
					BorderWidth:       1,
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
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					BackgroundColor:        "#FF0000",
					TextColor:              "#FFFFFF",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#333333",
					MatchedBorderColor:     "#FF0000",
					BorderColor:            "#666666",
					FontSize:               14,
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
					Enabled:         true,
					Characters:      "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					BackgroundColor: "invalid",
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
					Enabled:                true,
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					RowLabels:              "123456789",
					FontSize:               12,
					BackgroundColor:        "#ffffff",
					TextColor:              "#000000",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#ffffff",
					MatchedBorderColor:     "#000000",
					BorderColor:            "#000000",
				},
			},
			wantErr: false,
		},
		{
			name: "grid with valid col_labels",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:                true,
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					ColLabels:              "abcdefghij",
					FontSize:               12,
					BackgroundColor:        "#ffffff",
					TextColor:              "#000000",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#ffffff",
					MatchedBorderColor:     "#000000",
					BorderColor:            "#000000",
				},
			},
			wantErr: false,
		},
		{
			name: "grid with too short row_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:                true,
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					RowLabels:              "1", // Too short
					FontSize:               12,
					BackgroundColor:        "#ffffff",
					TextColor:              "#000000",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#ffffff",
					MatchedBorderColor:     "#000000",
					BorderColor:            "#000000",
				},
			},
			wantErr: true,
		},
		{
			name: "grid with too short col_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:                true,
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					ColLabels:              "a", // Too short
					FontSize:               12,
					BackgroundColor:        "#ffffff",
					TextColor:              "#000000",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#ffffff",
					MatchedBorderColor:     "#000000",
					BorderColor:            "#000000",
				},
			},
			wantErr: true,
		},
		{
			name: "grid with reserved character in row_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:                true,
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					RowLabels:              "123 456", // Contains ' '
					FontSize:               12,
					BackgroundColor:        "#ffffff",
					TextColor:              "#000000",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#ffffff",
					MatchedBorderColor:     "#000000",
					BorderColor:            "#ffffff",
				},
			},
			wantErr: true,
		},
		{
			name: "grid with reserved character in col_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:                true,
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					ColLabels:              "abc def", // Contains ' '
					FontSize:               12,
					BackgroundColor:        "#ffffff",
					TextColor:              "#000000",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#ffffff",
					MatchedBorderColor:     "#000000",
					BorderColor:            "#ffffff",
				},
			},
			wantErr: true,
		},
		{
			name: "grid with unicode in row_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:                true,
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					RowLabels:              "123é456", // Contains Unicode
					FontSize:               12,
					BackgroundColor:        "#ffffff",
					TextColor:              "#000000",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#ffffff",
					MatchedBorderColor:     "#000000",
					BorderColor:            "#000000",
				},
			},
			wantErr: true,
		},
		{
			name: "grid with unicode in col_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:                true,
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					ColLabels:              "abc😀def", // Contains Unicode
					FontSize:               12,
					BackgroundColor:        "#ffffff",
					TextColor:              "#000000",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#ffffff",
					MatchedBorderColor:     "#000000",
					BorderColor:            "#000000",
				},
			},
			wantErr: true,
		},
		{
			name: "grid with duplicate characters in sublayer_keys - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:                true,
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					SublayerKeys:           "aaabbbccc", // Contains duplicate 'a', 'b', 'c'
					FontSize:               12,
					BackgroundColor:        "#ffffff",
					TextColor:              "#000000",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#ffffff",
					MatchedBorderColor:     "#000000",
					BorderColor:            "#000000",
				},
			},
			wantErr: true,
		},
		{
			name: "grid with duplicate characters - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:                true,
					Characters:             "AaaBCDEF", // Contains duplicate 'A' and 'a'
					FontSize:               12,
					BackgroundColor:        "#ffffff",
					TextColor:              "#000000",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#ffffff",
					MatchedBorderColor:     "#000000",
					BorderColor:            "#000000",
				},
			},
			wantErr: true,
		},
		{
			name: "grid with duplicate characters in row_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:                true,
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					RowLabels:              "1123456789", // Contains duplicate '1'
					FontSize:               12,
					BackgroundColor:        "#ffffff",
					TextColor:              "#000000",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#ffffff",
					MatchedBorderColor:     "#000000",
					BorderColor:            "#000000",
				},
			},
			wantErr: true,
		},
		{
			name: "grid with duplicate characters in col_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:                true,
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					ColLabels:              "aabbcdef", // Contains duplicate 'a' and 'b'
					FontSize:               12,
					BackgroundColor:        "#ffffff",
					TextColor:              "#000000",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#ffffff",
					MatchedBorderColor:     "#000000",
					BorderColor:            "#000000",
				},
			},
			wantErr: true,
		},
		{
			name: "grid with valid ASCII digits and symbols",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:             "123!@#",    // Valid - ASCII digits and symbols
					SublayerKeys:           "abcdefghi", // Required for subgrid
					BackgroundColor:        "#FF0000",
					TextColor:              "#FFFFFF",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#333333",
					MatchedBorderColor:     "#FF0000",
					BorderColor:            "#666666",
					FontSize:               14,
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
			name: "negative font size",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:    true,
					Characters: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					FontSize:   -1,
				},
			},
			wantErr: true,
		},
		{
			name: "valid grid config with auto_exit_actions",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					BackgroundColor:        "#FF0000",
					TextColor:              "#FFFFFF",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#333333",
					MatchedBorderColor:     "#FF0000",
					BorderColor:            "#666666",
					FontSize:               14,
					AutoExitActions:        []string{"left_click", "right_click"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid grid config with empty auto_exit_actions",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					BackgroundColor:        "#FF0000",
					TextColor:              "#FFFFFF",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#333333",
					MatchedBorderColor:     "#FF0000",
					BorderColor:            "#666666",
					FontSize:               14,
					AutoExitActions:        []string{},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid grid auto_exit_actions with unknown action",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					BackgroundColor:        "#FF0000",
					TextColor:              "#FFFFFF",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#333333",
					MatchedBorderColor:     "#FF0000",
					BorderColor:            "#666666",
					FontSize:               14,
					AutoExitActions:        []string{"unknown_action"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid grid auto_exit_actions with scroll (IPC-only)",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					BackgroundColor:        "#FF0000",
					TextColor:              "#FFFFFF",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#333333",
					MatchedBorderColor:     "#FF0000",
					BorderColor:            "#666666",
					FontSize:               14,
					AutoExitActions:        []string{"scroll"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid grid auto_exit_actions with move_mouse (IPC-only)",
			config: config.Config{
				Grid: config.GridConfig{
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					BackgroundColor:        "#FF0000",
					TextColor:              "#FFFFFF",
					MatchedTextColor:       "#000000",
					MatchedBackgroundColor: "#333333",
					MatchedBorderColor:     "#FF0000",
					BorderColor:            "#666666",
					FontSize:               14,
					AutoExitActions:        []string{"move_mouse"},
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
			name: "invalid key binding format",
			config: config.Config{
				Action: config.ActionConfig{
					KeyBindings: config.ActionKeyBindingsCfg{
						LeftClick: "l", // lowercase not allowed
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
					Delay:            5,
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
			name: "negative delay",
			config: config.Config{
				SmoothCursor: config.SmoothCursorConfig{
					MoveMouseEnabled: true,
					Steps:            10,
					Delay:            -1,
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
			name:       "invalid single lowercase alphabet",
			keybinding: "l",
			wantErr:    true,
		},
		{
			name:       "invalid single uppercase alphabet",
			keybinding: "L",
			wantErr:    true,
		},
		{
			name:       "invalid single number",
			keybinding: "1",
			wantErr:    true,
		},
		{
			name:       "invalid single symbol",
			keybinding: "!",
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
			name:       "invalid lowercase key with modifier",
			keybinding: "Cmd+l",
			wantErr:    true,
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
