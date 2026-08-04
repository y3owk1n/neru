// Package heldrepeat drives the repeat-while-held loop shared by both hotkey paths.
package heldrepeat

import (
	"context"
	"time"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// Run blocks, repeating actions until ctx is canceled. The dispatch on key-down
// belongs to the caller: the two call sites disagree about whether the press has
// already fired.
func Run(
	ctx context.Context,
	cfg config.HeldRepeatConfig,
	actions []string,
	dispatch func([]string),
) {
	accelerate := cfg.AccelAppliesTo(config.HeldRepeatActionName(actions))

	initialTimer := time.NewTimer(time.Duration(cfg.InitialDelay) * time.Millisecond)
	defer initialTimer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-initialTimer.C:
	}

	// Timed from the first repeat, not the press: nothing moves during
	// initial_delay, so an earlier start would land tick one mid-ramp.
	repeatingSince := time.Now()

	ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tickActions := actions
			if accelerate {
				tickActions = action.ScaleAllDeltaFlags(
					actions,
					cfg.AccelMultiplierAt(time.Since(repeatingSince)),
				)
			}

			dispatch(tickActions)
		}
	}
}
