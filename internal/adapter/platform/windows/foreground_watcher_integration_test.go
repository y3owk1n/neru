//go:build integration && windows

package windows

import (
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Real Win32 test for the foreground watcher. In-package because the window
// creation it drives reuses the package's user32 bindings.
//
// Windows only lets a process bring its own window to the foreground while
// that process is allowed to take it (it owns the foreground already, or was
// launched from it), so the test skips rather than fails when the OS declines
// the request: on a headless runner the hook installs, but nothing can be
// switched to.

var procSetForegroundWindow = user32.NewProc("SetForegroundWindow")

const (
	wsOverlappedWindow = 0x00CF0000
	wsVisible          = 0x10000000
	cwUseDefault       = 0x80000000
	testWindowSize     = 200
)

func createTestWindow(t *testing.T, title string) uintptr {
	t.Helper()

	className, err := windows.UTF16PtrFromString("STATIC")
	if err != nil {
		t.Fatalf("UTF16PtrFromString: %v", err)
	}

	windowName, err := windows.UTF16PtrFromString(title)
	if err != nil {
		t.Fatalf("UTF16PtrFromString: %v", err)
	}

	hwnd, _, callErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		wsOverlappedWindow|wsVisible,
		cwUseDefault,
		cwUseDefault,
		testWindowSize,
		testWindowSize,
		0,
		0,
		moduleHandle(),
		0,
	)
	if hwnd == 0 {
		t.Skipf("skipping: CreateWindowExW requires an interactive desktop (%v)", callErr)
	}

	t.Cleanup(func() { discardCall(procDestroyWindow.Call(hwnd)) })

	return hwnd
}

// pumpMessages drains the calling thread's queue so the windows it owns
// process the activation messages a foreground change sends them.
func pumpMessages() {
	var message msg

	for {
		ret, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, pmRemove)
		if ret == 0 {
			return
		}

		discardCall(procTranslateMessage.Call(uintptr(unsafe.Pointer(&message))))
		discardCall(procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message))))
	}
}

func bringToFront(t *testing.T, hwnd uintptr) {
	t.Helper()

	ret, _, _ := procSetForegroundWindow.Call(hwnd)

	pumpMessages()

	if ret == 0 || uintptr(windows.GetForegroundWindow()) != hwnd {
		t.Skip("skipping: this session does not let the test process take the foreground")
	}
}

func TestForegroundWatcher_ObservesFocusMovingBetweenTwoWindows(t *testing.T) {
	// The windows and the foreground requests must come from one thread, the
	// one whose queue pumpMessages drains.
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	var (
		seenMu sync.Mutex
		seen   []uintptr
	)

	watcher, err := StartForegroundWatcher(func(hwnd uintptr) {
		seenMu.Lock()
		defer seenMu.Unlock()

		seen = append(seen, hwnd)
	})
	if err != nil {
		t.Fatalf("StartForegroundWatcher: %v", err)
	}

	defer watcher.Stop()

	first := createTestWindow(t, "neru-foreground-first")
	second := createTestWindow(t, "neru-foreground-second")

	bringToFront(t, first)
	bringToFront(t, second)
	bringToFront(t, first)

	observed := func(want uintptr) bool {
		seenMu.Lock()
		defer seenMu.Unlock()

		return slices.Contains(seen, want)
	}

	deadline := time.Now().Add(3 * time.Second)

	for !observed(first) || !observed(second) {
		if time.Now().After(deadline) {
			seenMu.Lock()

			got := append([]uintptr(nil), seen...)

			seenMu.Unlock()

			t.Fatalf(
				"foreground events did not name both windows: got %#v, want %#x and %#x",
				got,
				first,
				second,
			)
		}

		pumpMessages()
		time.Sleep(10 * time.Millisecond)
	}

	watcher.Stop()

	seenMu.Lock()
	count := len(seen)
	seenMu.Unlock()

	bringToFront(t, second)
	time.Sleep(50 * time.Millisecond)

	seenMu.Lock()
	defer seenMu.Unlock()

	if len(seen) != count {
		t.Fatalf("hook fired after Stop: %d events before, %d after", count, len(seen))
	}
}
