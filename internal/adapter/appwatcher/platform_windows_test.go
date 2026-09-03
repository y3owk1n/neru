//go:build windows

package appwatcher

import (
	"errors"
	"sync"
	"testing"
	"time"
)

const (
	kindActivate   = "activate"
	kindDeactivate = "deactivate"

	hwndNotepad = uintptr(0x10)
	hwndCode    = uintptr(0x20)
	hwndDesktop = uintptr(0)

	nameNotepad = "notepad"
	nameCode    = "Code"
	pathNotepad = `C:\Windows\notepad.exe`
	pathCode    = `C:\Users\me\AppData\Local\Programs\Microsoft VS Code\Code.exe`
)

var errHookInstall = errors.New("SetWinEventHook failed")

type watchEvent struct {
	kind   string
	name   string
	bundle string
}

// eventRecorder collects Watcher dispatches from the backend's goroutine.
type eventRecorder struct {
	mu     sync.Mutex
	events []watchEvent
}

func (r *eventRecorder) add(kind, name, bundle string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, watchEvent{kind, name, bundle})
}

// waitFor blocks until the recorder holds want events, or fails the test.
func (r *eventRecorder) waitFor(t *testing.T, want []watchEvent) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for {
		r.mu.Lock()
		got := append([]watchEvent(nil), r.events...)
		r.mu.Unlock()

		if len(got) >= len(want) {
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("event %d = %+v, want %+v (all: %+v)", i, got[i], want[i], got)
				}
			}

			if len(got) > len(want) {
				t.Fatalf("got %d events, want %d: %+v", len(got), len(want), got)
			}

			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %+v, got %+v", want, got)
		}

		time.Sleep(5 * time.Millisecond)
	}
}

// fakeHook stands in for the Win32 foreground hook: the test fires events by
// calling the callback the backend subscribed with.
type fakeHook struct {
	mu       sync.Mutex
	callback func(uintptr)
	unhooked bool
	err      error
}

func (f *fakeHook) subscribe(callback func(uintptr)) (func(), error) {
	if f.err != nil {
		return nil, f.err
	}

	f.mu.Lock()
	f.callback = callback
	f.mu.Unlock()

	return func() {
		f.mu.Lock()
		f.unhooked = true
		f.callback = nil
		f.mu.Unlock()
	}, nil
}

func (f *fakeHook) fire(hwnd uintptr) {
	f.mu.Lock()
	callback := f.callback
	f.mu.Unlock()

	if callback != nil {
		callback(hwnd)
	}
}

func identityFor(hwnd uintptr) (string, string, bool) {
	switch hwnd {
	case hwndNotepad:
		return nameNotepad, pathNotepad, true
	case hwndCode:
		return nameCode, pathCode, true
	default:
		return "", "", false
	}
}

func newTestBackend(hook *fakeHook, foreground uintptr) (*windowsAppWatcher, *eventRecorder) {
	watcher := NewWatcher(nil)
	recorder := &eventRecorder{}

	watcher.OnActivate(func(name, bundle string) { recorder.add(kindActivate, name, bundle) })
	watcher.OnDeactivate(func(name, bundle string) { recorder.add(kindDeactivate, name, bundle) })

	backend := &windowsAppWatcher{
		subscribe:  hook.subscribe,
		foreground: func() uintptr { return foreground },
		identity:   identityFor,
	}
	backend.register(watcher)

	return backend, recorder
}

func TestWindowsAppWatcher_Start_PublishesForegroundChanges(t *testing.T) {
	hook := &fakeHook{}
	backend, recorder := newTestBackend(hook, hwndNotepad)

	backend.start()
	defer backend.stop()

	// The app in front at start is published without waiting for a switch.
	recorder.waitFor(t, []watchEvent{
		{kindActivate, nameNotepad, pathNotepad},
	})

	hook.fire(hwndCode)
	recorder.waitFor(t, []watchEvent{
		{kindActivate, nameNotepad, pathNotepad},
		{kindDeactivate, nameNotepad, pathNotepad},
		{kindActivate, nameCode, pathCode},
	})

	// A second window of the same application is not a change of application.
	hook.fire(hwndCode)
	// Focusing the desktop deactivates without activating anything.
	hook.fire(hwndDesktop)
	recorder.waitFor(t, []watchEvent{
		{kindActivate, nameNotepad, pathNotepad},
		{kindDeactivate, nameNotepad, pathNotepad},
		{kindActivate, nameCode, pathCode},
		{kindDeactivate, nameCode, pathCode},
	})
}

func TestWindowsAppWatcher_Stop_UnhooksBeforeSilencing(t *testing.T) {
	hook := &fakeHook{}
	backend, recorder := newTestBackend(hook, hwndDesktop)

	backend.start()
	backend.stop()

	if !hook.unhooked {
		t.Fatal("stop did not unhook the foreground event hook")
	}

	hook.fire(hwndNotepad)
	time.Sleep(20 * time.Millisecond)

	recorder.waitFor(t, nil)

	// A second stop is a no-op, and a restart is a fresh subscription.
	backend.stop()
	backend.start()

	defer backend.stop()

	hook.fire(hwndNotepad)
	recorder.waitFor(t, []watchEvent{{kindActivate, nameNotepad, pathNotepad}})
}

func TestWindowsAppWatcher_Start_HookFailureLeavesWatcherIdle(t *testing.T) {
	hook := &fakeHook{err: errHookInstall}
	backend, recorder := newTestBackend(hook, hwndNotepad)

	backend.start()
	backend.stop()

	recorder.waitFor(t, nil)
}
