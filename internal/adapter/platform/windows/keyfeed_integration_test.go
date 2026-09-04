//go:build integration && windows

package windows

import (
	"runtime"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Real Win32 test for FeedKey: a key fed through SendInput must arrive as
// WM_KEYDOWN / WM_KEYUP in a window this process owns and holds in the
// foreground. In-package because the message pump helpers are unexported.

const (
	keyFeedTestClass = "NeruKeyFeedTest"
	homeKeyName      = "Home"
	// keyFeedArrival bounds how long an injected key may take to reach the
	// window's queue. SendInput is synchronous into the system queue, so a
	// real arrival is milliseconds; the margin is for a loaded runner.
	keyFeedArrival = 2 * time.Second
	// extendedKeyBit is bit 24 of a WM_KEYDOWN lParam: the scan code carried
	// the 0xE0 prefix.
	extendedKeyBit = 1 << 24
)

var procSetFocus = user32.NewProc("SetFocus")

func TestFeedKey_ArrivesInOwnedWindow(t *testing.T) {
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	hwnd := newForegroundTestWindow(t)

	tests := []struct {
		key        string
		virtualKey uintptr
		extended   bool
	}{
		{key: "a", virtualKey: 'A', extended: false},
		{key: homeKeyName, virtualKey: vkHome, extended: true},
	}

	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			err := FeedKey(test.key)
			if err != nil {
				t.Fatalf("FeedKey(%q): %v", test.key, err)
			}

			down := awaitKeyMessages(t, hwnd, test.virtualKey)

			extended := down&extendedKeyBit != 0
			if extended != test.extended {
				t.Fatalf(
					"FeedKey(%q): extended flag = %v, want %v",
					test.key,
					extended,
					test.extended,
				)
			}
		})
	}
}

// newForegroundTestWindow creates a plain top-level window and brings it to
// the foreground, skipping when the session refuses foreground activation,
// which is what a non-interactive window station does.
func newForegroundTestWindow(t *testing.T) uintptr {
	t.Helper()

	className, err := windows.UTF16PtrFromString(keyFeedTestClass)
	if err != nil {
		t.Fatalf("UTF16PtrFromString: %v", err)
	}

	class := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   procDefWindowProcW.Addr(),
		hInstance:     windows.Handle(moduleHandle()),
		lpszClassName: className,
	}

	atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		t.Skipf("skipping: RegisterClassExW: %v", err)
	}

	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0,
		wsOverlappedWindow|wsVisible,
		100, 100, 200, 200,
		0, 0, moduleHandle(), 0,
	)
	if hwnd == 0 {
		t.Skipf("skipping: CreateWindowExW: %v", err)
	}

	t.Cleanup(func() { discardCall(procDestroyWindow.Call(hwnd)) })

	discardCall(procSetForegroundWindow.Call(hwnd))
	discardCall(procSetFocus.Call(hwnd))
	drainWindowMessages(hwnd)

	if uintptr(windows.GetForegroundWindow()) != hwnd {
		t.Skip(
			"skipping: the session refused foreground activation, so no window can receive input",
		)
	}

	return hwnd
}

// awaitKeyMessages pumps the window's queue until WM_KEYUP for virtualKey
// arrives, returning the WM_KEYDOWN lParam, and fails at the deadline.
func awaitKeyMessages(t *testing.T, hwnd uintptr, virtualKey uintptr) uintptr {
	t.Helper()

	deadline := time.Now().Add(keyFeedArrival)

	var (
		downParam uintptr
		sawDown   bool
	)

	for time.Now().Before(deadline) {
		var message msg

		ret, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&message)), hwnd, 0, 0, pmRemove)
		if ret == 0 {
			time.Sleep(5 * time.Millisecond)

			continue
		}

		switch {
		case message.message == wmKeyDown && message.wParam == virtualKey:
			downParam = message.lParam
			sawDown = true
		case message.message == wmKeyUp && message.wParam == virtualKey:
			if !sawDown {
				t.Fatal("WM_KEYUP arrived before WM_KEYDOWN")
			}

			return downParam
		}

		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}

	t.Fatalf(
		"no WM_KEYUP for virtual key %#x within %v (WM_KEYDOWN seen: %v)",
		virtualKey,
		keyFeedArrival,
		sawDown,
	)

	return 0
}

func drainWindowMessages(hwnd uintptr) {
	for range maxMessagesPerPump {
		var message msg

		ret, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&message)), hwnd, 0, 0, pmRemove)
		if ret == 0 {
			return
		}

		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}
