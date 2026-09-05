//go:build windows

package windows

// The hook either hands a chord to the application or dispatches it to Neru,
// never both. Which of the two happens is decided from the passthrough state
// the mode layer pushes down, in the same way the macOS tap decides it; deleting
// that decision is silent everywhere except on a Windows desktop (ADR 0011).

import (
	"testing"
	"time"
)

// The hook spells a chord the way pressedModifierNames assembles it: lowercase.
const chordCtrlC = "ctrl+c"

// routedKey drives handleKey for one key-down and reports where it went.
// Delivery is asynchronous, so a sentinel is queued behind the key and the
// keys that arrive ahead of it are the ones the chord dispatched: the
// dispatcher delivers in order, which is what makes the sentinel a fence.
func routedKey(t *testing.T, eventTap *EventTap, key string) ([]string, bool) {
	t.Helper()

	const sentinel = "routedKey-sentinel"

	arrived := make(chan string, 8)

	eventTap.SetHandler(func(key string) { arrived <- key })

	consumed := eventTap.handleKey(key, false)

	eventTap.dispatchKey(sentinel)

	var dispatched []string

	for {
		select {
		case delivered := <-arrived:
			if delivered == sentinel {
				return dispatched, consumed
			}

			dispatched = append(dispatched, delivered)
		case <-time.After(time.Second):
			t.Fatal("the dispatcher never delivered the sentinel queued behind the key")
		}
	}
}

func TestEventTap_HandleKey_ConsumesAnUnboundChordWithPassthroughOff(t *testing.T) {
	t.Parallel()

	eventTap := newTestTap(t)
	eventTap.SetModifierPassthrough(false, nil)

	dispatched, consumed := routedKey(t, eventTap, chordCtrlC)

	if !consumed {
		t.Error("Ctrl+C reached the application while a mode had the keyboard")
	}

	if len(dispatched) != 1 || dispatched[0] != "ctrl+c" {
		t.Errorf("dispatched %v, want [ctrl+c]", dispatched)
	}
}

func TestEventTap_HandleKey_PassesAnUnboundChordThrough(t *testing.T) {
	t.Parallel()

	eventTap := newTestTap(t)
	eventTap.SetModifierPassthrough(true, nil)

	fired := make(chan struct{}, 1)
	eventTap.SetPassthroughCallback(func() { fired <- struct{}{} })

	dispatched, consumed := routedKey(t, eventTap, chordCtrlC)

	if consumed {
		t.Error("Ctrl+C was consumed with passthrough on")
	}

	if len(dispatched) != 0 {
		t.Errorf("dispatched %v to Neru as well as passing it through", dispatched)
	}

	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Error("the passthrough callback never fired, so the mode cannot refresh after the chord")
	}
}

func TestEventTap_HandleKey_KeepsAChordTheModeBinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(*EventTap)
	}{
		{
			name: "blacklisted, written in another modifier order",
			setup: func(eventTap *EventTap) {
				eventTap.SetModifierPassthrough(true, []string{"shift+ctrl+c"})
			},
		},
		{
			name: "intercepted by the mode",
			setup: func(eventTap *EventTap) {
				eventTap.SetModifierPassthrough(true, nil)
				eventTap.SetInterceptedModifierKeys([]string{"Ctrl+Shift+C"})
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			eventTap := newTestTap(t)
			testCase.setup(eventTap)

			dispatched, consumed := routedKey(t, eventTap, "ctrl+shift+c")

			if !consumed || len(dispatched) != 1 {
				t.Errorf(
					"consumed=%v dispatched=%v, want the chord consumed and dispatched once",
					consumed,
					dispatched,
				)
			}
		})
	}
}

// Shift-only chords and bare keys are the mode's input, whatever passthrough
// says: a hint label or a grid cell key is typed with them.
func TestEventTap_HandleKey_ConsumesShiftOnlyChordsWithPassthroughOn(t *testing.T) {
	t.Parallel()

	eventTap := newTestTap(t)
	eventTap.SetModifierPassthrough(true, nil)

	for _, key := range []string{"Shift+A", "a", "Return"} {
		if _, consumed := routedKey(t, eventTap, key); !consumed {
			t.Errorf("%q reached the application instead of the mode", key)
		}
	}
}

// A chord RegisterHotKey owns is handed back undispatched whether or not
// passthrough is on, so exactly one of the two runs the binding.
func TestEventTap_HandleKey_HandsARegisteredHotkeyBack(t *testing.T) {
	t.Parallel()

	eventTap := newTestTap(t)
	eventTap.SetHotkeys([]string{"Ctrl+G"})
	eventTap.SetModifierPassthrough(false, nil)

	dispatched, consumed := routedKey(t, eventTap, "Ctrl+G")

	if consumed || len(dispatched) != 0 {
		t.Errorf(
			"consumed=%v dispatched=%v, want the chord left to RegisterHotKey",
			consumed,
			dispatched,
		)
	}
}
