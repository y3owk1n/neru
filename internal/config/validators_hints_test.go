package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// TestValidateHints tests the validateHints function.
func TestValidateHints(t *testing.T) {
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
					BackgroundColor:  "#FFFFFF",
					TextColor:        "#000000",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#000000",
					FontSize:         12,
					BorderRadius:     4,
					Padding:          4,
					BorderWidth:      1,
					ClickableRoles:   []string{"AXButton"},
					AdditionalAXSupport: config.AdditionalAXSupport{
						AdditionalElectronBundles: []string{"com.example.app"},
						AdditionalChromiumBundles: []string{"com.example.chromium"},
						AdditionalFirefoxBundles:  []string{"com.example.firefox"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty hint_characters",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "",
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
			wantErr: true,
		},
		{
			name: "hint_characters too short",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "a",
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
			wantErr: true,
		},
		{
			name: "invalid background_color",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "abcd",
					BackgroundColor:  "invalid",
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
			wantErr: true,
		},
		{
			name: "invalid text_color",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "abcd",
					BackgroundColor:  "#FFFFFF",
					TextColor:        "invalid",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#000000",
					FontSize:         12,
					BorderRadius:     4,
					Padding:          4,
					BorderWidth:      1,
					ClickableRoles:   []string{"AXButton"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid matched_text_color",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "abcd",
					BackgroundColor:  "#FFFFFF",
					TextColor:        "#000000",
					MatchedTextColor: "invalid",
					BorderColor:      "#000000",
					FontSize:         12,
					BorderRadius:     4,
					Padding:          4,
					BorderWidth:      1,
					ClickableRoles:   []string{"AXButton"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid border_color",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "abcd",
					BackgroundColor:  "#FFFFFF",
					TextColor:        "#000000",
					MatchedTextColor: "#FF0000",
					BorderColor:      "invalid",
					FontSize:         12,
					BorderRadius:     4,
					Padding:          4,
					BorderWidth:      1,
					ClickableRoles:   []string{"AXButton"},
				},
			},
			wantErr: true,
		},
		{
			name: "negative font_size",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "abcd",
					BackgroundColor:  "#FFFFFF",
					TextColor:        "#000000",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#000000",
					FontSize:         -1,
					BorderRadius:     4,
					Padding:          4,
					BorderWidth:      1,
					ClickableRoles:   []string{"AXButton"},
				},
			},
			wantErr: true,
		},
		{
			name: "zero font_size",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "abcd",
					BackgroundColor:  "#FFFFFF",
					TextColor:        "#000000",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#000000",
					FontSize:         0,
					BorderRadius:     4,
					Padding:          4,
					BorderWidth:      1,
					ClickableRoles:   []string{"AXButton"},
				},
			},
			wantErr: true,
		},
		{
			name: "negative border_radius",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "abcd",
					BackgroundColor:  "#FFFFFF",
					TextColor:        "#000000",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#000000",
					FontSize:         12,
					BorderRadius:     -1,
					Padding:          4,
					BorderWidth:      1,
					ClickableRoles:   []string{"AXButton"},
				},
			},
			wantErr: true,
		},
		{
			name: "negative padding",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "abcd",
					BackgroundColor:  "#FFFFFF",
					TextColor:        "#000000",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#000000",
					FontSize:         12,
					BorderRadius:     4,
					Padding:          -1,
					BorderWidth:      1,
					ClickableRoles:   []string{"AXButton"},
				},
			},
			wantErr: true,
		},
		{
			name: "negative border_width",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "abcd",
					BackgroundColor:  "#FFFFFF",
					TextColor:        "#000000",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#000000",
					FontSize:         12,
					BorderRadius:     4,
					Padding:          4,
					BorderWidth:      -1,
					ClickableRoles:   []string{"AXButton"},
				},
			},
			wantErr: true,
		},
		{
			name: "empty string in clickable_roles",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "abcd",
					BackgroundColor:  "#FFFFFF",
					TextColor:        "#000000",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#000000",
					FontSize:         12,
					BorderRadius:     4,
					Padding:          4,
					BorderWidth:      1,
					ClickableRoles:   []string{"AXButton", ""},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateHints()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHints() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
