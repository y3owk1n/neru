package heldrepeat_test

import (
	"context"
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

func startRun(t *testing.T, cfg config.HeldRepeatConfig) (*collector, func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	col := &collector{}
	done := make(chan struct{})

	go func() {
		defer close(done)

		heldrepeat.Run(ctx, cfg, []string{testBinding}, col.dispatch)
	}()

	stop := func() {
		t.Helper()
		cancel()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Run did not return after cancellation")
		}
	}

	return col, stop
}

func runUntilIdle(t *testing.T, cfg config.HeldRepeatConfig, hold time.Duration) [][]string {
	t.Helper()

	col, stop := startRun(t, cfg)

	time.Sleep(hold)
	stop()

	return col.snapshot()
}

// runUntilTicks runs the repeat loop until it has dispatched want ticks. It
// waits for a count rather than a fixed hold. How many ticks fit in a hold
// depends on the timer resolution, which is about 15.6 ms on Windows.
func runUntilTicks(t *testing.T, cfg config.HeldRepeatConfig, want int) [][]string {
	t.Helper()

	col, stop := startRun(t, cfg)
	deadline := time.Now().Add(2 * time.Second)

	for len(col.snapshot()) < want {
		if time.Now().After(deadline) {
			stop()
			t.Fatalf("Run dispatched %d ticks in 2s, want %d", len(col.snapshot()), want)
		}

		time.Sleep(time.Millisecond)
	}

	stop()

	return col.snapshot()
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

func TestRunRepeatsTheBindingUnchanged(t *testing.T) {
	cfg := config.HeldRepeatConfig{
		Enabled:      true,
		InitialDelay: 1,
		Interval:     5,
	}

	ticks := runUntilTicks(t, cfg, 3)

	for idx, tick := range ticks {
		if tick[0] != testBinding {
			t.Fatalf("tick %d was rewritten to %q, want the binding untouched", idx, tick[0])
		}
	}
}
