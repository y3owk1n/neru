package config_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
)

// unknownScope is a capture scope no vocabulary declares.
const unknownScope = "desk"

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

func TestConfig_ValidateBisect_ErrorNamesTheUIKey(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(cfg *config.Config)
		want   string
	}{
		{
			"line width",
			func(cfg *config.Config) { cfg.Bisect.UI.LineWidth = -1 },
			"bisect.ui.line_width",
		},
		{
			"label char",
			func(cfg *config.Config) { cfg.Bisect.UI.LabelChar = "ab" },
			"bisect.ui.label_char",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			testCase.mutate(cfg)

			err := cfg.ValidateBisect()
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error = %v, want it to name %s", err, testCase.want)
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
