package config_test

import (
	"math"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func TestConfigValidateHeldRepeat_AccelInvalid(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*config.Config)
	}{
		{
			name: "negative ramp",
			mutate: func(c *config.Config) {
				c.HeldRepeat.AccelRampMs = -1
			},
		},
		{
			name: "multiplier below one",
			mutate: func(c *config.Config) {
				c.HeldRepeat.AccelMaxMultiplier = 0.5
			},
		},
		{
			name: "multiplier above the upper bound",
			mutate: func(c *config.Config) {
				c.HeldRepeat.AccelMaxMultiplier = config.MaxHeldRepeatAccelMultiplier + 1
			},
		},
		{
			name: "multiplier is NaN",
			mutate: func(c *config.Config) {
				c.HeldRepeat.AccelMaxMultiplier = math.NaN()
			},
		},
		{
			name: "multiplier is Inf",
			mutate: func(c *config.Config) {
				c.HeldRepeat.AccelMaxMultiplier = math.Inf(1)
			},
		},
		{
			name: "no targets while acceleration is enabled",
			mutate: func(c *config.Config) {
				c.HeldRepeat.AccelEnabled = true
				c.HeldRepeat.AccelTargets = nil
			},
		},
		{
			name: "target action does not repeat while held",
			mutate: func(c *config.Config) {
				c.HeldRepeat.AccelTargets = []string{"left_click"}
			},
		},
		{
			name: "target repeats but takes no delta flags",
			mutate: func(c *config.Config) {
				c.HeldRepeat.AccelTargets = []string{"scroll_down"}
			},
		},
		{
			name: "target action is unknown",
			mutate: func(c *config.Config) {
				c.HeldRepeat.AccelTargets = []string{"move_mouse_relatve"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			tt.mutate(cfg)

			err := cfg.ValidateHeldRepeat(nil)
			if err == nil {
				t.Error("ValidateHeldRepeat() expected error, got nil")
			}
		})
	}
}

// TestConfigValidateHeldRepeat_AccelWithRepeatIsSilent is the other half of
// TestLoadWithValidation_ReportsAnInertAccelSetting. The warning names a
// combination, so it must not fire on the one that works.
func TestConfigValidateHeldRepeat_AccelWithRepeatIsSilent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.HeldRepeat.Enabled = true
	cfg.HeldRepeat.AccelEnabled = true

	warnings := &config.Warnings{}

	err := cfg.ValidateWithWarnings(warnings, config.WrittenConfig{})
	if err != nil {
		t.Fatalf("ValidateWithWarnings() refused the working combination: %v", err)
	}

	if got := warnings.Messages(); len(got) > 0 {
		t.Errorf("warnings = %q, want none", got)
	}
}

func TestHeldRepeatAccelAppliesTo(t *testing.T) {
	enabled := config.HeldRepeatConfig{
		AccelEnabled: true,
		AccelTargets: []string{testMoveMouseRelative},
	}

	if !enabled.AccelAppliesTo(testMoveMouseRelative) {
		t.Error("expected move_mouse_relative to be accelerated")
	}

	if enabled.AccelAppliesTo("scroll_up") {
		t.Error("expected action outside the allowlist to be left alone")
	}

	if enabled.AccelAppliesTo("") {
		t.Error("expected empty action name to be left alone")
	}

	disabled := enabled
	disabled.AccelEnabled = false

	if disabled.AccelAppliesTo(testMoveMouseRelative) {
		t.Error("expected no acceleration while accel_enabled is false")
	}
}
