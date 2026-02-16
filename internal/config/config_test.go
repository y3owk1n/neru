package config_test

import (
	"path/filepath"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"go.uber.org/zap"
)

func TestConfig_IsAppExcluded(t *testing.T) {
	tests := []struct {
		name     string
		excluded []string
		bundleID string
		want     bool
	}{
		{
			name:     "empty excluded list",
			excluded: []string{},
			bundleID: "com.example.app",
			want:     false,
		},
		{
			name:     "exact match",
			excluded: []string{"com.example.app"},
			bundleID: "com.example.app",
			want:     true,
		},
		{
			name:     "case insensitive match",
			excluded: []string{"COM.EXAMPLE.APP"},
			bundleID: "com.example.app",
			want:     true,
		},
		{
			name:     "partial match",
			excluded: []string{"com.example"},
			bundleID: "com.example.app",
			want:     false,
		},
		{
			name:     "multiple excluded apps",
			excluded: []string{"com.app1", "com.app2", "com.app3"},
			bundleID: "com.app2",
			want:     true,
		},
		{
			name:     "empty bundle ID",
			excluded: []string{"com.example.app"},
			bundleID: "",
			want:     false,
		},
		{
			name:     "whitespace in bundle ID",
			excluded: []string{"com.example.app"},
			bundleID: " com.example.app ",
			want:     true,
		},
		{
			name:     "whitespace in excluded list",
			excluded: []string{" com.example.app "},
			bundleID: "com.example.app",
			want:     true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := &config.Config{
				General: config.GeneralConfig{
					ExcludedApps: testCase.excluded,
				},
			}

			got := cfg.IsAppExcluded(testCase.bundleID)
			if got != testCase.want {
				t.Errorf("IsAppExcluded(%q) = %v, want %v", testCase.bundleID, got, testCase.want)
			}
		})
	}
}

func TestConfig_ClickableRolesForApp(t *testing.T) {
	tests := []struct {
		name     string
		config   config.Config
		bundleID string
		want     []string
	}{
		{
			name: "default roles only",
			config: config.Config{
				Hints: config.HintsConfig{
					ClickableRoles: []string{"AXButton", "AXLink"},
				},
			},
			bundleID: "com.example.app",
			want:     []string{"AXButton", "AXLink"},
		},
		{
			name: "with app-specific roles",
			config: config.Config{
				Hints: config.HintsConfig{
					ClickableRoles: []string{"AXButton", "AXLink"},
					AppConfigs: []config.AppConfig{
						{
							BundleID: "com.example.app",
							AdditionalClickable: []string{
								"AXTextField",
								"AXButton",
							}, // Button is duplicate
						},
					},
				},
			},
			bundleID: "com.example.app",
			want:     []string{"AXButton", "AXLink", "AXTextField"}, // Should be deduplicated
		},
		{
			name: "with menubar hints",
			config: config.Config{
				Hints: config.HintsConfig{
					ClickableRoles:      []string{"AXButton"},
					IncludeMenubarHints: true,
				},
			},
			bundleID: "com.example.app",
			want:     []string{"AXButton", "AXMenuBarItem"},
		},
		{
			name: "with dock hints",
			config: config.Config{
				Hints: config.HintsConfig{
					ClickableRoles:   []string{"AXButton"},
					IncludeDockHints: true,
				},
			},
			bundleID: "com.example.app",
			want:     []string{"AXButton", "AXDockItem"},
		},
		{
			name: "with both menubar and dock hints",
			config: config.Config{
				Hints: config.HintsConfig{
					ClickableRoles:      []string{"AXButton"},
					IncludeMenubarHints: true,
					IncludeDockHints:    true,
				},
			},
			bundleID: "com.example.app",
			want:     []string{"AXButton", "AXMenuBarItem", "AXDockItem"},
		},
		{
			name: "empty roles filtered out",
			config: config.Config{
				Hints: config.HintsConfig{
					ClickableRoles: []string{"AXButton", "", "AXLink", " "},
				},
			},
			bundleID: "com.example.app",
			want:     []string{"AXButton", "AXLink"},
		},
		{
			name: "non-matching app config",
			config: config.Config{
				Hints: config.HintsConfig{
					ClickableRoles: []string{"AXButton"},
					AppConfigs: []config.AppConfig{
						{
							BundleID:            "com.other.app",
							AdditionalClickable: []string{"AXTextField"},
						},
					},
				},
			},
			bundleID: "com.example.app",
			want:     []string{"AXButton"},
		},
		{
			name: "multiple apps with different configs",
			config: config.Config{
				Hints: config.HintsConfig{
					ClickableRoles: []string{"AXButton", "AXLink"},
					AppConfigs: []config.AppConfig{
						{
							BundleID:            "com.chrome.app",
							AdditionalClickable: []string{"AXTabGroup"},
						},
						{
							BundleID:            "com.firefox.app",
							AdditionalClickable: []string{"AXWebArea"},
						},
					},
				},
			},
			bundleID: "com.chrome.app",
			want:     []string{"AXButton", "AXLink", "AXTabGroup"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := testCase.config.ClickableRolesForApp(testCase.bundleID)

			// Convert to maps for comparison since order doesn't matter
			gotMap := make(map[string]bool)
			for _, role := range got {
				gotMap[role] = true
			}

			wantMap := make(map[string]bool)
			for _, role := range testCase.want {
				wantMap[role] = true
			}

			if len(gotMap) != len(wantMap) {
				t.Errorf(
					"ClickableRolesForApp() length = %d, want %d",
					len(got),
					len(testCase.want),
				)
				t.Errorf("Got: %v", got)
				t.Errorf("Want: %v", testCase.want)

				return
			}

			for role := range wantMap {
				if !gotMap[role] {
					t.Errorf("ClickableRolesForApp() missing role %q", role)
				}
			}
		})
	}
}

func TestConfig_AppConfigIgnoreClickableCheck(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.Config
		bundleID string
		want     bool
	}{
		{
			name: "no app configs",
			config: &config.Config{
				Hints: config.HintsConfig{},
			},
			bundleID: "com.example.app",
			want:     false,
		},
		{
			name: "app config with matching bundle ID and ignore true",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:             "com.example.app",
							IgnoreClickableCheck: true,
						},
					},
				},
			},
			bundleID: "com.example.app",
			want:     true,
		},
		{
			name: "app config with matching bundle ID and ignore false",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:             "com.example.app",
							IgnoreClickableCheck: false,
						},
					},
				},
			},
			bundleID: "com.example.app",
			want:     false,
		},
		{
			name: "app config with non-matching bundle ID",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:             "com.other.app",
							IgnoreClickableCheck: true,
						},
					},
				},
			},
			bundleID: "com.example.app",
			want:     false,
		},
		{
			name: "multiple app configs, one matching",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:             "com.other.app",
							IgnoreClickableCheck: true,
						},
						{
							BundleID:             "com.example.app",
							IgnoreClickableCheck: true,
						},
					},
				},
			},
			bundleID: "com.example.app",
			want:     true,
		},
		{
			name: "global ignore clickable check true",
			config: &config.Config{
				Hints: config.HintsConfig{
					IgnoreClickableCheck: true,
				},
			},
			bundleID: "com.example.app",
			want:     true,
		},
		{
			name: "app config overrides global ignore clickable check",
			config: &config.Config{
				Hints: config.HintsConfig{
					IgnoreClickableCheck: true, // global true
					AppConfigs: []config.AppConfig{
						{
							BundleID:             "com.example.app",
							IgnoreClickableCheck: false, // app-specific false
						},
					},
				},
			},
			bundleID: "com.example.app",
			want:     false, // app-specific should take precedence
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := testCase.config.ShouldIgnoreClickableCheckForApp(testCase.bundleID)
			if got != testCase.want {
				t.Errorf("ShouldIgnoreClickableCheckForApp() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestFindConfigFile(t *testing.T) {
	// Test that FindConfigFile doesn't panic and returns a string
	// (We can't easily test the actual file discovery without complex mocking)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("FindConfigFile() panicked: %v", r)
		}
	}()

	service := config.NewService(config.DefaultConfig(), "", zap.NewNop())
	result := service.FindConfigFile()

	// Result should be a string (could be empty if no config found)
	if result != "" {
		// If a config file is found, it should be a valid path
		if !filepath.IsAbs(result) {
			t.Errorf("FindConfigFile() returned relative path: %s", result)
		}
	}
}

func TestNormalizeKeyForComparison_FullwidthChars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Fullwidth comma (most common case - reset key)
		{
			name:     "fullwidth comma",
			input:    "\uFF0C",
			expected: ",",
		},
		{
			name:     "fullwidth comma uppercase",
			input:    "\uFF0C",
			expected: ",",
		},
		// Fullwidth space (should normalize to canonical "space")
		{
			name:     "fullwidth space U+3000",
			input:    "\u3000",
			expected: "space",
		},
		// Regular space (should also normalize to "space")
		{
			name:     "regular space",
			input:    " ",
			expected: "space",
		},
		// Other fullwidth punctuation
		{
			name:     "fullwidth period",
			input:    "\uFF0E",
			expected: ".",
		},
		{
			name:     "fullwidth exclamation",
			input:    "\uFF01",
			expected: "!",
		},
		{
			name:     "fullwidth question mark",
			input:    "\uFF1F",
			expected: "?",
		},
		// Fullwidth letters
		{
			name:     "fullwidth A",
			input:    "\uFF21",
			expected: "a",
		},
		{
			name:     "fullwidth z",
			input:    "\uFF5A",
			expected: "z",
		},
		// Fullwidth numbers
		{
			name:     "fullwidth 0",
			input:    "\uFF10",
			expected: "0",
		},
		{
			name:     "fullwidth 9",
			input:    "\uFF19",
			expected: "9",
		},
		// ASCII characters (should pass through unchanged)
		{
			name:     "regular comma",
			input:    ",",
			expected: ",",
		},
		{
			name:     "regular letter",
			input:    "a",
			expected: "a",
		},
		{
			name:     "regular uppercase letter",
			input:    "A",
			expected: "a",
		},
		// Special keys (should use canonical forms)
		{
			name:     "escape",
			input:    "escape",
			expected: "escape",
		},
		{
			name:     "fullwidth escape should not match",
			input:    "\uFF25\uFF33\uFF23\uFF21\uFF30\uFF25",
			expected: "escape",
		},
		// Edge cases
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "multiple fullwidth chars",
			input:    "\uFF0C\uFF0E", // fullwidth comma + period
			expected: ",.",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := config.NormalizeKeyForComparison(testCase.input)
			if got != testCase.expected {
				t.Errorf("NormalizeKeyForComparison(%q) = %q, want %q",
					testCase.input, got, testCase.expected)
			}
		})
	}
}

func TestIsResetKey_WithFullwidthChars(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		resetKey string
		want     bool
	}{
		{
			name:     "fullwidth comma matches regular comma reset",
			key:      "\uFF0C", // fullwidth comma
			resetKey: ",",
			want:     true,
		},
		{
			name:     "regular comma matches fullwidth comma reset",
			key:      ",",
			resetKey: "\uFF0C",
			want:     true,
		},
		{
			name:     "fullwidth comma matches fullwidth comma reset",
			key:      "\uFF0C",
			resetKey: "\uFF0C",
			want:     true,
		},
		{
			name:     "regular comma matches regular comma reset",
			key:      ",",
			resetKey: ",",
			want:     true,
		},
		{
			name:     "fullwidth period does not match comma reset",
			key:      "\uFF0E",
			resetKey: ",",
			want:     false,
		},
		{
			name:     "fullwidth space matches regular space reset",
			key:      "\u3000",
			resetKey: " ",
			want:     true,
		},
		{
			name:     "modifier combo not affected by fullwidth",
			key:      "Ctrl+R",
			resetKey: "ctrl+r",
			want:     true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := config.IsResetKey(testCase.key, testCase.resetKey)
			if got != testCase.want {
				t.Errorf("IsResetKey(%q, %q) = %v, want %v",
					testCase.key, testCase.resetKey, got, testCase.want)
			}
		})
	}
}

func TestIsExitKey_WithFullwidthChars(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		exitKeys []string
		want     bool
	}{
		{
			name:     "fullwidth space matches regular space exit",
			key:      "\u3000", // fullwidth space
			exitKeys: []string{" "},
			want:     true,
		},
		{
			name:     "fullwidth space matches space exit key",
			key:      "\u3000",
			exitKeys: []string{"space"},
			want:     true,
		},
		{
			name:     "regular space matches fullwidth space exit",
			key:      " ",
			exitKeys: []string{"\u3000"},
			want:     true,
		},
		{
			name:     "fullwidth comma in exit keys",
			key:      "\uFF0C",
			exitKeys: []string{","},
			want:     true,
		},
		{
			name:     "multiple exit keys with fullwidth",
			key:      "\u3000",
			exitKeys: []string{"escape", " ", "q"},
			want:     true,
		},
		{
			name:     "no matching exit key",
			key:      "\uFF0C",
			exitKeys: []string{"escape", "q"},
			want:     false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := config.IsExitKey(testCase.key, testCase.exitKeys)
			if got != testCase.want {
				t.Errorf("IsExitKey(%q, %v) = %v, want %v",
					testCase.key, testCase.exitKeys, got, testCase.want)
			}
		})
	}
}

func TestNormalizeKeyForComparison_CJKInputMethodScenarios(t *testing.T) {
	// These tests simulate real-world CJK input method scenarios
	tests := []struct {
		name     string
		input    string
		expected string
		desc     string
	}{
		{
			name:     "Chinese input comma key",
			input:    "，",
			expected: ",",
			desc:     "User presses comma key with Chinese IM active",
		},
		{
			name:     "fullwidth period key (U+FF0E)",
			input:    "\uFF0E",
			expected: ".",
			desc:     "Fullwidth period from keyboard layout",
		},
		{
			name:     "Chinese input space key",
			input:    "　",
			expected: "space",
			desc:     "User presses space key with Chinese IM active",
		},
		{
			name:     "Japanese fullwidth exclamation",
			input:    "！",
			expected: "!",
			desc:     "Japanese fullwidth exclamation mark",
		},
		{
			name:     "Korean input (also uses fullwidth chars)",
			input:    "，",
			expected: ",",
			desc:     "Korean input methods also produce fullwidth punctuation",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := config.NormalizeKeyForComparison(testCase.input)
			if got != testCase.expected {
				t.Errorf("%s: NormalizeKeyForComparison(%q) = %q, want %q",
					testCase.desc, testCase.input, got, testCase.expected)
			}
		})
	}
}
