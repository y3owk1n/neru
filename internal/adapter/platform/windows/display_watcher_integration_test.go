//go:build integration && windows

package windows

import (
	"sync/atomic"
	"testing"
	"time"
)

// Real Win32 test for the display watcher. Changing the display mode on a CI
// runner is not an option, so the test posts the message Windows would
// broadcast to the watcher's own hidden window and checks that it comes out
// the callback.

var procPostMessageW = user32.NewProc("PostMessageW")

func waitForCount(t *testing.T, counter *atomic.Int32, want int32) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)

	for counter.Load() < want {
		if time.Now().After(deadline) {
			t.Fatalf("display change callbacks = %d, want at least %d", counter.Load(), want)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func TestDisplayWatcher_ObservesDisplayChangePostedToHiddenWindow(t *testing.T) {
	var fired atomic.Int32

	watcher, err := StartDisplayWatcher(func() { fired.Add(1) })
	if err != nil {
		t.Skipf("skipping: StartDisplayWatcher requires an interactive desktop (%v)", err)
	}

	defer watcher.Stop()

	hwnd := watcher.WindowHandle()

	ret, _, callErr := procPostMessageW.Call(hwnd, wmDisplayChange, 32, 0)
	if ret == 0 {
		t.Fatalf("PostMessageW(WM_DISPLAYCHANGE): %v", callErr)
	}

	waitForCount(t, &fired, 1)

	ret, _, callErr = procPostMessageW.Call(hwnd, wmDPIChanged, 0, 0)
	if ret == 0 {
		t.Fatalf("PostMessageW(WM_DPICHANGED): %v", callErr)
	}

	waitForCount(t, &fired, 2)

	watcher.Stop()

	before := fired.Load()

	// The window is destroyed with the pump, so the post fails rather than
	// reaching a callback; either way nothing may fire after Stop.
	discardCall(procPostMessageW.Call(hwnd, wmDisplayChange, 32, 0))

	time.Sleep(50 * time.Millisecond)

	if fired.Load() != before {
		t.Fatalf("callback fired after Stop: %d before, %d after", before, fired.Load())
	}
}
