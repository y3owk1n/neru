//go:build linux

//nolint:testpackage // Exercises the unexported linuxAppWatcher poll/dispatch logic directly.
package appwatcher

import (
	"sync"
	"testing"
	"time"
)

const (
	kindActivate   = "activate"
	kindDeactivate = "deactivate"

	appFirefox = "firefox"
	appKonsole = "org.kde.konsole"
)

type watchEvent struct {
	kind   string
	name   string
	bundle string
}

// newRecordingWatcher returns a Watcher whose activate/deactivate events are
// appended to the returned slice pointer.
func newRecordingWatcher() (*Watcher, *[]watchEvent) {
	watcher := NewWatcher(nil)

	var events []watchEvent

	watcher.OnActivate(func(name, bundle string) {
		events = append(events, watchEvent{kindActivate, name, bundle})
	})
	watcher.OnDeactivate(func(name, bundle string) {
		events = append(events, watchEvent{kindDeactivate, name, bundle})
	})

	return watcher, &events
}

func TestLinuxAppWatcherTickDispatch(t *testing.T) {
	watcher, events := newRecordingWatcher()

	var (
		curID string
		curOK bool
	)

	poller := &linuxAppWatcher{
		identity: func(string) (string, bool) { return curID, curOK },
		interval: time.Millisecond,
		watcher:  watcher,
	}

	steps := []struct {
		name     string
		id       string
		ok       bool
		expected []watchEvent
	}{
		{
			name: "initial focus emits activate only",
			id:   appFirefox, ok: true,
			expected: []watchEvent{{kindActivate, appFirefox, appFirefox}},
		},
		{
			name: "same app repeated emits nothing",
			id:   appFirefox, ok: true,
			expected: nil,
		},
		{
			name: "switch app emits deactivate then activate",
			id:   appKonsole, ok: true,
			expected: []watchEvent{
				{kindDeactivate, appFirefox, appFirefox},
				{kindActivate, appKonsole, appKonsole},
			},
		},
		{
			name: "focus lost emits deactivate only",
			id:   "", ok: false,
			expected: []watchEvent{{kindDeactivate, appKonsole, appKonsole}},
		},
		{
			name: "still no focus emits nothing",
			id:   "", ok: false,
			expected: nil,
		},
		{
			name: "regain focus emits activate only",
			id:   appFirefox, ok: true,
			expected: []watchEvent{{kindActivate, appFirefox, appFirefox}},
		},
	}

	for _, step := range steps {
		curID, curOK = step.id, step.ok
		*events = nil

		poller.tick("x11")

		if !eventsEqual(*events, step.expected) {
			t.Errorf("%s: got %+v, want %+v", step.name, *events, step.expected)
		}
	}
}

// TestLinuxAppWatcherOKFalseWithIDTreatedAsNoFocus ensures a non-ok result is
// treated as "no focus" even when the identity returns a stray non-empty id.
func TestLinuxAppWatcherOKFalseWithIDTreatedAsNoFocus(t *testing.T) {
	watcher, events := newRecordingWatcher()

	poller := &linuxAppWatcher{
		identity: func(string) (string, bool) { return "stale", false },
		interval: time.Millisecond,
		watcher:  watcher,
	}

	poller.tick("wayland-kde")

	if len(*events) != 0 {
		t.Fatalf("expected no events when ok=false, got %+v", *events)
	}

	if poller.last != "" {
		t.Fatalf("expected last to stay empty, got %q", poller.last)
	}
}

func TestLinuxAppWatcherStartStopLifecycle(t *testing.T) {
	watcher, _ := newRecordingWatcher()

	var callMu sync.Mutex

	calls := 0

	poller := &linuxAppWatcher{
		identity: func(string) (string, bool) {
			callMu.Lock()
			calls++
			callMu.Unlock()

			return "app", true
		},
		interval: time.Millisecond,
		watcher:  watcher,
	}

	// stop before start must be safe.
	poller.stop()

	poller.start()

	// start is idempotent while running.
	poller.start()

	// Give the poll loop time to run a few ticks.
	time.Sleep(20 * time.Millisecond)

	poller.stop()

	callMu.Lock()
	got := calls
	callMu.Unlock()

	if got == 0 {
		t.Fatal("expected the poll loop to sample identity at least once")
	}

	// stop is idempotent.
	poller.stop()
}

func TestLinuxAppWatcherStartWithoutWatcherIsNoop(t *testing.T) {
	poller := &linuxAppWatcher{
		identity: func(string) (string, bool) { return "app", true },
		interval: time.Millisecond,
	}

	poller.start()
	defer poller.stop()

	poller.mu.Lock()
	running := poller.cancel != nil
	poller.mu.Unlock()

	if running {
		t.Fatal("start with no registered watcher should not launch the poll loop")
	}
}

func eventsEqual(left, right []watchEvent) bool {
	if len(left) != len(right) {
		return false
	}

	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}

	return true
}
