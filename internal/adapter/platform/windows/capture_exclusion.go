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
// to a screen share or recording the rest of the time. Hiding the overlay from
// screen shares keeps the affinity on every window instead.
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

	// hiddenFromCapture says the overlay is hidden from screen shares, so
	// every window Neru shows keeps the affinity instead of taking it for one
	// capture. affinityMu serializes a change of the flag with a window
	// registering, so a new window cannot miss the change. It is a leaf, taken
	// after captureMu.
	hiddenFromCapture atomic.Bool
	affinityMu        sync.Mutex
)

// CaptureExcludesOwnWindows reports whether CaptureScreenRegion leaves Neru's
// own windows out, which takes Windows 10 version 2004 or later and every
// window accepting the affinity.
func CaptureExcludesOwnWindows() bool {
	return supportsExclusion() && !exclusionRefused.Load()
}

func registerOwnWindow(hwnd uintptr) {
	affinityMu.Lock()
	defer affinityMu.Unlock()

	ownWindows.Store(hwnd, struct{}{})

	if hiddenFromCapture.Load() {
		excludeFromCapture(hwnd)
	}
}

// excludeFromCapture keeps hwnd out of every capture, and reports whether a
// live window refused. One that refused stays in every capture, so it turns
// CaptureExcludesOwnWindows false like a refusal during a capture does.
func excludeFromCapture(hwnd uintptr) bool {
	ret, _, _ := procSetWindowDisplayAffinity.Call(hwnd, wdaExcludeFromCapture)
	if ret != 0 {
		return false
	}

	if live, _, _ := procIsWindow.Call(hwnd); live == 0 {
		return false
	}

	exclusionRefused.Store(true)

	return true
}

// SetOwnWindowsHiddenFromCapture keeps every window Neru shows out of screen
// captures, screen shares and recordings included, or lets them back in. The
// windows stay on the monitor either way. Like the per-capture exclusion it
// takes Windows 10 version 2004 or later. An earlier build reads the affinity
// as WDA_MONITOR and would black the full-screen overlay out of the share, so
// there hiding reports CodeNotSupported and changes nothing. A window that
// refuses the affinity stays in the share, and hiding reports that too.
//
// It waits one compositor frame when a visible window changed, so a capture
// queued behind it on captureMu reads a frame that already has the change.
func SetOwnWindowsHiddenFromCapture(hidden bool) error {
	if !supportsExclusion() && hidden {
		return derrors.New(
			derrors.CodeNotSupported,
			"hiding windows from screen captures needs Windows 10 version 2004",
		)
	}

	if !supportsExclusion() {
		return nil
	}

	captureMu.Lock()
	defer captureMu.Unlock()

	affinityMu.Lock()
	defer affinityMu.Unlock()

	hiddenFromCapture.Store(hidden)

	var refused, anyVisible bool

	ownWindows.Range(func(key, _ any) bool {
		hwnd, _ := key.(uintptr)
		if hidden {
			refused = excludeFromCapture(hwnd) || refused
		} else {
			discardCall(procSetWindowDisplayAffinity.Call(hwnd, wdaNone))
		}

		if visible, _, _ := procIsWindowVisible.Call(hwnd); visible != 0 {
			anyVisible = true
		}

		return true
	})

	if anyVisible {
		discardCall(procDwmFlush.Call())
	}

	if refused {
		return derrors.New(
			derrors.CodeActionFailed,
			"a window refused to be hidden from screen captures",
		)
	}

	return nil
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

	// While the overlay is hidden from screen shares the windows already have
	// the affinity, and clearing it after the capture would show them in the
	// share.
	if hiddenFromCapture.Load() {
		return capture()
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
