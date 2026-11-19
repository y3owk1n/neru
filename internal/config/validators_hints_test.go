package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
)

const (
	dirNormal     = "normal"
	dirReverse    = "reverse"
	bundleExample = "com.example"
	bundleOther   = "com.example.other"
)

func TestValidateHints_LabelDirection(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		wantErr   bool
	}{
		{name: "reverse is valid", direction: dirReverse, wantErr: false},
		{name: "normal is valid", direction: dirNormal, wantErr: false},
		{name: "empty defaults to normal (no error)", direction: "", wantErr: false},
		{name: "unknown value is rejected", direction: "sideways", wantErr: true},
		{name: "uppercase is rejected (case sensitive)", direction: "NORMAL", wantErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Hints.LabelDirection = testCase.direction

			err := cfg.ValidateHints(nil)
			if testCase.wantErr && err == nil {
				t.Fatalf(
					"ValidateHints() expected error for label_direction=%q, got nil",
					testCase.direction,
				)
			}

			if !testCase.wantErr && err != nil {
				t.Fatalf(
					"ValidateHints() unexpected error for label_direction=%q: %v",
					testCase.direction,
					err,
				)
			}
		})
	}
}

func TestValidateHints_LabelAlternation(t *testing.T) {
	tests := []struct {
		alternation string
		wantErr     bool
	}{
		{alternation: domain.LabelAlternationNone, wantErr: false},
		{alternation: domain.LabelAlternationHalves, wantErr: false},
		{alternation: "", wantErr: false},
		{alternation: "thirds", wantErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.alternation, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Hints.LabelAlternation = testCase.alternation

			err := cfg.ValidateHints(nil)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("ValidateHints() with label_alternation=%q: err = %v, wantErr %v",
					testCase.alternation, err, testCase.wantErr)
			}
		})
	}
}

func TestValidateAppConfigs_LabelDirection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.AppConfigs = []config.AppConfig{
		{
			BundleID:       bundleExample,
			LabelDirection: dirNormal,
		},
	}

	err := cfg.ValidateAppConfigs()
	if err != nil {
		t.Fatalf("ValidateAppConfigs() unexpected error for valid app config: %v", err)
	}

	cfg = config.DefaultConfig()
	cfg.Hints.AppConfigs = []config.AppConfig{
		{
			BundleID:       bundleExample,
			LabelDirection: "diagonal",
		},
	}

	err = cfg.ValidateAppConfigs()
	if err == nil {
		t.Fatal("ValidateAppConfigs() expected error for invalid app label_direction")
	}
}

func TestLabelDirectionForApp(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.LabelDirection = dirReverse
	cfg.Hints.AppConfigs = []config.AppConfig{
		{
			BundleID:       bundleExample,
			LabelDirection: dirNormal,
		},
	}

	// App override takes precedence.
	if got := cfg.Hints.LabelDirectionForApp(bundleExample); got != dirNormal {
		t.Errorf("LabelDirectionForApp(app with override) = %q, want %q", got, dirNormal)
	}

	// Fallback to global config.
	if got := cfg.Hints.LabelDirectionForApp("com.other"); got != dirReverse {
		t.Errorf("LabelDirectionForApp(app without override) = %q, want %q", got, dirReverse)
	}

	// Empty global value normalizes to the default (normal).
	cfg = config.DefaultConfig()
	cfg.Hints.LabelDirection = ""

	if got := cfg.Hints.LabelDirectionForApp(bundleExample); got != dirNormal {
		t.Errorf("LabelDirectionForApp(empty global) = %q, want %q", got, dirNormal)
	}
}
