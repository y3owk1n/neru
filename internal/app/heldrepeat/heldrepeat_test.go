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

	ticks := runUntilIdle(t, cfg, 60*time.Millisecond)
	if len(ticks) < 3 {
		t.Fatalf("expected several ticks, got %d", len(ticks))
	}

	for idx, tick := range ticks {
		if tick[0] != testBinding {
			t.Fatalf("tick %d was rewritten to %q, want the binding untouched", idx, tick[0])
		}
	}
}
