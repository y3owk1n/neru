package heldrepeat_test

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/app/heldrepeat"
	"github.com/y3owk1n/neru/internal/config"
)

const testBinding = "action move_mouse_relative --dx=10 --dy=0"

type collector struct {
	mu    sync.Mutex
	ticks [][]string
}

func (c *collector) dispatch(actions []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ticks = append(c.ticks, append([]string(nil), actions...))
}

func (c *collector) snapshot() [][]string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([][]string(nil), c.ticks...)
}

func dxOf(t *testing.T, actionStr string) int {
	t.Helper()

	for token := range strings.FieldsSeq(actionStr) {
		flag, value, found := strings.Cut(token, "=")
		if !found || flag != "--dx" {
			continue
		}

		parsed, err := strconv.Atoi(value)
		if err != nil {
			t.Fatalf("unparsable --dx in %q: %v", actionStr, err)
		}

		return parsed
	}

	t.Fatalf("no --dx flag in %q", actionStr)

	return 0
}

func runUntilIdle(t *testing.T, cfg config.HeldRepeatConfig, hold time.Duration) [][]string {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var col collector

	done := make(chan struct{})

	go func() {
		defer close(done)

		heldrepeat.Run(ctx, cfg, []string{testBinding}, col.dispatch)
	}()

	time.Sleep(hold)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancellation")
	}

	return col.snapshot()
}

func TestRunAcceleratesWhileHeld(t *testing.T) {
	cfg := config.HeldRepeatConfig{
		Enabled:            true,
		InitialDelay:       1,
		Interval:           5,
		AccelEnabled:       true,
		AccelRampMs:        50,
		AccelMaxMultiplier: 4,
		AccelTargets:       []string{"move_mouse_relative"},
	}

	ticks := runUntilIdle(t, cfg, 250*time.Millisecond)

	if len(ticks) < 3 {
		t.Fatalf("expected several ticks, got %d", len(ticks))
	}

	previous := 0

	for idx, tick := range ticks {
		got := dxOf(t, tick[0])

		if got < previous {
			t.Errorf("tick %d moved %d px, less than the previous %d: ramp went backwards",
				idx, got, previous)
		}

		if got < 10 || got > 40 {
			t.Errorf("tick %d moved %d px, outside the 1x..4x range of a 10px step", idx, got)
		}

		previous = got
	}

	// The ramp is 50ms and the hold is far longer, so the tail must be clamped.
	if last := dxOf(t, ticks[len(ticks)-1][0]); last != 40 {
		t.Errorf("final tick moved %d px, want the clamped 40 px", last)
	}
}

func TestRunDoesNotAccelerateUntargetedAction(t *testing.T) {
	cfg := config.HeldRepeatConfig{
		Enabled:            true,
		InitialDelay:       1,
		Interval:           5,
		AccelEnabled:       true,
		AccelRampMs:        50,
		AccelMaxMultiplier: 4,
		AccelTargets:       []string{"scroll_down"},
	}

	for idx, tick := range runUntilIdle(t, cfg, 120*time.Millisecond) {
		if tick[0] != testBinding {
			t.Fatalf("tick %d was rewritten to %q, want the binding untouched", idx, tick[0])
		}
	}
}

func TestRunDispatchesNothingBeforeInitialDelay(t *testing.T) {
	cfg := config.HeldRepeatConfig{
		Enabled:      true,
		InitialDelay: 500,
		Interval:     5,
	}

	if ticks := runUntilIdle(t, cfg, 20*time.Millisecond); len(ticks) != 0 {
		t.Errorf("expected no repeats during initial_delay, got %d", len(ticks))
	}
}
