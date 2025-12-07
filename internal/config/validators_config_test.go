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
					HintCharacters:   "abcd",
					Opacity:          0.9,
					BackgroundColor:  "#FFFFFF",
					TextColor:        "#000000",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#000000",
					FontSize:         12,
					BorderRadius:     4,
					Padding:          4,
					BorderWidth:      1,
					ClickableRoles:   []string{"AXButton"},
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
					HintCharacters:   "123!@#", // Valid - ASCII digits and symbols
					Opacity:          0.9,
					BackgroundColor:  "#FFFFFF",
					TextColor:        "#000000",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#000000",
					FontSize:         12,
					BorderRadius:     4,
					Padding:          4,
					BorderWidth:      1,
					ClickableRoles:   []string{"AXButton"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateHints()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.ValidateHints() error = %v, wantErr %v", err, tt.wantErr)
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateAppConfigs()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.ValidateAppConfigs() error = %v, wantErr %v", err, tt.wantErr)
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
					Opacity:                0.8,
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
					Characters: "ABC<DEF", // Contains '<'
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
					RowLabels:              "123<456", // Contains '<'
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
			name: "grid with reserved character in col_labels - invalid",
			config: config.Config{
				Grid: config.GridConfig{
					Enabled:                true,
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					ColLabels:              "abc<def", // Contains '<'
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
					Opacity:                0.8,
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
					SublayerKeys: "abcdefg<h", // Invalid - contains '<'
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateGrid()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.ValidateGrid() error = %v, wantErr %v", err, tt.wantErr)
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
					HighlightWidth: 2,
					HighlightColor: "#FF0000",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid highlight width zero",
			config: config.Config{
				Action: config.ActionConfig{
					HighlightWidth: 0,
					HighlightColor: "#FF0000",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid highlight width negative",
			config: config.Config{
				Action: config.ActionConfig{
					HighlightWidth: -1,
					HighlightColor: "#FF0000",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid highlight color empty",
			config: config.Config{
				Action: config.ActionConfig{
					HighlightWidth: 2,
					HighlightColor: "",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid highlight color not hex",
			config: config.Config{
				Action: config.ActionConfig{
					HighlightWidth: 2,
					HighlightColor: "red",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid highlight color too short",
			config: config.Config{
				Action: config.ActionConfig{
					HighlightWidth: 2,
					HighlightColor: "#12",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid highlight color invalid length",
			config: config.Config{
				Action: config.ActionConfig{
					HighlightWidth: 2,
					HighlightColor: "#12345",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid highlight color invalid characters",
			config: config.Config{
				Action: config.ActionConfig{
					HighlightWidth: 2,
					HighlightColor: "#GGGGGG",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateAction()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.ValidateAction() error = %v, wantErr %v", err, tt.wantErr)
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateSmoothCursor()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.ValidateSmoothCursor() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
