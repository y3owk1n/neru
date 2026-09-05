//go:build windows

package windows

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

// A held hotkey reaches the binder as one press and one release, which is what
// lets it repeat the binding while the key is down. RegisterHotKey reports the
// press only, so the release is read from the key's state after it.
func TestHotkeyRegistry_AHeldHotkeyIsOnePressAndOneRelease(t *testing.T) {
	t.Parallel()

	const (
		hotkeyID   = 1
		virtualKey = 0x47
	)

	// The key reads down for the first two polls and up from the third.
	polls := 0
	registry := &HotkeyRegistry{
		callbacks:  make(map[int]hotkeyCallbacks),
		held:       make(map[int]*hotkeyHold),
		registered: make(map[int]hotkeyRegistration),
		logger:     zap.NewNop(),
		keyDown: func(uint32) bool {
			polls++

			return polls <= 2
		},
	}

	pressed := make(chan struct{}, 4)
	released := make(chan struct{}, 4)

	registry.registered[hotkeyID] = hotkeyRegistration{id: hotkeyID, virtualKey: virtualKey}
	registry.callbacks[hotkeyID] = hotkeyCallbacks{
		press:   func() { pressed <- struct{}{} },
		release: func() { released <- struct{}{} },
	}

	registry.handleHotkeyMessage(hotkeyID)
	// The autorepeat WM_HOTKEY of a key still held.
	registry.handleHotkeyMessage(hotkeyID)

	select {
	case <-pressed:
	default:
		t.Fatal("the press did not fire")
	}

	select {
	case <-pressed:
		t.Fatal("a WM_HOTKEY while held fired the press again; the binder would restart its repeat")
	default:
	}

	select {
	case <-released:
	case <-time.After(2 * time.Second):
		t.Fatal("the key read up and no release fired; the binder would repeat forever")
	}

	// The next press is a new hold, however soon it follows the release.
	registry.handleHotkeyMessage(hotkeyID)

	select {
	case <-pressed:
	default:
		t.Fatal("a press after the release fired nothing")
	}
}
