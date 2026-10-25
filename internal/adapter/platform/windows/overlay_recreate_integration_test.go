//go:build integration && windows

package windows

import (
	"image"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestOverlayWindow_Show_RecreateCatchesUpToAMidCreateResize stops a recreate
// on the UI thread right after it snapshots the bounds and calls ResizeTo in
// that gap. The handle is 0 there, so ResizeTo only records the bounds, and
// the window that Show finishes building has to end up at them anyway. It is
// not parallel, because the hook is package state.
func TestOverlayWindow_Show_RecreateCatchesUpToAMidCreateResize(t *testing.T) {
	overlay, err := NewOverlayWindowAt(0, 0, 64, 64)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "interactive") ||
			strings.Contains(msg, "CreateWindowExW") ||
			strings.Contains(msg, "RegisterClassExW") {
			t.Skipf("skipping: overlay requires interactive desktop (%v)", err)
		}

		t.Fatalf("NewOverlayWindowAt: %v", err)
	}

	t.Cleanup(overlay.Destroy)

	overlay.Destroy()

	paused := make(chan struct{})
	resume := make(chan struct{})
	release := sync.OnceFunc(func() { close(resume) })

	afterBoundsSnapshot = func(created *OverlayWindow) {
		if created != overlay {
			return
		}

		close(paused)
		<-resume
	}

	// Registered after Destroy, so it runs first. A failing test must not
	// leave the UI thread parked in the hook.
	t.Cleanup(func() {
		release()

		afterBoundsSnapshot = nil
	})

	shown := make(chan struct{})

	go func() {
		overlay.Show()
		close(shown)
	}()

	select {
	case <-paused:
	case <-time.After(5 * time.Second):
		t.Fatal("Show never reached the bounds snapshot")
	}

	want := image.Rect(10, 20, 106, 68)

	err = overlay.ResizeTo(want.Min.X, want.Min.Y, want.Dx(), want.Dy())
	if err != nil {
		t.Fatalf("ResizeTo: %v", err)
	}

	release()

	select {
	case <-shown:
	case <-time.After(5 * time.Second):
		t.Fatal("Show never finished")
	}

	hwnd := overlay.HWND()
	if hwnd == 0 {
		t.Fatal("Show did not recreate the window")
	}

	var rect windows.Rect

	ret, _, callErr := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
	if ret == 0 {
		t.Fatalf("GetWindowRect: %v", callErr)
	}

	if got := rectToImage(rect); got != want {
		t.Fatalf("window rect = %v, want the mid-create ResizeTo %v", got, want)
	}
}
