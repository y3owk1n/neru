//go:build windows

package windows

import (
	"context"
	"image"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/windows"

	"github.com/y3owk1n/neru/internal/derrors"
)

// A screen capture reads the desktop as composed, Neru's own windows included,
// so a pixel detector run while a mode is drawn would read the overlay as part
// of the interface. Windows 10 version 2004 added a display affinity that
// leaves a window out of every capture while it stays on the monitor. Each
// capture sets it on every window Neru shows, waits for the compositor to
// apply it, reads the screen and clears it again, so the windows stay visible
// to a screen share or recording the rest of the time.
const (
	wdaNone               = 0x00000000
	wdaExcludeFromCapture = 0x00000011

	// excludeFromCaptureBuild is Windows 10 version 2004, the first build
	// that honors wdaExcludeFromCapture.
	excludeFromCaptureBuild = 19041
)

var (
	procSetWindowDisplayAffinity = user32.NewProc("SetWindowDisplayAffinity")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procDwmFlush                 = windows.NewLazySystemDLL("dwmapi.dll").NewProc("DwmFlush")

	// ownWindows holds the handle of every top-level window Neru shows.
	ownWindows sync.Map

	// captureMu runs one capture at a time, so one capture clearing the
	// affinity cannot uncover the windows while another is still reading.
	captureMu sync.Mutex

	supportsExclusion = sync.OnceValue(func() bool {
		return windows.RtlGetVersion().BuildNumber >= excludeFromCaptureBuild
	})

	// exclusionRefused is set once a live window refused the affinity. Its
	// capture read that window, so from then on CaptureExcludesOwnWindows
	// answers false and callers take their own drawing off first.
	exclusionRefused atomic.Bool
)

// CaptureExcludesOwnWindows reports whether CaptureScreenRegion leaves Neru's
// own windows out, which takes Windows 10 version 2004 or later and every
// window accepting the affinity.
func CaptureExcludesOwnWindows() bool {
	return supportsExclusion() && !exclusionRefused.Load()
}

func registerOwnWindow(hwnd uintptr) {
	ownWindows.Store(hwnd, struct{}{})
}

func unregisterOwnWindow(hwnd uintptr) {
	ownWindows.Delete(hwnd)
}

// withOwnWindowsExcluded runs capture while every window Neru shows is left
// out of screen captures. Hidden windows get the affinity too, so one shown
// while the capture runs, such as an indicator appearing, is already left
// out. Only a visible window makes the capture wait a compositor frame, so a
// capture with none visible pays nothing. It skips a window that closed in
// the meantime. A live window that refuses the affinity shows up in the
// capture, and turns CaptureExcludesOwnWindows false for every capture after
// it.
//
// It checks ctx again once it holds the lock, because a capture can queue here
// behind another and its deadline can pass while it waits.
func withOwnWindowsExcluded(
	ctx context.Context,
	capture func() (*image.RGBA, error),
) (*image.RGBA, error) {
	if !supportsExclusion() {
		return capture()
	}

	captureMu.Lock()
	defer captureMu.Unlock()

	err := ctx.Err()
	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeContextCanceled, "screen capture canceled")
	}

	var (
		excluded   []uintptr
		anyVisible bool
	)

	ownWindows.Range(func(key, _ any) bool {
		hwnd, _ := key.(uintptr)

		ret, _, _ := procSetWindowDisplayAffinity.Call(hwnd, wdaExcludeFromCapture)
		if ret == 0 {
			if live, _, _ := procIsWindow.Call(hwnd); live != 0 {
				exclusionRefused.Store(true)
			}

			return true
		}

		excluded = append(excluded, hwnd)

		if visible, _, _ := procIsWindowVisible.Call(hwnd); visible != 0 {
			anyVisible = true
		}

		return true
	})

	defer func() {
		for _, hwnd := range excluded {
			discardCall(procSetWindowDisplayAffinity.Call(hwnd, wdaNone))
		}
	}()

	// The affinity applies at the next composition, so the capture waits for
	// it rather than reading a frame that still has the windows in it.
	if anyVisible {
		discardCall(procDwmFlush.Call())
	}

	return capture()
}
