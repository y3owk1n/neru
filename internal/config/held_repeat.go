package config

import (
	"slices"
	"time"

	"github.com/y3owk1n/neru/internal/domain/action"
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

// AccelMultiplierAt ramps linearly from 1x to AccelMaxMultiplier over AccelRampMs.
// Only the distance grows, never the interval, so the timer is never rescheduled.
func (c HeldRepeatConfig) AccelMultiplierAt(held time.Duration) float64 {
	if !c.AccelEnabled || c.AccelMaxMultiplier <= 1 {
		return 1
	}

	if c.AccelRampMs <= 0 {
		return c.AccelMaxMultiplier
	}

	progress := float64(held.Milliseconds()) / float64(c.AccelRampMs)
	if progress <= 0 {
		return 1
	}

	if progress >= 1 {
		return c.AccelMaxMultiplier
	}

	return 1 + progress*(c.AccelMaxMultiplier-1)
}

// AccelAppliesTo reports whether the named action is in the acceleration allowlist.
func (c HeldRepeatConfig) AccelAppliesTo(name string) bool {
	return c.AccelEnabled && slices.Contains(c.AccelTargets, name)
}
