// Package heldrepeat drives the repeat-while-held loop shared by both hotkey paths.
package heldrepeat

import (
	"context"
	"time"

	"github.com/y3owk1n/neru/internal/config"
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
	initialTimer := time.NewTimer(time.Duration(cfg.InitialDelay) * time.Millisecond)
	defer initialTimer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-initialTimer.C:
	}

	ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dispatch(actions)
		}
	}
}
