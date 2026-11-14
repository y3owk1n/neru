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
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

const (
	goosDarwin = "darwin"

	testBundleIDA       = "com.example.app"
	testBundleIDB       = "com.other.app"
	testBundleIDSafari  = "com.apple.Safari"
	testRoleButton      = TestRoleButton
	testRoleTextField   = TestRoleTextField
	testActionLeftClick = "action left_click"
	testKeyReturn       = "Return"
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

func TestFindConfigFile(t *testing.T) {
	// Only checks that FindConfigFile does not panic. The actual file discovery
	// would need complex mocking.
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
	// at ours. It reads USERPROFILE on Windows and HOME on Unix.
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
						// Linux and Windows, and distinct only on macOS.
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

	// Exactly one of the three prefixed entries applies here. The other two
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

// TestUsePrimaryModifier_ResolvesPrimaryHotkeys pins that the table the daemon
// publishes is what a Primary hotkey means to every key comparison after it.
func TestUsePrimaryModifier_ResolvesPrimaryHotkeys(t *testing.T) {
	t.Cleanup(func() {
		config.UsePrimaryModifier(config.DefaultConfig().General.PrimaryModifier)
	})

	const option = "Option"

	config.UsePrimaryModifier(config.PrimaryModifierConfig{
		Darwin:  option,
		Linux:   option,
		Windows: option,
	})

	if got := keyvocab.CanonicalHotkeyForPlatform("Primary+Space"); got != "Alt+Space" {
		t.Errorf(`CanonicalHotkeyForPlatform("Primary+Space") = %q, want "Alt+Space"`, got)
	}

	if got := keyvocab.NormalizeKeyForComparison("Primary+Shift+X"); got != "alt+shift+x" {
		t.Errorf(`NormalizeKeyForComparison("Primary+Shift+X") = %q, want "alt+shift+x"`, got)
	}

	// An action's --modifier primary means the same modifier a hotkey does.
	got, err := action.ParseModifiers("primary")
	if err != nil || got != action.ModAlt {
		t.Errorf(`ParseModifiers("primary") = %v, %v, want Alt`, got, err)
	}
}
