package eventtap_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/eventtap"
	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
)

// blockedCallTimeout bounds a call that must not block. A lock inversion parks
// its caller for good, so giving up on the call is the only way to observe one.
const blockedCallTimeout = 2 * time.Second

// fakeTap stands in for a platform tap. Every method records that it was
// reached, so a test can say what the adapter did and did not forward.
//
// onDestroy is what makes it useful here: both real backends spend their
// Destroy waiting for the key dispatcher to drain — dispatchWg.Wait() on Linux,
// stopDispatcher on macOS — and that wait runs whatever the dispatcher is in
// the middle of delivering. onDestroy is where a test puts that.
type fakeTap struct {
	mu     sync.Mutex
	counts map[string]int

	onDestroy func()
}

func newFakeTap() *fakeTap {
	return &fakeTap{counts: map[string]int{}}
}

func (f *fakeTap) Enable()  { f.record("Enable") }
func (f *fakeTap) Disable() { f.record("Disable") }

func (f *fakeTap) Destroy() {
	f.record("Destroy")

	if f.onDestroy != nil {
		f.onDestroy()
	}
}

func (f *fakeTap) SetHotkeys(_ []string)                     { f.record("SetHotkeys") }
func (f *fakeTap) SetModifierPassthrough(_ bool, _ []string) { f.record("SetModifierPassthrough") }

func (f *fakeTap) SetInterceptedModifierKeys(
	_ []string,
) {
	f.record("SetInterceptedModifierKeys")
}
func (f *fakeTap) SetStickyModifierToggle(_ bool) { f.record("SetStickyModifierToggle") }
func (f *fakeTap) SetPassthroughCallback(_ tap.PassthroughCallback) {
	f.record("SetPassthroughCallback")
}

func (f *fakeTap) SetKeyboardLayout(_ string) bool { f.record("SetKeyboardLayout"); return true }
func (f *fakeTap) PostModifierEvent(_ string, _ bool) {
	f.record("PostModifierEvent")
}

func (f *fakeTap) record(method string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.counts[method]++
}

// count reports how many times a method was reached.
func (f *fakeTap) count(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.counts[method]
}

// total reports how many tap methods were reached in all.
func (f *fakeTap) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	sum := 0
	for _, calls := range f.counts {
		sum += calls
	}

	return sum
}

var _ tap.Tap = (*fakeTap)(nil)

// TestAdapter_Destroy_DoesNotHoldTheLockWhileTheTapDrainsItsDispatcher is the
// ordering pin for the lock inversion `internal/app/modes/AGENTS.md` states the
// rule against: nothing holding the adapter's mu may wait on anything that
// takes the mode handler's mu.
//
// Tearing a tap down waits for its key dispatcher, and that dispatcher delivers
// keys into modes.Handler.HandleKeyPress, which takes h.mu and pushes the
// passthrough lists straight back out through this adapter. Hold the adapter's
// lock across the wait and a shutdown racing a focus change deadlocks with
// neither side able to give way — so the push modeled here has to complete
// while Destroy is still inside the tap.
func TestAdapter_Destroy_DoesNotHoldTheLockWhileTheTapDrainsItsDispatcher(t *testing.T) {
	t.Parallel()

	fake := newFakeTap()
	adapter := eventtap.NewAdapter(fake, nil)

	fake.onDestroy = func() {
		pushed := make(chan struct{})

		go func() {
			defer close(pushed)

			adapter.SetModifierPassthrough(true, nil)
		}()

		select {
		case <-pushed:
		case <-time.After(blockedCallTimeout):
			t.Error(
				"a passthrough push blocked while the tap was being torn down: " +
					"Destroy is holding the adapter lock across the dispatcher wait",
			)
		}
	}

	adapter.Destroy()
}

// TestAdapter_Destroy_IsSafeToCallTwice covers the two paths that reach it —
// the ordinary shutdown (app/lifecycle.go) and the startup unwind
// (app/startup_phases.go) — plus the case of both running.
//
// The adapter's own lock used to serialize the whole teardown; now that the tap
// is torn down outside it, the guard that keeps a second teardown from starting
// has to be the adapter's own.
func TestAdapter_Destroy_IsSafeToCallTwice(t *testing.T) {
	t.Parallel()

	fake := newFakeTap()
	adapter := eventtap.NewAdapter(fake, nil)

	adapter.Destroy()
	adapter.Destroy()

	if got := fake.count("Destroy"); got != 1 {
		t.Fatalf("tap torn down %d times, want 1", got)
	}
}

// TestAdapter_Destroy_MakesASecondCallerWaitForTheTeardown keeps the method's
// postcondition now that the lock no longer supplies it.
//
// Holding mu across the teardown used to park a second caller until the tap was
// down; with the teardown outside the lock, the destroyed flag alone would let
// that caller return while the dispatcher was still draining. The app closes
// the rest of its infrastructure on the strength of Destroy having returned, so
// the second caller waits — on the teardown, not on the lock.
func TestAdapter_Destroy_MakesASecondCallerWaitForTheTeardown(t *testing.T) {
	t.Parallel()

	fake := newFakeTap()
	adapter := eventtap.NewAdapter(fake, nil)

	teardownEntered := make(chan struct{})
	releaseTeardown := make(chan struct{})

	fake.onDestroy = func() {
		close(teardownEntered)
		<-releaseTeardown
	}

	firstReturned := make(chan struct{})

	go func() {
		defer close(firstReturned)

		adapter.Destroy()
	}()

	<-teardownEntered

	secondReturned := make(chan struct{})

	go func() {
		defer close(secondReturned)

		adapter.Destroy()
	}()

	select {
	case <-secondReturned:
		t.Fatal("a second Destroy returned while the tap was still being torn down")
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseTeardown)

	select {
	case <-secondReturned:
	case <-time.After(blockedCallTimeout):
		t.Fatal("the second Destroy never returned after the teardown finished")
	}

	<-firstReturned

	if got := fake.count("Destroy"); got != 1 {
		t.Fatalf("tap torn down %d times, want 1", got)
	}
}

// TestAdapter_Destroy_StopsLaterCallsFromReachingTheTap is the other half of
// letting go of the lock: with the teardown outside it, a caller racing a
// shutdown can no longer be made to queue behind it, so it must find an adapter
// that has already stopped answering for the tap.
//
// A late call reaching a tap mid-teardown is a use-after-free on macOS, where
// the tap methods run against a handle Destroy is about to release. It covers
// every method that takes the adapter's lock to drive the tap, which is every
// method that reaches the handle: SetKeyboardLayout and PostModifierEvent take
// no lock and are deliberately left out, because no backend routes either
// through it — their doc comments say so.
func TestAdapter_Destroy_StopsLaterCallsFromReachingTheTap(t *testing.T) {
	t.Parallel()

	fake := newFakeTap()
	adapter := eventtap.NewAdapter(fake, nil)

	adapter.Destroy()

	err := adapter.Enable(context.Background())
	if err != nil {
		t.Fatalf("Enable after Destroy reported %v, want no error", err)
	}

	err = adapter.Disable(context.Background())
	if err != nil {
		t.Fatalf("Disable after Destroy reported %v, want no error", err)
	}

	adapter.SetHotkeys([]string{"a"})
	adapter.SetModifierPassthrough(true, []string{"cmd+q"})
	adapter.SetInterceptedModifierKeys([]string{"cmd+q"})
	adapter.SetStickyModifierToggle(true)
	adapter.SetPassthroughCallback(func() {})

	if adapter.IsEnabled() {
		t.Fatal("adapter reports enabled after Destroy")
	}

	if got := fake.total(); got != 1 {
		t.Fatalf("tap reached %d times, want 1 (the teardown itself)", got)
	}
}
