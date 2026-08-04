package config_test

import (
	"math"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/config"
)

func TestConfigValidateHeldRepeat_AccelDefaults(t *testing.T) {
	cfg := config.DefaultConfig()

	err := cfg.ValidateHeldRepeat()
	if err != nil {
		t.Fatalf("ValidateHeldRepeat() unexpected error: %v", err)
	}
}

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

			err := cfg.ValidateHeldRepeat()
			if err == nil {
				t.Error("ValidateHeldRepeat() expected error, got nil")
			}
		})
	}
}

func TestHeldRepeatAccelMultiplierAt(t *testing.T) {
	accelerating := config.HeldRepeatConfig{
		AccelEnabled:       true,
		AccelRampMs:        500,
		AccelMaxMultiplier: 4,
	}

	tests := []struct {
		name string
		cfg  config.HeldRepeatConfig
		held time.Duration
		want float64
	}{
		{"press starts at 1x", accelerating, 0, 1},
		{"quarter ramp", accelerating, 125 * time.Millisecond, 1.75},
		{"half ramp", accelerating, 250 * time.Millisecond, 2.5},
		{"full ramp", accelerating, 500 * time.Millisecond, 4},
		{"past ramp stays clamped", accelerating, 5 * time.Second, 4},
		{
			name: "disabled never scales",
			cfg: config.HeldRepeatConfig{
				AccelEnabled:       false,
				AccelRampMs:        500,
				AccelMaxMultiplier: 4,
			},
			held: time.Second,
			want: 1,
		},
		{
			name: "max multiplier of one never scales",
			cfg: config.HeldRepeatConfig{
				AccelEnabled:       true,
				AccelRampMs:        500,
				AccelMaxMultiplier: 1,
			},
			held: time.Second,
			want: 1,
		},
		{
			name: "zero ramp jumps straight to max",
			cfg: config.HeldRepeatConfig{
				AccelEnabled:       true,
				AccelRampMs:        0,
				AccelMaxMultiplier: 4,
			},
			held: time.Millisecond,
			want: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.AccelMultiplierAt(tt.held)
			if got != tt.want {
				t.Errorf("AccelMultiplierAt(%v) = %v, want %v", tt.held, got, tt.want)
			}
		})
	}
}

func TestHeldRepeatAccelAppliesTo(t *testing.T) {
	enabled := config.HeldRepeatConfig{
		AccelEnabled: true,
		AccelTargets: []string{"move_mouse_relative"},
	}

	if !enabled.AccelAppliesTo("move_mouse_relative") {
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

	if disabled.AccelAppliesTo("move_mouse_relative") {
		t.Error("expected no acceleration while accel_enabled is false")
	}
}
