//go:build integration && windows

package windows

import (
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestOverlayUI_RespondsToSentMessagesWhileIdle(t *testing.T) {
	hwnd := newIdleOverlayTestWindow(t)

	var result uintptr

	// SMTO_ABORTIFHUNG bounds the probe without sending keyboard input or
	// queueing an overlay callback that would mask an idle message-pump bug.
	const abortIfHung = 0x0002

	ret, _, err := user32.NewProc("SendMessageTimeoutW").Call(
		hwnd, 0, 0, 0, abortIfHung, 1000, uintptr(unsafe.Pointer(&result)),
	)
	if ret == 0 {
		t.Fatalf("idle overlay thread did not answer WM_NULL: %v", err)
	}
}

func TestOverlayUI_DispatchesQueuedMessagesWhileIdle(t *testing.T) {
	hwnd := newIdleOverlayTestWindow(t)

	const closeMessage = 0x0010

	ret, _, err := procPostMessageW.Call(hwnd, closeMessage, 0, 0)
	if ret == 0 {
		t.Fatalf("PostMessageW: %v", err)
	}

	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()

	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for {
		alive, _, _ := procIsWindow.Call(hwnd)
		if alive == 0 {
			return
		}

		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("idle overlay thread did not dispatch WM_CLOSE")
		}
	}
}

func TestOverlayUI_PostedCallbackWakesIdleThread(t *testing.T) {
	_ = newIdleOverlayTestWindow(t)
	done := make(chan struct{})

	postOnOverlayUI(func() {
		// Reentrant UI work must still run inline rather than wait on itself.
		runOnOverlayUI(func() { close(done) })
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("posted callback did not wake the idle overlay thread")
	}
}

func newIdleOverlayTestWindow(t *testing.T) uintptr {
	t.Helper()

	className, err := windows.UTF16PtrFromString("STATIC")
	if err != nil {
		t.Fatal(err)
	}

	var hwnd uintptr

	var createErr error
	runOnOverlayUI(func() {
		// Hidden, non-activating window: no desktop input or visible overlay.
		hwnd, _, createErr = procCreateWindowExW.Call(
			wsExToolWindow|wsExNoActivate, uintptr(unsafe.Pointer(className)),
			0, wsPopup, 0, 0, 1, 1, 0, 0, moduleHandle(), 0,
		)
	})

	if hwnd == 0 {
		t.Fatalf("CreateWindowExW: %v", createErr)
	}

	t.Cleanup(func() {
		runOnOverlayUI(func() {
			alive, _, _ := procIsWindow.Call(hwnd)
			if alive != 0 {
				discardCall(procDestroyWindow.Call(hwnd))
			}
		})
	})

	// Let the creation callback's final pump finish before sending a probe.
	// No further Go callbacks may wake the UI thread during the assertion.
	time.Sleep(50 * time.Millisecond)

	return hwnd
}
