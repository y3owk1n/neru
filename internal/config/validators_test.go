package config

import (
	"testing"
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateColor(test.color, test.fieldName)
			if (err != nil) != test.wantErr {
				t.Errorf("validateColor() error = %v, wantErr %v", err, test.wantErr)
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validateHotkeyErr := validateHotkey(test.hotkey, test.fieldName)
			if (validateHotkeyErr != nil) != test.wantErr {
				t.Errorf("validateHotkey() error = %v, wantErr %v", validateHotkeyErr, test.wantErr)
			}
		})
	}
}

// TestConfig_ValidateHints tests the Config.validateHints method.
func TestConfig_ValidateHints(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid hints config",
			config: &Config{
				Hints: HintsConfig{
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
			config: &Config{
				Hints: HintsConfig{
					HintCharacters: "",
				},
			},
			wantErr: true,
		},
		{
			name: "too few hint characters",
			config: &Config{
				Hints: HintsConfig{
					HintCharacters: "A",
				},
			},
			wantErr: true,
		},
		{
			name: "opacity too low",
			config: &Config{
				Hints: HintsConfig{
					HintCharacters: "AB",
					Opacity:        -0.1,
				},
			},
			wantErr: true,
		},
		{
			name: "opacity too high",
			config: &Config{
				Hints: HintsConfig{
					HintCharacters: "AB",
					Opacity:        1.1,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid background color",
			config: &Config{
				Hints: HintsConfig{
					HintCharacters:  "AB",
					Opacity:         0.9,
					BackgroundColor: "invalid",
				},
			},
			wantErr: true,
		},
		{
			name: "font size too small",
			config: &Config{
				Hints: HintsConfig{
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
			config: &Config{
				Hints: HintsConfig{
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
			config: &Config{
				Hints: HintsConfig{
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
			config: &Config{
				Hints: HintsConfig{
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
			config: &Config{
				Hints: HintsConfig{
					HintCharacters:   "AB",
					Opacity:          0.9,
					BackgroundColor:  "#000000",
					TextColor:        "#FFFFFF",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#333333",
					FontSize:         14,
					AdditionalAXSupport: AdditionalAXSupport{
						AdditionalElectronBundles: []string{""},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty chromium bundle",
			config: &Config{
				Hints: HintsConfig{
					HintCharacters:   "AB",
					Opacity:          0.9,
					BackgroundColor:  "#000000",
					TextColor:        "#FFFFFF",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#333333",
					FontSize:         14,
					AdditionalAXSupport: AdditionalAXSupport{
						AdditionalChromiumBundles: []string{""},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty firefox bundle",
			config: &Config{
				Hints: HintsConfig{
					HintCharacters:   "AB",
					Opacity:          0.9,
					BackgroundColor:  "#000000",
					TextColor:        "#FFFFFF",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#333333",
					FontSize:         14,
					AdditionalAXSupport: AdditionalAXSupport{
						AdditionalFirefoxBundles: []string{""},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid additional bundles",
			config: &Config{
				Hints: HintsConfig{
					HintCharacters:   "AB",
					Opacity:          0.9,
					BackgroundColor:  "#000000",
					TextColor:        "#FFFFFF",
					MatchedTextColor: "#FF0000",
					BorderColor:      "#333333",
					FontSize:         14,
					AdditionalAXSupport: AdditionalAXSupport{
						AdditionalElectronBundles: []string{"com.electron.app"},
						AdditionalChromiumBundles: []string{"com.chromium.app"},
						AdditionalFirefoxBundles:  []string{"org.mozilla.firefox"},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validateHintsErr := test.config.validateHints()
			if (validateHintsErr != nil) != test.wantErr {
				t.Errorf(
					"Config.validateHints() error = %v, wantErr %v",
					validateHintsErr,
					test.wantErr,
				)
			}
		})
	}
}

// TestConfig_ValidateAppConfigs tests the Config.validateAppConfigs method.
func TestConfig_ValidateAppConfigs(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid app config",
			config: &Config{
				Hints: HintsConfig{
					AppConfigs: []AppConfig{
						{
							BundleID:            "com.example.app",
							AdditionalClickable: []string{"button", "link"},
						},
					},
				},
				Hotkeys: HotkeysConfig{
					Bindings: map[string]string{
						"Cmd+Space": "hints",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty bundle ID",
			config: &Config{
				Hints: HintsConfig{
					AppConfigs: []AppConfig{
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
			config: &Config{
				Hints: HintsConfig{
					AppConfigs: []AppConfig{
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
			config: &Config{
				Hints: HintsConfig{
					AppConfigs: []AppConfig{
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
			config: &Config{
				Hints: HintsConfig{
					AppConfigs: []AppConfig{
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
			config: &Config{
				Hints: HintsConfig{
					AppConfigs: []AppConfig{
						{
							BundleID: "com.example.app",
						},
					},
				},
				Hotkeys: HotkeysConfig{
					Bindings: map[string]string{
						"": "hints",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty hotkey binding value",
			config: &Config{
				Hints: HintsConfig{
					AppConfigs: []AppConfig{
						{
							BundleID: "com.example.app",
						},
					},
				},
				Hotkeys: HotkeysConfig{
					Bindings: map[string]string{
						"Cmd+Space": "",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid hotkey format",
			config: &Config{
				Hints: HintsConfig{
					AppConfigs: []AppConfig{
						{
							BundleID: "com.example.app",
						},
					},
				},
				Hotkeys: HotkeysConfig{
					Bindings: map[string]string{
						"Invalid+": "hints",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple app configs",
			config: &Config{
				Hints: HintsConfig{
					AppConfigs: []AppConfig{
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
				Hotkeys: HotkeysConfig{
					Bindings: map[string]string{
						"Cmd+Space": "hints",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validateAppConfigsErr := test.config.validateAppConfigs()
			if (validateAppConfigsErr != nil) != test.wantErr {
				t.Errorf(
					"Config.validateAppConfigs() error = %v, wantErr %v",
					validateAppConfigsErr,
					test.wantErr,
				)
			}
		})
	}
}

// TestConfig_ValidateGrid tests the Config.validateGrid method.
func TestConfig_ValidateGrid(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid grid config",
			config: &Config{
				Grid: GridConfig{
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
			config: &Config{
				Grid: GridConfig{
					Characters: "",
				},
			},
			wantErr: true,
		},
		{
			name: "too few characters",
			config: &Config{
				Grid: GridConfig{
					Characters: "A",
				},
			},
			wantErr: true,
		},
		{
			name: "font size too small",
			config: &Config{
				Grid: GridConfig{
					Characters: "AB",
					FontSize:   5,
				},
			},
			wantErr: true,
		},
		{
			name: "negative border width",
			config: &Config{
				Grid: GridConfig{
					Characters:  "AB",
					FontSize:    16,
					BorderWidth: -1,
				},
			},
			wantErr: true,
		},
		{
			name: "opacity out of range",
			config: &Config{
				Grid: GridConfig{
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
			config: &Config{
				Grid: GridConfig{
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validateGridErr := test.config.validateGrid()
			if (validateGridErr != nil) != test.wantErr {
				t.Errorf(
					"Config.validateGrid() error = %v, wantErr %v",
					validateGridErr,
					test.wantErr,
				)
			}
		})
	}
}

// TestConfig_ValidateAction tests the Config.validateAction method.
func TestConfig_ValidateAction(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid action config",
			config: &Config{
				Action: ActionConfig{
					HighlightWidth: 2,
					HighlightColor: "#FF0000",
				},
			},
			wantErr: false,
		},
		{
			name: "highlight width too small",
			config: &Config{
				Action: ActionConfig{
					HighlightWidth: 0,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid highlight color",
			config: &Config{
				Action: ActionConfig{
					HighlightWidth: 2,
					HighlightColor: "invalid",
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validateActionErr := test.config.validateAction()
			if (validateActionErr != nil) != test.wantErr {
				t.Errorf(
					"Config.validateAction() error = %v, wantErr %v",
					validateActionErr,
					test.wantErr,
				)
			}
		})
	}
}

// TestConfig_ValidateSmoothCursor tests the Config.validateSmoothCursor method.
func TestConfig_ValidateSmoothCursor(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid smooth cursor config",
			config: &Config{
				SmoothCursor: SmoothCursorConfig{
					Steps: 10,
					Delay: 5,
				},
			},
			wantErr: false,
		},
		{
			name: "steps too small",
			config: &Config{
				SmoothCursor: SmoothCursorConfig{
					Steps: 0,
				},
			},
			wantErr: true,
		},
		{
			name: "negative delay",
			config: &Config{
				SmoothCursor: SmoothCursorConfig{
					Steps: 10,
					Delay: -1,
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validateSmoothCursorErr := test.config.validateSmoothCursor()
			if (validateSmoothCursorErr != nil) != test.wantErr {
				t.Errorf(
					"Config.validateSmoothCursor() error = %v, wantErr %v",
					validateSmoothCursorErr,
					test.wantErr,
				)
			}
		})
	}
}

// Benchmark tests.
func BenchmarkValidateColor(b *testing.B) {
	for b.Loop() {
		_ = validateColor("#FF0000", "test_color")
	}
}

func BenchmarkValidateHotkey(b *testing.B) {
	for b.Loop() {
		_ = validateHotkey("Cmd+Shift+Space", "test_hotkey")
	}
}

func BenchmarkValidateHints(b *testing.B) {
	config := &Config{
		Hints: HintsConfig{
			HintCharacters:   "ABCDEFGH",
			Opacity:          0.9,
			BackgroundColor:  "#000000",
			TextColor:        "#FFFFFF",
			MatchedTextColor: "#FF0000",
			BorderColor:      "#333333",
			FontSize:         14,
		},
	}

	for b.Loop() {
		_ = config.validateHints()
	}
}
