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

// captureStatus mirrors the NERU_CAPTURE_* codes in screencapture.h.
//
// It is a mirror rather than the cgo constants themselves because a Go test
// file cannot use cgo, and the mapping from a native failure to the sentence a
// user reads is exactly the part worth testing. screencapture_cgo.go carries
// constant expressions that fail to compile if the two lists ever disagree.
type captureStatus int

const (
	captureStatusOK captureStatus = iota
	captureStatusNoDisplay
	captureStatusNoProtocol
	captureStatusNoOutput
	captureStatusRegion
	captureStatusFormat
	captureStatusAlloc
	captureStatusFailed
	captureStatusTimeout
)

// captureError maps a native capture status onto the shared error vocabulary.
//
// what names the display server, so a user reading the message knows which one
// refused rather than only that "capture failed" — the difference between
// "KWin cannot do this, and here is the gap" and "something went wrong".
//
// The two statuses that mean "this display server will never do this" are
// CodeNotSupported, so callers degrade instead of retrying; everything else is
// a live failure.
func captureError(status captureStatus, what string) error {
	switch status {
	case captureStatusNoDisplay:
		return derrors.New(
			derrors.CodeNotSupported,
			"screen capture is unavailable: could not connect to "+what,
		)
	case captureStatusNoProtocol:
		return derrors.New(
			derrors.CodeNotSupported,
			what+" does not implement wlr-screencopy-unstable-v1, the protocol Neru "+
				"captures the screen with",
		)
	case captureStatusFormat:
		return derrors.New(
			derrors.CodeNotSupported,
			what+" offered a pixel format Neru cannot read",
		)
	case captureStatusNoOutput:
		return derrors.New(
			derrors.CodeActionFailed,
			"no output of "+what+" covers the requested region",
		)
	case captureStatusRegion:
		return derrors.New(
			derrors.CodeActionFailed,
			"the requested capture region is empty or lies outside the screen",
		)
	case captureStatusAlloc:
		return derrors.New(
			derrors.CodeInternal,
			"could not allocate a buffer for the screen capture",
		)
	case captureStatusTimeout:
		return derrors.New(
			derrors.CodeActionFailed,
			what+" did not answer the screen capture in time",
		)
	case captureStatusOK, captureStatusFailed:
		return derrors.New(
			derrors.CodeActionFailed,
			what+" failed to capture the screen",
		)
	default:
		return derrors.New(
			derrors.CodeActionFailed,
			what+" failed to capture the screen",
		)
	}
}

// Names capture errors use for the display server that refused. They are the
// user-facing half of every message in captureError, so they live beside it.
const (
	captureLabelXServer    = "the X server"
	captureLabelCompositor = "this Wayland compositor"
	captureLabelKDE        = "KDE Plasma (KWin)"
)

// captureCompositorLabel names the compositor family in capture errors. KDE is
// spelled out because it is the one that reaches captureStatusNoProtocol on a
// healthy session, and the documentation stakes a Known Gaps entry on it
// saying so.
func captureCompositorLabel(backend string) string {
	if backend == backendWaylandKDE {
		return captureLabelKDE
	}

	return captureLabelCompositor
}

// CaptureScreenRegion captures the pixels currently inside region and returns
// them as an RGBA image.
//
// region is in Neru's shared coordinate space: global origin, top-left, Y down,
// unscaled pixels. The zero rectangle means "the whole active screen" — the
// screen holding the cursor, which is what ports.VisionPort.CaptureScreen means
// on Linux (Wayland exposes no primary display) — and is resolved here so the
// native backends only ever receive a concrete rectangle and every capture goes
// down one code path. Honoring the region is the point of the parameter: the
// caller is normally the focused window, and reading a whole 4K display back to
// examine one window is the difference between usable and not.
//
// What comes back covers **exactly** the requested region. A rectangle that
// leaves the screen, or that spans two monitors on Wayland, fails rather than
// coming back clipped, because a clipped frame carries nothing that says where
// its top-left actually is. The image's own bounds start at (0, 0); the region
// passed in is what places those pixels.
//
// backend is the label NewSystemAdapter takes ("x11", "wayland-wlroots",
// "wayland-kde"). Capture is per-backend by construction: X11 reads the root
// window back, wlroots-family compositors implement wlr-screencopy, and a
// display server with neither reports CodeNotSupported naming itself.
//
// On a scaled Wayland output the compositor answers in physical pixels, so the
// returned image can be larger than the requested region by the output's scale
// factor — the same thing a Retina capture does on macOS.
//
// Privacy: the returned image is the only copy that outlives this call. The
// native buffers are wiped before they are freed or unmapped. Callers must
// never log it, derive log text from it, write it to disk, or hold it past the
// detection that asked for it.
func CaptureScreenRegion(backend string, region image.Rectangle) (*image.RGBA, error) {
	if backend == backendX11 {
		resolved, err := resolveCaptureRegion(region, x11ActiveScreenBounds)
		if err != nil {
			return nil, err
		}

		return x11CaptureRegion(resolved)
	}

	if backendUsesWlrClientStack(backend) {
		resolved, err := resolveCaptureRegion(region, wlrootsScreenBounds)
		if err != nil {
			return nil, err
		}

		return wlrootsCaptureRegion(resolved, backend)
	}

	return nil, derrors.New(derrors.CodeNotSupported, unsupportedCaptureBackend(backend))
}

// unsupportedCaptureBackend explains which display server has no capture path.
// "screen capture failed" means two different things on GNOME and on a session
// with no display server at all, and only the message can tell them apart.
func unsupportedCaptureBackend(backend string) string {
	// DetectLinuxBackend spells "nothing detected" as "unknown"; an empty label
	// reaches here from a SystemAdapter built before detection ran. Both mean
	// there is no session, and both deserve the sentence that says what to do.
	if backend == "" || backend == backendUnknown {
		return "screen capture is unavailable: no display backend detected; " +
			"start a session under X11 or a Wayland compositor"
	}

	return "screen capture is not implemented on linux backend " + backend +
		"; supported backends are x11, wayland-wlroots and wayland-kde"
}

// resolveCaptureRegion turns the caller's request into the rectangle handed to
// a native backend.
//
// Only the *zero* rectangle means "the whole active screen". A region that was
// asked for and came out degenerate — a focused-window rectangle that collapsed
// to zero height, say — is refused rather than widened, because the one thing a
// caller asking for a window must never get instead is everything on screen.
func resolveCaptureRegion(
	region image.Rectangle,
	activeScreen func() (image.Rectangle, error),
) (image.Rectangle, error) {
	if region != (image.Rectangle{}) {
		canonical := region.Canon()
		if canonical.Empty() {
			return image.Rectangle{}, derrors.Newf(
				derrors.CodeActionFailed,
				"the requested capture region %v is empty; refusing to widen it to the whole screen",
				region,
			)
		}

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
