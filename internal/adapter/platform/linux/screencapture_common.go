//go:build linux

package linux

import (
	"image"

	"github.com/y3owk1n/neru/internal/derrors"
)

// screenCaptureTimeoutMS bounds one Wayland capture exchange. Screencopy is a
// request/reply against the compositor, and a compositor that stops answering
// must surface as a failed capture rather than a wedged caller. Two seconds is
// far longer than a real grab (a 4K frame is tens of milliseconds) and short
// enough that a hint refresh does not appear to hang.
const screenCaptureTimeoutMS = 2000

// CaptureScreenRegion captures the pixels currently inside region and returns
// them as an RGBA image.
//
// region is in Neru's shared coordinate space: global origin, top-left, Y down,
// unscaled pixels. An empty region means "the whole active screen", which is
// ports.VisionPort.CaptureScreen's contract; it is resolved here so the native
// backends only ever receive a concrete rectangle and every capture goes down
// one code path. Honoring the region is the point of the parameter — the caller
// is normally the focused window, and reading a whole 4K display back to
// examine one window is the difference between usable and not.
//
// backend is the label NewSystemAdapter takes ("x11", "wayland-wlroots",
// "wayland-kde"). Capture is per-backend by construction: X11 reads the root
// window back, wlroots-family compositors implement wlr-screencopy, and a
// display server with neither reports CodeNotSupported naming itself.
//
// On a scaled Wayland output the compositor answers in physical pixels, so the
// returned image can be larger than the requested region — the same thing a
// Retina capture does on macOS.
//
// Privacy: the returned image is the only copy that outlives this call. The
// native buffers are wiped before they are freed or unmapped. Callers must
// never log it, derive log text from it, write it to disk, or hold it past the
// detection that asked for it.
func CaptureScreenRegion(backend string, region image.Rectangle) (*image.RGBA, error) {
	switch backend {
	case backendX11:
		resolved, err := resolveCaptureRegion(region, x11ActiveScreenBounds)
		if err != nil {
			return nil, err
		}

		return x11CaptureRegion(resolved)
	case backendWaylandWlroots, backendWaylandKDE:
		resolved, err := resolveCaptureRegion(region, wlrootsScreenBounds)
		if err != nil {
			return nil, err
		}

		return wlrootsCaptureRegion(resolved, backend)
	default:
		return nil, derrors.New(derrors.CodeNotSupported, unsupportedCaptureBackend(backend))
	}
}

// unsupportedCaptureBackend explains which display server has no capture path.
// "screen capture failed" means two different things on GNOME and on a session
// with no display server at all, and only the message can tell them apart.
func unsupportedCaptureBackend(backend string) string {
	if backend == "" {
		return "screen capture is unavailable: no display backend detected; " +
			"start a session under X11 or a Wayland compositor"
	}

	return "screen capture is not implemented on linux backend " + backend +
		"; supported backends are x11, wayland-wlroots and wayland-kde"
}

// resolveCaptureRegion turns the caller's request into the rectangle handed to
// a native backend. An empty rectangle means the active screen; anything else
// is canonicalized and passed through, so the region the caller asked for is
// the region that gets read back.
func resolveCaptureRegion(
	region image.Rectangle,
	activeScreen func() (image.Rectangle, error),
) (image.Rectangle, error) {
	canonical := region.Canon()
	if !canonical.Empty() {
		return canonical, nil
	}

	bounds, err := activeScreen()
	if err != nil {
		return image.Rectangle{}, err
	}

	bounds = bounds.Canon()
	if bounds.Empty() {
		return image.Rectangle{}, derrors.New(
			derrors.CodeActionFailed,
			"the active screen reports empty bounds; there is nothing to capture",
		)
	}

	return bounds, nil
}
