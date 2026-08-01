package scroll_test

import (
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/app/components/scroll"
)

// TestContext_ZeroValueIsInactive pins that a Context is usable without a
// constructor — the mode handler embeds one directly.
func TestContext_ZeroValueIsInactive(t *testing.T) {
	t.Parallel()

	var ctx scroll.Context

	if ctx.IsActive() {
		t.Error("zero-value Context IsActive() = true, want false")
	}
}

func TestContext_SetIsActiveRoundTrips(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		set  bool
		want bool
	}{
		{name: "activate", set: true, want: true},
		{name: "deactivate", set: false, want: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var ctx scroll.Context

			ctx.SetIsActive(testCase.set)

			if got := ctx.IsActive(); got != testCase.want {
				t.Errorf("IsActive() = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestContext_ResetClearsActive covers the mode-exit path, which resets rather
// than setting false explicitly.
func TestContext_ResetClearsActive(t *testing.T) {
	t.Parallel()

	var ctx scroll.Context

	ctx.SetIsActive(true)
	ctx.Reset()

	if ctx.IsActive() {
		t.Error("IsActive() after Reset() = true, want false")
	}
}

// TestContext_ConcurrentAccess exercises the mutex. Scroll state is written by
// the event-tap thread and read by the indicator polling goroutine, so the race
// detector needs a test that actually contends.
func TestContext_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	var (
		ctx       scroll.Context
		waitGroup sync.WaitGroup
	)

	const iterations = 200

	waitGroup.Add(3)

	go func() {
		defer waitGroup.Done()

		for index := range iterations {
			ctx.SetIsActive(index%2 == 0)
		}
	}()

	go func() {
		defer waitGroup.Done()

		for range iterations {
			_ = ctx.IsActive()
		}
	}()

	go func() {
		defer waitGroup.Done()

		for range iterations {
			ctx.Reset()
		}
	}()

	waitGroup.Wait()

	// The race detector is the primary check. Also assert the context is still
	// coherent and mutable, so the test can fail without -race too.
	settled := ctx.IsActive()
	if ctx.IsActive() != settled {
		t.Fatal("IsActive() disagreed with itself with no writer running")
	}

	ctx.SetIsActive(true)

	if !ctx.IsActive() {
		t.Error("SetIsActive(true) after concurrent access did not take effect")
	}

	ctx.Reset()

	if ctx.IsActive() {
		t.Error("Reset() after concurrent access did not clear the flag")
	}
}
