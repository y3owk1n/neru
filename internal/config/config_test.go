package config_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

const (
	isDarwinRuntime = runtime.GOOS == "darwin"

	testBundleIDA       = "com.example.app"
	testBundleIDB       = "com.other.app"
	testBundleIDSafari  = "com.apple.Safari"
	testRoleButton      = TestRoleButton
	testRoleTextField   = TestRoleTextField
	testActionLeftClick = "action left_click"
	testKeyReturn       = "Return"
	testKeyEscape       = "escape"
	testKeySpace        = "space"
	testKeyShiftReturn  = "shift+return"
	testKeyCmdSpace     = KeyCmdSpace
	testKeySuperSpace   = KeySuperSpace
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
			bundleID: testBundleIDA,
			want:     false,
		},
		{
			name:     "exact match",
			excluded: []string{testBundleIDA},
			bundleID: testBundleIDA,
			want:     true,
		},
		{
			name:     "case insensitive match",
			excluded: []string{"COM.EXAMPLE.APP"},
			bundleID: testBundleIDA,
			want:     true,
		},
		{
			name:     "partial match",
			excluded: []string{bundleExample},
			bundleID: testBundleIDA,
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
			excluded: []string{testBundleIDA},
			bundleID: "",
			want:     false,
		},
		{
			name:     "whitespace in bundle ID",
			excluded: []string{testBundleIDA},
			bundleID: " com.example.app ",
			want:     true,
		},
		{
			name:     "whitespace in excluded list",
			excluded: []string{" com.example.app "},
			bundleID: testBundleIDA,
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
					ClickableRoles: []string{TestRoleButton, TestRoleLink},
				},
			},
			bundleID: testBundleIDA,
			want:     []string{TestRoleButton, TestRoleLink},
		},
		{
			name: "with app-specific roles",
			config: config.Config{
				Hints: config.HintsConfig{
					ClickableRoles: []string{TestRoleButton, TestRoleLink},
					AppConfigs: []config.AppConfig{
						{
							BundleID: testBundleIDA,
							AdditionalClickable: []string{
								TestRoleTextField,
								TestRoleButton,
							}, // Button is duplicate
						},
					},
				},
			},
			bundleID: testBundleIDA,
			want: []string{
				TestRoleButton,
				TestRoleLink,
				TestRoleTextField,
			}, // Should be deduplicated
		},
		{
			name: "with menubar hints",
			config: config.Config{
				Hints: config.HintsConfig{
					ClickableRoles:      []string{TestRoleButton},
					IncludeMenubarHints: true,
				},
			},
			bundleID: testBundleIDA,
			want:     []string{TestRoleButton, "AXMenuBarItem"},
		},
		{
			name: "with dock hints",
			config: config.Config{
				Hints: config.HintsConfig{
					ClickableRoles:   []string{TestRoleButton},
					IncludeDockHints: true,
				},
			},
			bundleID: testBundleIDA,
			want:     []string{TestRoleButton, "AXDockItem"},
		},
		{
			name: "with both menubar and dock hints",
			config: config.Config{
				Hints: config.HintsConfig{
					ClickableRoles:      []string{TestRoleButton},
					IncludeMenubarHints: true,
					IncludeDockHints:    true,
				},
			},
			bundleID: testBundleIDA,
			want:     []string{TestRoleButton, "AXMenuBarItem", "AXDockItem"},
		},
		{
			name: "empty roles filtered out",
			config: config.Config{
				Hints: config.HintsConfig{
					ClickableRoles: []string{TestRoleButton, "", TestRoleLink, " "},
				},
			},
			bundleID: testBundleIDA,
			want:     []string{TestRoleButton, TestRoleLink},
		},
		{
			name: "non-matching app config",
			config: config.Config{
				Hints: config.HintsConfig{
					ClickableRoles: []string{TestRoleButton},
					AppConfigs: []config.AppConfig{
						{
							BundleID:            testBundleIDB,
							AdditionalClickable: []string{TestRoleTextField},
						},
					},
				},
			},
			bundleID: testBundleIDA,
			want:     []string{TestRoleButton},
		},
		{
			name: "multiple apps with different configs",
			config: config.Config{
				Hints: config.HintsConfig{
					ClickableRoles: []string{TestRoleButton, TestRoleLink},
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
			want:     []string{TestRoleButton, TestRoleLink, "AXTabGroup"},
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
			bundleID: testBundleIDA,
			want:     false,
		},
		{
			name: "app config with matching bundle ID and ignore true",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:             testBundleIDA,
							IgnoreClickableCheck: true,
						},
					},
				},
			},
			bundleID: testBundleIDA,
			want:     true,
		},
		{
			name: "app config with matching bundle ID and ignore false",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:             testBundleIDA,
							IgnoreClickableCheck: false,
						},
					},
				},
			},
			bundleID: testBundleIDA,
			want:     false,
		},
		{
			name: "app config with non-matching bundle ID",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:             testBundleIDB,
							IgnoreClickableCheck: true,
						},
					},
				},
			},
			bundleID: testBundleIDA,
			want:     false,
		},
		{
			name: "multiple app configs, one matching",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:             testBundleIDB,
							IgnoreClickableCheck: true,
						},
						{
							BundleID:             testBundleIDA,
							IgnoreClickableCheck: true,
						},
					},
				},
			},
			bundleID: testBundleIDA,
			want:     true,
		},
		{
			name: "global ignore clickable check true",
			config: &config.Config{
				Hints: config.HintsConfig{
					IgnoreClickableCheck: true,
				},
			},
			bundleID: testBundleIDA,
			want:     true,
		},
		{
			name: "app config overrides global ignore clickable check",
			config: &config.Config{
				Hints: config.HintsConfig{
					IgnoreClickableCheck: true, // global true
					AppConfigs: []config.AppConfig{
						{
							BundleID:             testBundleIDA,
							IgnoreClickableCheck: false, // app-specific false
						},
					},
				},
			},
			bundleID: testBundleIDA,
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

func TestConfig_AppConfigVisibleCheckEnabled(t *testing.T) {
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
			bundleID: testBundleIDA,
			want:     false,
		},
		{
			name: "app config with matching bundle ID and visible check true",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:            testBundleIDA,
							VisibleCheckEnabled: true,
						},
					},
				},
			},
			bundleID: testBundleIDA,
			want:     true,
		},
		{
			name: "app config with matching bundle ID and visible check false",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:            testBundleIDA,
							VisibleCheckEnabled: false,
						},
					},
				},
			},
			bundleID: testBundleIDA,
			want:     false,
		},
		{
			name: "app config with non-matching bundle ID",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:            testBundleIDB,
							VisibleCheckEnabled: true,
						},
					},
				},
			},
			bundleID: testBundleIDA,
			want:     false,
		},
		{
			name: "multiple app configs, one matching",
			config: &config.Config{
				Hints: config.HintsConfig{
					AppConfigs: []config.AppConfig{
						{
							BundleID:            testBundleIDB,
							VisibleCheckEnabled: true,
						},
						{
							BundleID:            testBundleIDA,
							VisibleCheckEnabled: true,
						},
					},
				},
			},
			bundleID: testBundleIDA,
			want:     true,
		},
		{
			name: "global visible check enabled true",
			config: &config.Config{
				Hints: config.HintsConfig{
					VisibleCheckEnabled: true,
				},
			},
			bundleID: testBundleIDA,
			want:     true,
		},
		{
			name: "app config overrides global visible check",
			config: &config.Config{
				Hints: config.HintsConfig{
					VisibleCheckEnabled: true, // global true
					AppConfigs: []config.AppConfig{
						{
							BundleID:            testBundleIDA,
							VisibleCheckEnabled: false, // app-specific false
						},
					},
				},
			},
			bundleID: testBundleIDA,
			want:     false, // app-specific should take precedence
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := testCase.config.ShouldEnableVisibleCheckForApp(testCase.bundleID)
			if got != testCase.want {
				t.Errorf("ShouldEnableVisibleCheckForApp() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestConfig_HotkeysForModeAndApp(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.Hotkeys[testKeyReturn] = config.StringOrStringArray{
		testActionLeftClick, config.ModeNameHints,
	}
	cfg.Hints.Hotkeys["g"] = config.StringOrStringArray{testActionLeftClick}
	cfg.Hints.AppConfigs = []config.AppConfig{
		{
			BundleID: testBundleIDSafari,
			Hotkeys: map[string]config.StringOrStringArray{
				testKeyReturn: {testActionLeftClick, config.ModeNameHints},
				"g":           {config.DisabledSentinel},
				"x":           {"action right_click"},
			},
		},
	}

	got := cfg.HotkeysForModeAndApp(config.ModeNameHints, testBundleIDSafari)

	if actions := got[testKeyReturn]; len(actions) != 2 || actions[1] != config.ModeNameHints {
		t.Fatalf("HotkeysForModeAndApp() did not apply app override for Return: %v", actions)
	}

	if _, exists := got["g"]; exists {
		t.Fatal("HotkeysForModeAndApp() did not remove disabled inherited binding")
	}

	if actions := got["x"]; len(actions) != 1 || actions[0] != "action right_click" {
		t.Fatalf("HotkeysForModeAndApp() did not include app-specific binding: %v", actions)
	}

	base := cfg.HotkeysForMode(config.ModeNameHints)
	if actions := base[testKeyReturn]; len(actions) != 2 || actions[1] != config.ModeNameHints {
		t.Fatalf("HotkeysForMode() unexpectedly mutated base bindings: %v", actions)
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

	service := config.NewService(config.DefaultConfig(), "", zap.NewNop(), nil)
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
			expected: testKeySpace,
		},
		// Regular space (should also normalize to "space")
		{
			name:     "regular space",
			input:    " ",
			expected: testKeySpace,
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
			name:     testKeyEscape,
			input:    testKeyEscape,
			expected: testKeyEscape,
		},
		{
			name:     "fullwidth escape letters normalize to canonical escape",
			input:    "\uFF25\uFF33\uFF23\uFF21\uFF30\uFF25",
			expected: testKeyEscape,
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

func TestNormalizeKeyForComparison_ModifierComboAliases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Enter/Return aliases in modifier combos
		{
			name:     "Shift+Enter normalizes to shift+return",
			input:    "Shift+Enter",
			expected: testKeyShiftReturn,
		},
		{
			name:     "Shift+Return normalizes to shift+return",
			input:    "Shift+Return",
			expected: testKeyShiftReturn,
		},
		{
			name:     "Cmd+Enter normalizes to cmd+return",
			input:    "Cmd+Enter",
			expected: "cmd+return",
		},
		{
			name:     "Cmd+Shift+Enter normalizes to cmd+shift+return",
			input:    "Cmd+Shift+Enter",
			expected: "cmd+shift+return",
		},
		// Bare Enter still works
		{
			name:     "bare Enter normalizes to return",
			input:    "Enter",
			expected: "return",
		},
		{
			name:     "bare Return normalizes to return",
			input:    testKeyReturn,
			expected: "return",
		},
		// Backspace/Delete aliases in modifier combos
		{
			name:     "Shift+Backspace normalizes to shift+delete",
			input:    "Shift+Backspace",
			expected: "shift+delete",
		},
		{
			name:     "Cmd+Backspace normalizes to cmd+delete",
			input:    "Cmd+Backspace",
			expected: "cmd+delete",
		},
		// Esc alias in modifier combos
		{
			name:     "Ctrl+Esc normalizes to ctrl+escape",
			input:    "Ctrl+Esc",
			expected: "ctrl+escape",
		},
		// Non-aliased keys should pass through
		{
			name:     "Shift+Space unchanged",
			input:    "Shift+Space",
			expected: "shift+space",
		},
		{
			name:     "Cmd+L unchanged",
			input:    "Cmd+L",
			expected: "cmd+l",
		},
		// Canonical forms must pass through unchanged (regression: +esc prefix of +escape)
		{
			name:     "Ctrl+Escape stays ctrl+escape",
			input:    "Ctrl+Escape",
			expected: "ctrl+escape",
		},
		{
			name:     "Shift+Return stays shift+return",
			input:    "Shift+Return",
			expected: testKeyShiftReturn,
		},
		{
			name:     "Cmd+Delete stays cmd+delete",
			input:    "Cmd+Delete",
			expected: "cmd+delete",
		},
		{
			name:     "Primary+Space normalizes to platform primary modifier",
			input:    "Primary+Space",
			expected: map[bool]string{true: "cmd+space", false: "ctrl+space"}[isDarwinRuntime],
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

func TestCanonicalHotkeyForPlatform(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "primary modifier becomes current platform token",
			input:    "Primary+Space",
			expected: map[bool]string{true: testKeyCmdSpace, false: "Ctrl+Space"}[isDarwinRuntime],
		},
		{
			name:     "named key is canonicalized",
			input:    "Primary+enter",
			expected: map[bool]string{true: "Cmd+Enter", false: "Ctrl+Enter"}[isDarwinRuntime],
		},
		{
			name:     "super alias becomes platform cmd token",
			input:    KeySuperSpace,
			expected: map[bool]string{true: testKeyCmdSpace, false: testKeySuperSpace}[isDarwinRuntime],
		},
		{
			name:     "meta alias becomes platform cmd token",
			input:    "Meta+Space",
			expected: map[bool]string{true: testKeyCmdSpace, false: testKeySuperSpace}[isDarwinRuntime],
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := config.CanonicalHotkeyForPlatform(testCase.input)
			if got != testCase.expected {
				t.Fatalf(
					"CanonicalHotkeyForPlatform(%q) = %q, want %q",
					testCase.input,
					got,
					testCase.expected,
				)
			}
		})
	}
}

func TestHasPassthroughModifier(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{name: "cmd combo", key: "Cmd+Tab", want: true},
		{name: "ctrl combo", key: "Ctrl+D", want: true},
		{name: "option combo", key: "Option+Space", want: true},
		{name: "shift only combo", key: "Shift+Tab", want: false},
		{name: "plain key", key: "j", want: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := config.HasPassthroughModifier(testCase.key)
			if got != testCase.want {
				t.Errorf(
					"HasPassthroughModifier(%q) = %v, want %v",
					testCase.key,
					got,
					testCase.want,
				)
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
			expected: testKeySpace,
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
