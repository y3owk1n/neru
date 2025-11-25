package config_test

import (
	"path/filepath"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
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

func TestConfig_GetClickableRolesForApp(t *testing.T) {
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
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := testCase.config.GetClickableRolesForApp(testCase.bundleID)

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
					"GetClickableRolesForApp() length = %d, want %d",
					len(got),
					len(testCase.want),
				)
				t.Errorf("Got: %v", got)
				t.Errorf("Want: %v", testCase.want)

				return
			}

			for role := range wantMap {
				if !gotMap[role] {
					t.Errorf("GetClickableRolesForApp() missing role %q", role)
				}
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

	result := config.FindConfigFile()

	// Result should be a string (could be empty if no config found)
	if result != "" {
		// If a config file is found, it should be a valid path
		if !filepath.IsAbs(result) {
			t.Errorf("FindConfigFile() returned relative path: %s", result)
		}
	}
}
