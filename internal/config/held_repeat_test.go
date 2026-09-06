package config_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/motion"
)

const (
	testAccelMoveBinding  = "action move_mouse_relative --dx=20 --dy=0"
	testAccelScrollDown   = "action scroll_down"
	testAccelScrollUp     = "action scroll_up"
	testMotionRight       = "action move_mouse_relative --dx=10 --dy=0"
	testMoveMouseRelative = "move_mouse_relative"
)

func TestHeldRepeatActionName(t *testing.T) {
	tests := []struct {
		name    string
		actions []string
		want    string
	}{
		{
			name:    "single action binding",
			actions: []string{testAccelMoveBinding},
			want:    "move_mouse_relative",
		},
		{
			name:    "action without arguments",
			actions: []string{testAccelScrollDown},
			want:    "scroll_down",
		},
		{
			name:    "leading whitespace is tolerated",
			actions: []string{"  " + testAccelScrollUp + "  "},
			want:    "scroll_up",
		},
		{
			name:    "quoted name is unquoted",
			actions: []string{`action "scroll_up"`},
			want:    "scroll_up",
		},
		{
			name:    "multi-action binding has no single name",
			actions: []string{testAccelScrollDown, testAccelScrollUp},
			want:    "",
		},
		{
			name:    "mode switch is not an action binding",
			actions: []string{"hints --action left_click"},
			want:    "",
		},
		{
			name:    "exec binding is not an action binding",
			actions: []string{"exec echo test"},
			want:    "",
		},
		{
			name:    "empty list",
			actions: nil,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.HeldRepeatActionName(tt.actions)
			if got != tt.want {
				t.Errorf("HeldRepeatActionName(%v) = %q, want %q", tt.actions, got, tt.want)
			}
		})
	}
}

func TestHeldRepeatConfig_HeldMotion(t *testing.T) {
	accelOn := config.HeldRepeatConfig{
		Enabled:      true,
		AccelEnabled: true,
		AccelTargets: []string{testMoveMouseRelative},
	}

	tests := []struct {
		name    string
		cfg     config.HeldRepeatConfig
		actions []string
		want    motion.Direction
		step    int
		ok      bool
	}{
		{"right", accelOn, []string{testMotionRight}, motion.Direction{X: 1}, 10, true},
		{
			"up-left reduces to signs, largest axis is the step",
			accelOn,
			[]string{"action move_mouse_relative --dx -30 --dy -5"},
			motion.Direction{X: -1, Y: -1},
			30,
			true,
		},
		{
			"zero delta is not a direction",
			accelOn,
			[]string{"action move_mouse_relative --dx=0 --dy=0"},
			motion.Direction{},
			0,
			false,
		},
		{
			"scroll never qualifies",
			accelOn,
			[]string{testAccelScrollDown},
			motion.Direction{},
			0,
			false,
		},
		{
			"sequence never qualifies",
			accelOn,
			[]string{testMotionRight, "action left_click"},
			motion.Direction{},
			0,
			false,
		},
		{
			"accel off still glides",
			config.HeldRepeatConfig{Enabled: true},
			[]string{testMotionRight},
			motion.Direction{X: 1},
			10,
			true,
		},
		{
			"held repeat off",
			config.HeldRepeatConfig{
				AccelEnabled: true,
				AccelTargets: []string{testMoveMouseRelative},
			},
			[]string{testMotionRight},
			motion.Direction{},
			0,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, step, ok := tt.cfg.HeldMotion(tt.actions)
			if got != tt.want || step != tt.step || ok != tt.ok {
				t.Errorf("HeldMotion = (%+v, %d, %v), want (%+v, %d, %v)",
					got, step, ok, tt.want, tt.step, tt.ok)
			}
		})
	}
}

func TestHeldRepeatConfig_Ramp(t *testing.T) {
	base := config.HeldRepeatConfig{
		Enabled:            true,
		Interval:           50,
		AccelRampMs:        500,
		AccelMaxMultiplier: 4,
		AccelTargets:       []string{testMoveMouseRelative},
	}

	constant := base.Ramp()
	if constant.Multiplier != 1 || constant.Duration != 0 {
		t.Errorf("accel off: Ramp() = %+v, want a multiplier of 1 and no duration", constant)
	}

	if constant.Interval != 50*time.Millisecond {
		t.Errorf("Ramp().Interval = %v, want 50ms", constant.Interval)
	}

	accelerated := base
	accelerated.AccelEnabled = true

	ramp := accelerated.Ramp()
	if ramp.Multiplier != 4 || ramp.Duration != 500*time.Millisecond {
		t.Errorf("accel on: Ramp() = %+v, want 4x over 500ms", ramp)
	}
}
