//go:build linux

package appwatcher

import (
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
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
			// A stray id beside ok=false is still no focus, and the regain
			// step below would see it as a deactivate if it were remembered.
			name: "not ok with a stray id emits nothing",
			id:   "stale", ok: false,
			expected: nil,
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

// TestLinuxAppWatcherEventLoopWakesOnFD proves the event-driven path dispatches
// an activation when the focus-change fd becomes readable, rather than waiting
// for a poll interval. A real non-blocking pipe stands in for the platform
// focus fd, and writing a byte simulates a compositor/X11 focus-change signal.
func TestLinuxAppWatcherEventLoopWakesOnFD(t *testing.T) {
	watcher := NewWatcher(nil)

	activated := make(chan string, 4)
	watcher.OnActivate(func(_, bundle string) { activated <- bundle })

	var pipeFDs [2]int

	err := unix.Pipe(pipeFDs[:])
	if err != nil {
		t.Fatalf("unix.Pipe: %v", err)
	}

	err = unix.SetNonblock(pipeFDs[0], true)
	if err != nil {
		t.Fatalf("SetNonblock: %v", err)
	}

	t.Cleanup(func() {
		_ = unix.Close(pipeFDs[0])
		_ = unix.Close(pipeFDs[1])
	})

	var (
		curMu sync.Mutex
		cur   string
	)

	poller := &linuxAppWatcher{
		identity: func(string) (string, bool) {
			curMu.Lock()
			defer curMu.Unlock()

			if cur == "" {
				return "", false
			}

			return cur, true
		},
		// A large interval ensures a passing test exercises the fd wake, not the
		// safety-net re-sample.
		subscribe: func(string) (int, bool) { return pipeFDs[0], true },
		interval:  time.Hour,
		watcher:   watcher,
	}

	poller.start()
	t.Cleanup(poller.stop)

	// Change focus, then signal via the fd.
	curMu.Lock()
	cur = appFirefox
	curMu.Unlock()

	_, err = unix.Write(pipeFDs[1], []byte{1})
	if err != nil {
		t.Fatalf("unix.Write: %v", err)
	}

	select {
	case got := <-activated:
		if got != appFirefox {
			t.Fatalf("activate bundle = %q, want %q", got, appFirefox)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("event loop did not dispatch activate after fd signal")
	}
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
