package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
)

// scopedSection is one section that carries a capture scope, with the ways a
// test reaches it: the validator, the section's scope and its app entries.
// defaultScope is the scope the section ships with.
type scopedSection struct {
	name          string
	defaultScope  string
	validate      func(cfg *config.Config) error
	setScope      func(cfg *config.Config, scope string)
	setAppConfigs func(cfg *config.Config, entries []config.AppConfig)
	hasOverrides  func(cfg *config.Config) bool
	forApp        func(cfg *config.Config, bundleID string) string
}

func scopedSections() []scopedSection {
	return []scopedSection{
		{
			name:         "grid",
			defaultScope: domain.CaptureScopeScreen,
			validate: func(cfg *config.Config) error {
				return cfg.ValidateGrid(nil, config.WrittenConfig{})
			},
			setScope: func(cfg *config.Config, scope string) { cfg.Grid.CaptureScope = scope },
			setAppConfigs: func(cfg *config.Config, entries []config.AppConfig) {
				cfg.Grid.AppConfigs = entries
			},
			hasOverrides: func(cfg *config.Config) bool { return cfg.Grid.HasAppCaptureScopeOverrides() },
			forApp: func(cfg *config.Config, bundleID string) string {
				return cfg.Grid.CaptureScopeForApp(bundleID)
			},
		},
		{
			name:         "recursive_grid",
			defaultScope: domain.CaptureScopeScreen,
			validate:     func(cfg *config.Config) error { return cfg.ValidateRecursiveGrid() },
			setScope:     func(cfg *config.Config, scope string) { cfg.RecursiveGrid.CaptureScope = scope },
			setAppConfigs: func(cfg *config.Config, entries []config.AppConfig) {
				cfg.RecursiveGrid.AppConfigs = entries
			},
			hasOverrides: func(cfg *config.Config) bool {
				return cfg.RecursiveGrid.HasAppCaptureScopeOverrides()
			},
			forApp: func(cfg *config.Config, bundleID string) string {
				return cfg.RecursiveGrid.CaptureScopeForApp(bundleID)
			},
		},
		{
			name:         "bisect",
			defaultScope: domain.CaptureScopeScreen,
			validate:     func(cfg *config.Config) error { return cfg.ValidateBisect() },
			setScope:     func(cfg *config.Config, scope string) { cfg.Bisect.CaptureScope = scope },
			setAppConfigs: func(cfg *config.Config, entries []config.AppConfig) {
				cfg.Bisect.AppConfigs = entries
			},
			hasOverrides: func(cfg *config.Config) bool { return cfg.Bisect.HasAppCaptureScopeOverrides() },
			forApp: func(cfg *config.Config, bundleID string) string {
				return cfg.Bisect.CaptureScopeForApp(bundleID)
			},
		},
		{
			// Vim reads the focused window by default, so its section default is
			// the one the other sections keep for an override.
			name:         "vim",
			defaultScope: domain.CaptureScopeWindow,
			validate:     func(cfg *config.Config) error { return cfg.ValidateVim() },
			setScope:     func(cfg *config.Config, scope string) { cfg.Vim.CaptureScope = scope },
			setAppConfigs: func(cfg *config.Config, entries []config.AppConfig) {
				cfg.Vim.AppConfigs = entries
			},
			hasOverrides: func(cfg *config.Config) bool { return cfg.Vim.HasAppCaptureScopeOverrides() },
			forApp: func(cfg *config.Config, bundleID string) string {
				return cfg.Vim.CaptureScopeForApp(bundleID)
			},
		},
	}
}

func TestConfig_ValidateGridModes_AcceptsWindowScope(t *testing.T) {
	for _, section := range scopedSections() {
		t.Run(section.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			section.setScope(cfg, domain.CaptureScopeWindow)
			section.setAppConfigs(cfg, []config.AppConfig{
				{BundleID: bundleExample, CaptureScope: domain.CaptureScopeScreen},
			})

			err := section.validate(cfg)
			if err != nil {
				t.Fatalf("the window scope was rejected: %v", err)
			}
		})
	}
}

func TestConfig_ValidateGridModes_RejectsAnUnknownScope(t *testing.T) {
	for _, section := range scopedSections() {
		t.Run(section.name+" section", func(t *testing.T) {
			cfg := config.DefaultConfig()
			section.setScope(cfg, unknownScope)

			err := section.validate(cfg)
			if err == nil {
				t.Fatal("an unknown capture scope was accepted")
			}
		})

		t.Run(section.name+" app config", func(t *testing.T) {
			cfg := config.DefaultConfig()
			section.setAppConfigs(cfg, []config.AppConfig{
				{BundleID: bundleExample, CaptureScope: unknownScope},
			})

			err := section.validate(cfg)
			if err == nil {
				t.Fatal("an unknown per-app capture scope was accepted")
			}
		})
	}
}

// TestGridModes_CaptureScopeForApp_ShadowsTheSection pins that a per-app
// capture scope replaces the section's for that app alone, and that an app
// without one, or with no entry, gets the scope the section ships with.
func TestGridModes_CaptureScopeForApp_ShadowsTheSection(t *testing.T) {
	for _, section := range scopedSections() {
		t.Run(section.name, func(t *testing.T) {
			cfg := config.DefaultConfig()

			if section.hasOverrides(cfg) {
				t.Fatal("the defaults report a per-app capture scope")
			}

			override := domain.CaptureScopeWindow
			if section.defaultScope == domain.CaptureScopeWindow {
				override = domain.CaptureScopeScreen
			}

			section.setAppConfigs(cfg, []config.AppConfig{
				{BundleID: "com.example.Overridden", CaptureScope: override},
				{BundleID: "com.example.Plain"},
			})

			if !section.hasOverrides(cfg) {
				t.Fatal("an app config with a capture scope was not reported")
			}

			if got := section.forApp(cfg, "com.example.overridden"); got != override {
				t.Fatalf("scope for the app that sets one = %q, want %q", got, override)
			}

			for _, bundleID := range []string{"com.example.Plain", "com.example.Unknown"} {
				if got := section.forApp(cfg, bundleID); got != section.defaultScope {
					t.Fatalf("scope for %s = %q, want the section's %q",
						bundleID, got, section.defaultScope)
				}
			}
		})
	}
}
