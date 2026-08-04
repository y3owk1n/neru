package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/domain/element"
)

const (
	goosDarwin      = "darwin"
	isDarwinRuntime = runtime.GOOS == goosDarwin

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
			want:     []string{TestRoleButton, "menubar_item"},
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
			want:     []string{TestRoleButton, "dock_item"},
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
			want:     []string{TestRoleButton, "menubar_item", "dock_item"},
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
							AdditionalClickable: []string{TestRoleTabGroup},
						},
						{
							BundleID:            "com.firefox.app",
							AdditionalClickable: []string{TestRoleWebArea},
						},
					},
				},
			},
			bundleID: "com.chrome.app",
			want:     []string{TestRoleButton, TestRoleLink, TestRoleTabGroup},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := testCase.config.ClickableRolesForApp(testCase.bundleID)

			// ClickableRolesForApp returns native role names for the running
			// platform, so the expectation is resolved the same way. This keeps
			// the assertion about merge behavior rather than about which
			// platform the test happens to run on.
			want := element.ResolveRolesForCurrentPlatform(testCase.want).Native

			// Convert to maps for comparison since order doesn't matter
			gotMap := make(map[string]bool)
			for _, role := range got {
				gotMap[role] = true
			}

			wantMap := make(map[string]bool)
			for _, role := range want {
				wantMap[role] = true
			}

			if len(gotMap) != len(wantMap) {
				t.Errorf(
					"ClickableRolesForApp() length = %d, want %d",
					len(got),
					len(want),
				)
				t.Errorf("Got: %v", got)
				t.Errorf("Want: %v", want)

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
							IgnoreClickableCheck: new(true),
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
							IgnoreClickableCheck: new(false),
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
							IgnoreClickableCheck: new(true),
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
							IgnoreClickableCheck: new(true),
						},
						{
							BundleID:             testBundleIDA,
							IgnoreClickableCheck: new(true),
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
							IgnoreClickableCheck: new(false), // app-specific false
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
							VisibleCheckEnabled: new(true),
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
							VisibleCheckEnabled: new(false),
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
							VisibleCheckEnabled: new(true),
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
							VisibleCheckEnabled: new(true),
						},
						{
							BundleID:            testBundleIDA,
							VisibleCheckEnabled: new(true),
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
							VisibleCheckEnabled: new(false), // app-specific false
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

	service := loader.NewService(config.DefaultConfig(), "", zap.NewNop(), nil)
	result := service.FindConfigFile()

	// Result should be a string (could be empty if no config found)
	if result != "" {
		// If a config file is found, it should be a valid path
		if !filepath.IsAbs(result) {
			t.Errorf("FindConfigFile() returned relative path: %s", result)
		}
	}
}

// writeConfigFile creates a minimal, valid config file at path, creating any
// missing parent directories.
func writeConfigFile(t *testing.T, path string) {
	t.Helper()

	mkdirErr := os.MkdirAll(filepath.Dir(path), 0o755)
	if mkdirErr != nil {
		t.Fatalf("MkdirAll(%q) failed: %v", filepath.Dir(path), mkdirErr)
	}

	writeErr := os.WriteFile(path, []byte("[hints]\n"), 0o600)
	if writeErr != nil {
		t.Fatalf("WriteFile(%q) failed: %v", path, writeErr)
	}
}

// TestFindConfigFile_DotConfigFallback pins that ~/.config/neru/config.toml is
// discovered even when it is not the preferred directory. On Windows the
// preferred directory is %APPDATA%\neru, so this is what lets a config carried
// over from a Unix dotfiles repo work without setting XDG_CONFIG_HOME.
func TestFindConfigFile_DotConfigFallback(t *testing.T) {
	homeDir := t.TempDir()
	preferredDir := t.TempDir()

	// Point the preferred directory at an empty location, and os.UserHomeDir
	// at ours: it reads USERPROFILE on Windows and HOME on Unix.
	if runtime.GOOS == goosWindows {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("APPDATA", preferredDir)
		t.Setenv("USERPROFILE", homeDir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", preferredDir)
		t.Setenv("HOME", homeDir)
	}

	dotConfig := filepath.Join(homeDir, ".config", "neru", "config.toml")
	writeConfigFile(t, dotConfig)

	service := loader.NewService(config.DefaultConfig(), "", zap.NewNop(), nil)
	if got := service.FindConfigFile(); got != dotConfig {
		t.Errorf("FindConfigFile() = %q, want %q", got, dotConfig)
	}
}

// TestFindConfigFile_PrefersPreferredDir pins that the ~/.config fallback does
// not shadow the platform's preferred directory when both exist.
func TestFindConfigFile_PrefersPreferredDir(t *testing.T) {
	homeDir := t.TempDir()
	preferredDir := t.TempDir()

	if runtime.GOOS == goosWindows {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("APPDATA", preferredDir)
		t.Setenv("USERPROFILE", homeDir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", preferredDir)
		t.Setenv("HOME", homeDir)
	}

	preferred := filepath.Join(preferredDir, "neru", "config.toml")
	dotConfig := filepath.Join(homeDir, ".config", "neru", "config.toml")

	for _, path := range []string{preferred, dotConfig} {
		writeConfigFile(t, path)
	}

	service := loader.NewService(config.DefaultConfig(), "", zap.NewNop(), nil)
	if got := service.FindConfigFile(); got != preferred {
		t.Errorf("FindConfigFile() = %q, want %q", got, preferred)
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

// TestIsValidNamedKey_FunctionKeys pins the full F1-F24 range as valid named
// keys, case-insensitively, and confirms F25 is not silently accepted.
func TestIsValidNamedKey_FunctionKeys(t *testing.T) {
	for index := 1; index <= 24; index++ {
		display := "F" + strconv.Itoa(index)
		lower := strings.ToLower(display)

		t.Run(display, func(t *testing.T) {
			if !config.IsValidNamedKey(display) {
				t.Errorf("IsValidNamedKey(%q) = false, want true", display)
			}

			if !config.IsValidNamedKey(lower) {
				t.Errorf("IsValidNamedKey(%q) = false, want true", lower)
			}

			canonical, recognized := config.CanonicalNamedKeyForm(lower)
			if !recognized || canonical != display {
				t.Errorf(
					"CanonicalNamedKeyForm(%q) = (%q, %t), want (%q, true)",
					lower,
					canonical,
					recognized,
					display,
				)
			}
		})
	}

	if config.IsValidNamedKey("F25") {
		t.Error(`IsValidNamedKey("F25") = true, want false`)
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

// TestConfig_ClickableRolesForApp_MixedVocabulary covers app-config merging in
// the presence of the role vocabulary. Merging happens on the entries as
// written, resolution afterwards, so two different entries can legitimately
// expand onto the same native role and must not produce duplicates.
func TestConfig_ClickableRolesForApp_MixedVocabulary(t *testing.T) {
	cfg := config.Config{
		Hints: config.HintsConfig{
			ClickableRoles: []string{TestRoleTextField, TestRoleButton},
			AppConfigs: []config.AppConfig{
				{
					BundleID: testBundleIDA,
					AdditionalClickable: []string{
						// Resolves onto the same native role as text_field on
						// Linux and Windows; distinct only on macOS.
						"text_area",
						// Already in the base list: must not duplicate.
						TestRoleButton,
						// Applies on exactly one platform.
						"ax:AXGenericElement",
						"atspi:page tab list",
						"uia:Custom",
					},
				},
			},
		},
	}

	got := cfg.ClickableRolesForApp(testBundleIDA)

	seen := make(map[string]int, len(got))
	for _, role := range got {
		seen[role]++
	}

	for role, count := range seen {
		if count > 1 {
			t.Errorf("ClickableRolesForApp() returned %q %d times, want once", role, count)
		}
	}

	// Whatever the platform, the base roles must survive the merge.
	for _, want := range element.ResolveRolesForCurrentPlatform(
		[]string{TestRoleTextField, TestRoleButton},
	).Native {
		if seen[want] == 0 {
			t.Errorf("ClickableRolesForApp() = %v, missing base role %q", got, want)
		}
	}

	// Exactly one of the three prefixed entries applies here; the other two
	// must resolve to nothing rather than leaking across platforms.
	exclusive := map[string]string{
		goosDarwin: "AXGenericElement",
		"linux":    "page tab list",
		"windows":  "Custom",
	}

	for goos, name := range exclusive {
		switch {
		case goos == runtime.GOOS && seen[name] == 0:
			t.Errorf("ClickableRolesForApp() = %v, missing platform-native role %q", got, name)
		case goos != runtime.GOOS && seen[name] > 0:
			t.Errorf("ClickableRolesForApp() = %v, leaked %s-only role %q", got, goos, name)
		}
	}
}
