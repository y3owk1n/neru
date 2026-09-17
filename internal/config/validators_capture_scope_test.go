package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
)

// scopedSection is one section that carries a capture scope, with the ways a
// test reaches it: the validator, the section's scope and its app entries.
type scopedSection struct {
	name          string
	validate      func(cfg *config.Config) error
	setScope      func(cfg *config.Config, scope string)
	setAppConfigs func(cfg *config.Config, entries []config.AppConfig)
	hasOverrides  func(cfg *config.Config) bool
	forApp        func(cfg *config.Config, bundleID string) string
}

func scopedSections() []scopedSection {
	return []scopedSection{
		{
			name: "grid",
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
			name:     "recursive_grid",
			validate: func(cfg *config.Config) error { return cfg.ValidateRecursiveGrid() },
			setScope: func(cfg *config.Config, scope string) { cfg.RecursiveGrid.CaptureScope = scope },
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
	}
}

func TestConfig_ValidateGridModes_DefaultScopeIsScreen(t *testing.T) {
	cfg := config.DefaultConfig()

	if cfg.Grid.CaptureScope != domain.CaptureScopeScreen {
		t.Fatalf("grid default capture scope = %q, want screen", cfg.Grid.CaptureScope)
	}

	if cfg.RecursiveGrid.CaptureScope != domain.CaptureScopeScreen {
		t.Fatalf("recursive_grid default capture scope = %q, want screen",
			cfg.RecursiveGrid.CaptureScope)
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

func TestGridModes_CaptureScopeForApp_ShadowsTheSection(t *testing.T) {
	for _, section := range scopedSections() {
		t.Run(section.name, func(t *testing.T) {
			cfg := config.DefaultConfig()

			if section.hasOverrides(cfg) {
				t.Fatal("the defaults report a per-app capture scope")
			}

			section.setAppConfigs(cfg, []config.AppConfig{
				{BundleID: "com.example.Windowed", CaptureScope: domain.CaptureScopeWindow},
				{BundleID: "com.example.Plain"},
			})

			if !section.hasOverrides(cfg) {
				t.Fatal("an app config with a capture scope was not reported")
			}

			if got := section.forApp(
				cfg,
				"com.example.windowed",
			); got != domain.CaptureScopeWindow {
				t.Fatalf("scope for the windowed app = %q, want window", got)
			}

			if got := section.forApp(cfg, "com.example.Plain"); got != domain.CaptureScopeScreen {
				t.Fatalf("scope for an app without one = %q, want the section's screen", got)
			}

			if got := section.forApp(cfg, "com.example.Unknown"); got != domain.CaptureScopeScreen {
				t.Fatalf("scope for an unlisted app = %q, want the section's screen", got)
			}
		})
	}
}
