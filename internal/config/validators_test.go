package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// TestValidateColor tests the validateColor function.
func TestValidateColor(t *testing.T) {
	tests := []struct {
		name      string
		color     string
		fieldName string
		wantErr   bool
	}{
		{
			name:      "valid 6-digit hex",
			color:     "#FF0000",
			fieldName: "test_color",
			wantErr:   false,
		},
		{
			name:      "valid 3-digit hex",
			color:     "#F00",
			fieldName: "test_color",
			wantErr:   false,
		},
		{
			name:      "valid 8-digit hex with alpha",
			color:     "#FF0000AA",
			fieldName: "test_color",
			wantErr:   false,
		},
		{
			name:      "lowercase hex",
			color:     "#ff0000",
			fieldName: "test_color",
			wantErr:   false,
		},
		{
			name:      "empty color",
			color:     "",
			fieldName: "test_color",
			wantErr:   true,
		},
		{
			name:      "missing hash",
			color:     "FF0000",
			fieldName: "test_color",
			wantErr:   true,
		},
		{
			name:      "invalid characters",
			color:     "#GGGGGG",
			fieldName: "test_color",
			wantErr:   true,
		},
		{
			name:      "wrong length",
			color:     "#FF00",
			fieldName: "test_color",
			wantErr:   true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := config.ValidateColor(testCase.color, testCase.fieldName)
			if (err != nil) != testCase.wantErr {
				t.Errorf("ValidateColor() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

// TestValidateHints tests the ValidateHints function.
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
			wantErr: true,
		},
		{
			name: "hint_characters too short",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "a",
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
			wantErr: true,
		},
		{
			name: "opacity too low",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "abcd",
					Opacity:          -0.1,
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
			name: "opacity too high",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "abcd",
					Opacity:          1.1,
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
					Opacity:          0.9,
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
					Opacity:          0.9,
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
					Opacity:          0.9,
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
					Opacity:          0.9,
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
			name: "font_size too small",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "abcd",
					Opacity:          0.9,
					BackgroundColor:  "#FFFFFF",
					TextColor:        "#000000",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#000000",
					FontSize:         5,
					BorderRadius:     4,
					Padding:          4,
					BorderWidth:      1,
					ClickableRoles:   []string{"AXButton"},
				},
			},
			wantErr: true,
		},
		{
			name: "font_size too large",
			config: config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "abcd",
					Opacity:          0.9,
					BackgroundColor:  "#FFFFFF",
					TextColor:        "#000000",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#000000",
					FontSize:         73,
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
					Opacity:          0.9,
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
					Opacity:          0.9,
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
					Opacity:          0.9,
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
			name: "empty clickable_roles entry",
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
					ClickableRoles:   []string{"AXButton", ""},
				},
			},
			wantErr: true,
		},
		{
			name: "empty electron bundle",
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
					AdditionalAXSupport: config.AdditionalAXSupport{
						AdditionalElectronBundles: []string{""},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty chromium bundle",
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
					AdditionalAXSupport: config.AdditionalAXSupport{
						AdditionalChromiumBundles: []string{""},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty firefox bundle",
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
					AdditionalAXSupport: config.AdditionalAXSupport{
						AdditionalFirefoxBundles: []string{""},
					},
				},
			},
			wantErr: true,
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

// TestValidateHotkey tests the validateHotkey function.
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
			name:      "empty hotkey allowed",
			hotkey:    "",
			fieldName: "test_hotkey",
			wantErr:   false,
		},
		{
			name:      "invalid modifier",
			hotkey:    "Super+Space",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "empty key",
			hotkey:    "Cmd+",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
		{
			name:      "whitespace key",
			hotkey:    "Cmd+  ",
			fieldName: "test_hotkey",
			wantErr:   true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			validateHotkeyErr := config.ValidateHotkey(testCase.hotkey, testCase.fieldName)
			if (validateHotkeyErr != nil) != testCase.wantErr {
				t.Errorf(
					"ValidateHotkey() error = %v, wantErr %v",
					validateHotkeyErr,
					testCase.wantErr,
				)
			}
		})
	}
}

// TestConfig_ValidateHints tests the Config.validateHints method.
func TestConfig_ValidateHints(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name: "valid hints config",
			config: &config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "ABCDEFGH",
					Opacity:          0.9,
					BackgroundColor:  "#000000",
					TextColor:        "#FFFFFF",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#333333",
					FontSize:         14,
					BorderRadius:     4,
					Padding:          8,
					BorderWidth:      2,
				},
			},
			wantErr: false,
		},
		{
			name: "empty hint characters",
			config: &config.Config{
				Hints: config.HintsConfig{
					HintCharacters: "",
				},
			},
			wantErr: true,
		},
		{
			name: "too few hint characters",
			config: &config.Config{
				Hints: config.HintsConfig{
					HintCharacters: "A",
				},
			},
			wantErr: true,
		},
		{
			name: "opacity too low",
			config: &config.Config{
				Hints: config.HintsConfig{
					HintCharacters: "AB",
					Opacity:        -0.1,
				},
			},
			wantErr: true,
		},
		{
			name: "opacity too high",
			config: &config.Config{
				Hints: config.HintsConfig{
					HintCharacters: "AB",
					Opacity:        1.1,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid background color",
			config: &config.Config{
				Hints: config.HintsConfig{
					HintCharacters:  "AB",
					Opacity:         0.9,
					BackgroundColor: "invalid",
				},
			},
			wantErr: true,
		},
		{
			name: "font size too small",
			config: &config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "AB",
					Opacity:          0.9,
					BackgroundColor:  "#000000",
					TextColor:        "#FFFFFF",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#333333",
					FontSize:         5,
				},
			},
			wantErr: true,
		},
		{
			name: "font size too large",
			config: &config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "AB",
					Opacity:          0.9,
					BackgroundColor:  "#000000",
					TextColor:        "#FFFFFF",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#333333",
					FontSize:         73,
				},
			},
			wantErr: true,
		},
		{
			name: "negative border radius",
			config: &config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "AB",
					Opacity:          0.9,
					BackgroundColor:  "#000000",
					TextColor:        "#FFFFFF",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#333333",
					FontSize:         14,
					BorderRadius:     -1,
				},
			},
			wantErr: true,
		},
		{
			name: "empty clickable role",
			config: &config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "AB",
					Opacity:          0.9,
					BackgroundColor:  "#000000",
					TextColor:        "#FFFFFF",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#333333",
					FontSize:         14,
					ClickableRoles:   []string{"button", ""},
				},
			},
			wantErr: true,
		},
		{
			name: "empty electron bundle",
			config: &config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "AB",
					Opacity:          0.9,
					BackgroundColor:  "#000000",
					TextColor:        "#FFFFFF",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#333333",
					FontSize:         14,
					AdditionalAXSupport: config.AdditionalAXSupport{
						AdditionalElectronBundles: []string{""},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty chromium bundle",
			config: &config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "AB",
					Opacity:          0.9,
					BackgroundColor:  "#000000",
					TextColor:        "#FFFFFF",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#333333",
					FontSize:         14,
					AdditionalAXSupport: config.AdditionalAXSupport{
						AdditionalChromiumBundles: []string{""},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty firefox bundle",
			config: &config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "AB",
					Opacity:          0.9,
					BackgroundColor:  "#000000",
					TextColor:        "#FFFFFF",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#333333",
					FontSize:         14,
					AdditionalAXSupport: config.AdditionalAXSupport{
						AdditionalFirefoxBundles: []string{""},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid additional bundles",
			config: &config.Config{
				Hints: config.HintsConfig{
					HintCharacters:   "AB",
					Opacity:          0.9,
					BackgroundColor:  "#000000",
					TextColor:        "#FFFFFF",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#333333",
					FontSize:         14,
					AdditionalAXSupport: config.AdditionalAXSupport{
						AdditionalElectronBundles: []string{"com.electron.app"},
						AdditionalChromiumBundles: []string{"com.chromium.app"},
						AdditionalFirefoxBundles:  []string{"org.mozilla.firefox"},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			validateHintsErr := testCase.config.ValidateHints()
			if (validateHintsErr != nil) != testCase.wantErr {
				t.Errorf(
					"Config.ValidateHints() error = %v, wantErr %v",
					validateHintsErr,
					testCase.wantErr,
				)
			}
		})
	}
}

// TestConfig_ValidateAppConfigs tests the Config.validateAppConfigs method.
func TestConfig_ValidateAppConfigs(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name: "valid app config",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:            "com.example.app",
							AdditionalClickable: []string{"button", "link"},
						},
					},
				},
				Hotkeys: config.HotkeysConfig{
					Bindings: map[string]string{
						"Cmd+Space": "hints",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty bundle ID",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID: "",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "whitespace bundle ID",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID: "   ",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty clickable role",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:            "com.example.app",
							AdditionalClickable: []string{"button", ""},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "whitespace clickable role",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:            "com.example.app",
							AdditionalClickable: []string{"button", "   "},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty hotkey binding key",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID: "com.example.app",
						},
					},
				},
				Hotkeys: config.HotkeysConfig{
					Bindings: map[string]string{
						"": "hints",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty hotkey binding value",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID: "com.example.app",
						},
					},
				},
				Hotkeys: config.HotkeysConfig{
					Bindings: map[string]string{
						"Cmd+Space": "",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid hotkey format",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID: "com.example.app",
						},
					},
				},
				Hotkeys: config.HotkeysConfig{
					Bindings: map[string]string{
						"Invalid+": "hints",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple app configs",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:            "com.example.app1",
							AdditionalClickable: []string{"button"},
						},
						{
							BundleID:            "com.example.app2",
							AdditionalClickable: []string{"link"},
						},
					},
				},
				Hotkeys: config.HotkeysConfig{
					Bindings: map[string]string{
						"Cmd+Space": "hints",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			validateAppConfigsErr := testCase.config.ValidateAppConfigs()
			if (validateAppConfigsErr != nil) != testCase.wantErr {
				t.Errorf(
					"Config.ValidateAppConfigs() error = %v, wantErr %v",
					validateAppConfigsErr,
					testCase.wantErr,
				)
			}
		})
	}
}

// TestConfig_ValidateGrid tests the Config.validateGrid method.
func TestConfig_ValidateGrid(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name: "valid grid config",
			config: &config.Config{
				Grid: config.GridConfig{
					Characters:             "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					FontSize:               16,
					BorderWidth:            1,
					Opacity:                0.9,
					BackgroundColor:        "#000000",
					TextColor:              "#FFFFFF",
					MatchedTextColor:       "#FF0000",
					MatchedBackgroundColor: "#333333",
					MatchedBorderColor:     "#FF0000",
					BorderColor:            "#666666",
					SublayerKeys:           "123456789",
				},
			},
			wantErr: false,
		},
		{
			name: "empty characters",
			config: &config.Config{
				Grid: config.GridConfig{
					Characters: "",
				},
			},
			wantErr: true,
		},
		{
			name: "too few characters",
			config: &config.Config{
				Grid: config.GridConfig{
					Characters: "A",
				},
			},
			wantErr: true,
		},
		{
			name: "font size too small",
			config: &config.Config{
				Grid: config.GridConfig{
					Characters: "AB",
					FontSize:   5,
				},
			},
			wantErr: true,
		},
		{
			name: "negative border width",
			config: &config.Config{
				Grid: config.GridConfig{
					Characters:  "AB",
					FontSize:    16,
					BorderWidth: -1,
				},
			},
			wantErr: true,
		},
		{
			name: "opacity out of range",
			config: &config.Config{
				Grid: config.GridConfig{
					Characters:  "AB",
					FontSize:    16,
					BorderWidth: 1,
					Opacity:     1.5,
				},
			},
			wantErr: true,
		},
		{
			name: "sublayer keys too short",
			config: &config.Config{
				Grid: config.GridConfig{
					Characters:             "ABCDEFGH",
					FontSize:               16,
					BorderWidth:            1,
					Opacity:                0.9,
					BackgroundColor:        "#000000",
					TextColor:              "#FFFFFF",
					MatchedTextColor:       "#FF0000",
					MatchedBackgroundColor: "#333333",
					MatchedBorderColor:     "#FF0000",
					BorderColor:            "#666666",
					SublayerKeys:           "12345", // Less than 9 required
				},
			},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			validateGridErr := testCase.config.ValidateGrid()
			if (validateGridErr != nil) != testCase.wantErr {
				t.Errorf(
					"Config.ValidateGrid() error = %v, wantErr %v",
					validateGridErr,
					testCase.wantErr,
				)
			}
		})
	}
}

// TestConfig_ValidateAction tests the Config.validateAction method.
func TestConfig_ValidateAction(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name: "valid action config",
			config: &config.Config{
				Action: config.ActionConfig{
					HighlightWidth: 2,
					HighlightColor: "#FF0000",
				},
			},
			wantErr: false,
		},
		{
			name: "highlight width too small",
			config: &config.Config{
				Action: config.ActionConfig{
					HighlightWidth: 0,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid highlight color",
			config: &config.Config{
				Action: config.ActionConfig{
					HighlightWidth: 2,
					HighlightColor: "invalid",
				},
			},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			validateActionErr := testCase.config.ValidateAction()
			if (validateActionErr != nil) != testCase.wantErr {
				t.Errorf(
					"Config.ValidateAction() error = %v, wantErr %v",
					validateActionErr,
					testCase.wantErr,
				)
			}
		})
	}
}

// TestConfig_ValidateSmoothCursor tests the Config.validateSmoothCursor method.
func TestConfig_ValidateSmoothCursor(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name: "valid smooth cursor config",
			config: &config.Config{
				SmoothCursor: config.SmoothCursorConfig{
					Steps: 10,
					Delay: 5,
				},
			},
			wantErr: false,
		},
		{
			name: "steps too small",
			config: &config.Config{
				SmoothCursor: config.SmoothCursorConfig{
					Steps: 0,
				},
			},
			wantErr: true,
		},
		{
			name: "negative delay",
			config: &config.Config{
				SmoothCursor: config.SmoothCursorConfig{
					Steps: 10,
					Delay: -1,
				},
			},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			validateSmoothCursorErr := testCase.config.ValidateSmoothCursor()
			if (validateSmoothCursorErr != nil) != testCase.wantErr {
				t.Errorf(
					"Config.validateSmoothCursor() error = %v, wantErr %v",
					validateSmoothCursorErr,
					testCase.wantErr,
				)
			}
		})
	}
}
