package state_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// AppState exposes three independent publish/subscribe flags. The enabled flag
// is covered in app_state_test.go; the screen-share and scroll-invert flags had
// no subscription tests at all, so their "only notify when the value actually
// changed" guards and their nil-callback guards were unverified.
//
// Both guards fail quietly. A broken change-guard floods every subscriber on
// each no-op write (the systray redraws, the overlay reconfigures) without
// changing any observable state; a broken nil-guard stores a nil callback that
// panics later, on a different goroutine, far from the subscribe call.

// settleWindow is how long a test waits before concluding that no further
// notification is coming. Callbacks are dispatched with `go callback(...)`, so
// asserting "no callback" immediately after a write would pass even when a
// spurious one is already scheduled.
const settleWindow = 250 * time.Millisecond

// notifyRecorder collects the values a subscription delivered.
type notifyRecorder struct {
	mu       sync.Mutex
	received []bool
	signal   chan struct{}
}

func newNotifyRecorder() *notifyRecorder {
	return &notifyRecorder{signal: make(chan struct{}, 64)}
}

func (r *notifyRecorder) callback(value bool) {
	r.mu.Lock()
	r.received = append(r.received, value)
	r.mu.Unlock()

	select {
	case r.signal <- struct{}{}:
	default:
	}
}

func (r *notifyRecorder) values() []bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]bool(nil), r.received...)
}

// awaitCount blocks until at least want notifications have arrived, failing if
// that does not happen within callbackTimeout.
func (r *notifyRecorder) awaitCount(t *testing.T, want int, what string) {
	t.Helper()

	deadline := time.After(callbackTimeout)

	for len(r.values()) < want {
		select {
		case <-r.signal:
		case <-deadline:
			t.Fatalf("timed out waiting for %s: got %d notifications, want %d",
				what, len(r.values()), want)
		}
	}
}

// expectNoFurtherNotification asserts that the recorder stays at want after a
// settle window, so a spurious notification already in flight is still caught.
func (r *notifyRecorder) expectNoFurtherNotification(t *testing.T, want int, what string) {
	t.Helper()

	time.Sleep(settleWindow)

	if got := r.values(); len(got) != want {
		t.Errorf("%s: got %d notifications (%v), want %d", what, len(got), got, want)
	}
}

// boolSubscription describes one of AppState's publish/subscribe flags so the
// same contract can be asserted against each.
type boolSubscription struct {
	name        string
	get         func(*state.AppState) bool
	set         func(*state.AppState, bool)
	toggle      func(*state.AppState) bool
	subscribe   func(*state.AppState, func(bool)) uint64
	unsubscribe func(*state.AppState, uint64)
}

func boolSubscriptions() []boolSubscription {
	return []boolSubscription{
		{
			name:        "screen share hidden",
			get:         (*state.AppState).IsHiddenForScreenShare,
			set:         (*state.AppState).SetHiddenForScreenShare,
			toggle:      (*state.AppState).ToggleHiddenForScreenShare,
			subscribe:   (*state.AppState).OnScreenShareStateChanged,
			unsubscribe: (*state.AppState).OffScreenShareStateChanged,
		},
		{
			name:        "scroll inverted",
			get:         (*state.AppState).IsScrollInverted,
			set:         (*state.AppState).SetScrollInverted,
			toggle:      (*state.AppState).ToggleScrollInverted,
			subscribe:   (*state.AppState).OnScrollInvertStateChanged,
			unsubscribe: (*state.AppState).OffScrollInvertStateChanged,
		},
	}
}

// TestAppState_Subscriptions_RejectNilCallbacks pins the subscribe guard. A nil
// callback must be refused with a zero ID rather than stored, because a stored
// nil is only discovered when the notify loop reaches it.
func TestAppState_Subscriptions_RejectNilCallbacks(t *testing.T) {
	for _, sub := range boolSubscriptions() {
		t.Run(sub.name, func(t *testing.T) {
			appState := state.NewAppState()

			if id := sub.subscribe(appState, nil); id != 0 {
				t.Errorf("subscribing nil returned ID %d, want 0", id)
			}

			// A real callback must still get a usable, non-zero ID — the guard
			// must reject only nil.
			recorder := newNotifyRecorder()

			first := sub.subscribe(appState, recorder.callback)
			if first == 0 {
				t.Fatal("subscribing a real callback returned ID 0")
			}

			second := sub.subscribe(appState, newNotifyRecorder().callback)
			if second == first {
				t.Errorf("second subscription reused ID %d; IDs must be distinct", first)
			}

			// Toggling must not panic on the refused nil subscription.
			sub.toggle(appState)
			recorder.awaitCount(t, 2, "the initial and toggle notifications")
		})
	}
}

// TestAppState_Subscriptions_FireInitialStateOnSubscribe checks a new subscriber
// is told the current value immediately, so it can render without waiting for
// the next change.
func TestAppState_Subscriptions_FireInitialStateOnSubscribe(t *testing.T) {
	for _, sub := range boolSubscriptions() {
		t.Run(sub.name, func(t *testing.T) {
			appState := state.NewAppState()

			// Put the flag in the non-default position first, so the initial
			// notification carries a value that is not merely the zero value.
			sub.set(appState, true)

			if !sub.get(appState) {
				t.Fatal("set(true) did not take effect")
			}

			recorder := newNotifyRecorder()
			sub.subscribe(appState, recorder.callback)

			recorder.awaitCount(t, 1, "the initial notification")

			if got := recorder.values(); got[0] != true {
				t.Errorf("initial notification carried %t, want the current state true", got[0])
			}
		})
	}
}

// TestAppState_Subscriptions_NotifyOnlyOnActualChange is the core guard: writing
// the value the flag already holds must not wake subscribers.
func TestAppState_Subscriptions_NotifyOnlyOnActualChange(t *testing.T) {
	for _, sub := range boolSubscriptions() {
		t.Run(sub.name, func(t *testing.T) {
			appState := state.NewAppState()

			recorder := newNotifyRecorder()
			sub.subscribe(appState, recorder.callback)
			recorder.awaitCount(t, 1, "the initial notification")

			initial := sub.get(appState)

			// Writing the current value repeatedly must stay silent.
			sub.set(appState, initial)
			sub.set(appState, initial)
			recorder.expectNoFurtherNotification(t, 1, "writing the unchanged value")

			// A genuine change must notify exactly once, with the new value.
			sub.set(appState, !initial)
			recorder.awaitCount(t, 2, "the notification for a real change")

			values := recorder.values()
			if values[1] != !initial {
				t.Errorf("change notification carried %t, want %t", values[1], !initial)
			}

			// And writing that new value again must be silent too.
			sub.set(appState, !initial)
			recorder.expectNoFurtherNotification(t, 2, "rewriting the new value")

			if got := sub.get(appState); got != !initial {
				t.Errorf("state is %t after the change, want %t", got, !initial)
			}
		})
	}
}

// TestAppState_Subscriptions_ToggleAlwaysNotifies covers the toggle path, which
// deliberately skips the change guard because a toggle always changes the value.
func TestAppState_Subscriptions_ToggleAlwaysNotifies(t *testing.T) {
	for _, sub := range boolSubscriptions() {
		t.Run(sub.name, func(t *testing.T) {
			appState := state.NewAppState()

			recorder := newNotifyRecorder()
			sub.subscribe(appState, recorder.callback)
			recorder.awaitCount(t, 1, "the initial notification")

			want := !sub.get(appState)

			// The returned value must be the post-toggle state; callers use it
			// instead of a second racy read.
			if got := sub.toggle(appState); got != want {
				t.Errorf("toggle returned %t, want the new state %t", got, want)
			}

			if got := sub.get(appState); got != want {
				t.Errorf("state is %t after toggle, want %t", got, want)
			}

			recorder.awaitCount(t, 2, "the toggle notification")

			if values := recorder.values(); values[1] != want {
				t.Errorf("toggle notification carried %t, want %t", values[1], want)
			}

			// Toggling back notifies again with the original value.
			if got := sub.toggle(appState); got != !want {
				t.Errorf("second toggle returned %t, want %t", got, !want)
			}

			recorder.awaitCount(t, 3, "the second toggle notification")

			if values := recorder.values(); values[2] != !want {
				t.Errorf("second toggle notification carried %t, want %t", values[2], !want)
			}
		})
	}
}

// TestAppState_Subscriptions_UnsubscribeStopsNotifications checks that a
// canceled subscription really is detached — a leaked one keeps a stale
// component alive and repainting after it has been torn down.
func TestAppState_Subscriptions_UnsubscribeStopsNotifications(t *testing.T) {
	for _, sub := range boolSubscriptions() {
		t.Run(sub.name, func(t *testing.T) {
			appState := state.NewAppState()

			staying := newNotifyRecorder()
			leaving := newNotifyRecorder()

			sub.subscribe(appState, staying.callback)

			leavingID := sub.subscribe(appState, leaving.callback)

			staying.awaitCount(t, 1, "the initial notification for the retained subscriber")
			leaving.awaitCount(t, 1, "the initial notification for the departing subscriber")

			sub.unsubscribe(appState, leavingID)

			sub.toggle(appState)

			// The remaining subscriber must still be notified: unsubscribing
			// one must not detach the others.
			staying.awaitCount(t, 2, "the toggle notification for the retained subscriber")

			leaving.expectNoFurtherNotification(t, 1, "notifications after unsubscribing")

			// Unsubscribing an unknown ID must be a harmless no-op.
			sub.unsubscribe(appState, leavingID)
			sub.unsubscribe(appState, 999999)

			sub.toggle(appState)
			staying.awaitCount(t, 3, "the notification after redundant unsubscribes")
		})
	}
}

// TestAppState_SetMode_InvalidatesStaleExitReasonOnlyForRealModes pins the
// asymmetry in SetMode: entering a real mode clears any leftover exit reason,
// while entering idle preserves the reason that was just recorded — that is how
// the mode handler learns why the previous mode ended.
func TestAppState_SetMode_InvalidatesStaleExitReasonOnlyForRealModes(t *testing.T) {
	nonIdleModes := []domain.Mode{domain.ModeHints, domain.ModeGrid, domain.ModeScroll}

	t.Run("entering idle preserves a freshly recorded reason", func(t *testing.T) {
		appState := state.NewAppState()

		appState.SetMode(domain.ModeHints)
		appState.SetModeExitReason(state.ModeExitReasonCompleted)
		appState.SetMode(domain.ModeIdle)

		if got := appState.ConsumeModeExitReason(); got != state.ModeExitReasonCompleted {
			t.Errorf("ConsumeModeExitReason() = %v, want %v",
				got, state.ModeExitReasonCompleted)
		}
	})

	for _, mode := range nonIdleModes {
		t.Run(fmt.Sprintf("entering mode %v clears a stale reason", mode), func(t *testing.T) {
			appState := state.NewAppState()

			appState.SetModeExitReason(state.ModeExitReasonCompleted)

			// Entering a real mode must drop the reason from the *previous*
			// session, or the next exit would report why the last one ended.
			appState.SetMode(mode)

			if got := appState.ConsumeModeExitReason(); got != state.ModeExitReasonNone {
				t.Errorf("ConsumeModeExitReason() after entering %v = %v, want %v",
					mode, got, state.ModeExitReasonNone)
			}

			if got := appState.CurrentMode(); got != mode {
				t.Errorf("CurrentMode() = %v, want %v", got, mode)
			}
		})
	}
}

// TestModifierState_Reset_NotifiesOnlyWhenModifiersWereHeld covers the matching
// guard on the sticky-modifier indicator. Reset runs on every mode exit, so
// notifying unconditionally would repaint the indicator constantly even though
// nothing was ever held.
func TestModifierState_Reset_NotifiesOnlyWhenModifiersWereHeld(t *testing.T) {
	modifierState := state.NewModifierState()

	recorder := newNotifyRecorder()

	modifierState.OnChange(func(mods action.Modifiers) {
		recorder.callback(mods != 0)
	})

	recorder.awaitCount(t, 1, "the initial OnChange notification")

	// Nothing is held, so Reset has nothing to clear and must stay silent.
	modifierState.Reset()
	modifierState.Reset()
	recorder.expectNoFurtherNotification(t, 1, "resetting with no modifiers held")

	// With a modifier held, Reset must notify that it was cleared.
	modifierState.Toggle(action.ModShift)
	recorder.awaitCount(t, 2, "the toggle notification")

	modifierState.Reset()
	recorder.awaitCount(t, 3, "the reset notification")

	if values := recorder.values(); values[2] {
		t.Error("reset notification reported modifiers still held, want cleared")
	}

	// And a second reset is silent again.
	modifierState.Reset()
	recorder.expectNoFurtherNotification(t, 3, "resetting an already-cleared state")
}
