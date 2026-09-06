package config

import (
	"slices"
	"time"

	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/motion"
)

// HeldRepeatActionName returns the name of a lone "action <name> …" binding, or "".
func HeldRepeatActionName(actions []string) string {
	if len(actions) != 1 {
		return ""
	}

	parts := SplitStepArgs(actions[0])
	if len(parts) < 2 || parts[0] != action.PrefixAction {
		return ""
	}

	return parts[1]
}

// AccelAppliesTo reports whether the named action is in the acceleration allowlist.
func (c HeldRepeatConfig) AccelAppliesTo(name string) bool {
	return c.AccelEnabled && slices.Contains(c.AccelTargets, name)
}

// HeldMotion reports whether a held binding glides, with the direction it
// points in and its step size. Only a lone move_mouse_relative with parsable,
// non-zero --dx/--dy qualifies, and only while held_repeat.enabled is set.
func (c HeldRepeatConfig) HeldMotion(actions []string) (motion.Direction, int, bool) {
	if !c.Enabled || action.Name(HeldRepeatActionName(actions)) != action.NameMoveMouseRelative {
		return motion.Direction{}, 0, false
	}

	deltaX, deltaY, ok := action.ParseDeltaFlags(actions[0])
	if !ok {
		return motion.Direction{}, 0, false
	}

	dir := motion.FromDelta(deltaX, deltaY)
	if dir.IsZero() {
		return motion.Direction{}, 0, false
	}

	return dir, max(absInt(deltaX), absInt(deltaY)), true
}

// Ramp is the glide's speed in the units the integrator takes: a binding's
// step is worth one interval of travel, and the accel options say whether and
// how far that speed climbs. With accel_enabled off the multiplier is 1, a
// constant-speed glide.
func (c HeldRepeatConfig) Ramp() motion.Ramp {
	ramp := motion.Ramp{
		Interval:   time.Duration(c.Interval) * time.Millisecond,
		Multiplier: 1,
	}

	if c.AccelAppliesTo(string(action.NameMoveMouseRelative)) {
		ramp.Multiplier = c.AccelMaxMultiplier
		ramp.Duration = time.Duration(c.AccelRampMs) * time.Millisecond
	}

	return ramp
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}

	return v
}
