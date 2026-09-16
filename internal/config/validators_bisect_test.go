package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
)

// unknownScope is a capture scope no vocabulary declares.
const unknownScope = "desk"

func TestConfig_ValidateBisect_DefaultsAreValid(t *testing.T) {
	cfg := config.DefaultConfig()

	err := cfg.ValidateBisect()
	if err != nil {
		t.Fatalf("ValidateBisect() rejected the defaults: %v", err)
	}

	if cfg.Bisect.CaptureScope != domain.CaptureScopeScreen {
		t.Fatalf("default capture scope = %q, want screen", cfg.Bisect.CaptureScope)
	}
}

func TestConfig_ValidateBisect_RejectsBadValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(cfg *config.Config)
	}{
		{
			"unknown capture scope",
			func(cfg *config.Config) { cfg.Bisect.CaptureScope = unknownScope },
		},
		{"app config with an unknown capture scope", func(cfg *config.Config) {
			cfg.Bisect.AppConfigs = []config.AppConfig{
				{BundleID: "com.example", CaptureScope: unknownScope},
			}
		}},
		{"app config with a scroll field", func(cfg *config.Config) {
			step := 5
			cfg.Bisect.AppConfigs = []config.AppConfig{{BundleID: "com.example", ScrollStep: &step}}
		}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			testCase.mutate(cfg)

			err := cfg.ValidateBisect()
			if err == nil {
				t.Fatal("ValidateBisect() accepted an invalid value")
			}
		})
	}
}

func TestConfig_ValidateBisect_AcceptsWindowScopeAndSkipsADisabledMode(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Bisect.CaptureScope = domain.CaptureScopeWindow

	err := cfg.ValidateBisect()
	if err != nil {
		t.Fatalf("ValidateBisect() rejected the window scope: %v", err)
	}

	cfg.Bisect.Enabled = false
	cfg.Bisect.CaptureScope = unknownScope

	err = cfg.ValidateBisect()
	if err != nil {
		t.Fatalf("ValidateBisect() checked a disabled mode: %v", err)
	}
}

func TestBisectConfig_CaptureScopeForApp_ShadowsTheSection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Bisect.AppConfigs = []config.AppConfig{
		{BundleID: "com.example.Windowed", CaptureScope: domain.CaptureScopeWindow},
		{BundleID: "com.example.Plain"},
	}

	if !cfg.Bisect.HasAppCaptureScopeOverrides() {
		t.Fatal("an app config with a capture scope was not reported")
	}

	if got := cfg.Bisect.CaptureScopeForApp(
		"com.example.windowed",
	); got != domain.CaptureScopeWindow {
		t.Fatalf("scope for the windowed app = %q, want window", got)
	}

	if got := cfg.Bisect.CaptureScopeForApp("com.example.Plain"); got != domain.CaptureScopeScreen {
		t.Fatalf("scope for an app without one = %q, want the section's screen", got)
	}

	if got := cfg.Bisect.CaptureScopeForApp(
		"com.example.Unknown",
	); got != domain.CaptureScopeScreen {
		t.Fatalf("scope for an unlisted app = %q, want the section's screen", got)
	}
}

func TestDefaultConfig_BisectBindingsAreValidActions(t *testing.T) {
	cfg := config.DefaultConfig()

	err := cfg.ValidateHotkeys()
	if err != nil {
		t.Fatalf("the default bisect hotkey table does not validate: %v", err)
	}

	if got := cfg.Bisect.Hotkeys["y"]; len(got) != 1 || got[0] != config.CmdBisectUpLeft {
		t.Fatalf("default y binding = %v, want the top-left quadrant", got)
	}
}
