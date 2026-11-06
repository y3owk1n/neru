//go:build linux

package linux

import (
	"context"
	"image"
	"time"

	"github.com/y3owk1n/neru/internal/derrors"
)

// screenCaptureTimeoutMS bounds one Wayland capture exchange, so a compositor
// that stops answering fails the capture instead of wedging the caller. A real
// 4K grab takes tens of milliseconds.
const screenCaptureTimeoutMS = 2000

// captureBudgetMS is how long a native capture may wait, whatever is left of
// the caller's budget capped at screenCaptureTimeoutMS. The floor is 1ms because
// C reads a non-positive timeout as "use the default", which would bolt two
// seconds onto a budget that had already run out.
func captureBudgetMS(ctx context.Context) int {
	budget := screenCaptureTimeoutMS

	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline {
		budget = min(budget, int(time.Until(deadline).Milliseconds()))
	}

	return max(budget, 1)
}

// captureCanceled reports ctx ending as a canceled capture. A capture checks it
// again once the overlay is off, because waiting for the overlay's render lock
// can outlast the caller's deadline.
func captureCanceled(ctx context.Context) error {
	err := ctx.Err()
	if err != nil {
		return derrors.Wrap(err, derrors.CodeContextCanceled, "screen capture canceled")
	}

	return nil
}

// captureStatus mirrors the NERU_CAPTURE_* codes in screencapture.h so the
// status-to-sentence mapping is testable without cgo. screencapture_cgo.go
// fails to compile if the two lists disagree.
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
// what names the display server, so a user knows which one refused. Statuses
// meaning "this display server will never do this" are CodeNotSupported, so
// callers degrade instead of retrying.
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

// Names captureError uses for the display server that refused. KDE gets the
// portal label because its backend is the portal, and blaming "this Wayland
// compositor" would send someone looking at KWin for a failure that belongs to
// xdg-desktop-portal or PipeWire.
const (
	captureLabelXServer    = "the X server"
	captureLabelCompositor = "this Wayland compositor"
	captureLabelPortal     = "the screen-sharing portal"
)

// CaptureScreenRegion captures the pixels inside region and returns them as an
// RGBA image. region is in Neru's shared coordinate space. The zero rectangle
// means the active screen (the one holding the cursor, since Wayland exposes
// no primary display), resolved here so native backends only see a concrete
// rectangle. Callers normally pass the focused window, because reading a whole
// 4K display to examine one window is too slow.
//
// The result covers exactly region, with bounds starting at (0, 0). A region
// that leaves the screen, or spans two monitors on Wayland, fails rather than
// coming back clipped, because a clipped frame cannot say where its top-left
// is. On a scaled Wayland output the image is in physical pixels, so it can be
// larger than region by the scale factor, as on Retina.
//
// backend is NewSystemAdapter's label. X11 reads the root window, wlroots uses
// wlr-screencopy, KDE reads a PipeWire stream off the portal's ScreenCast
// session, and anything else reports CodeNotSupported naming itself.
//
// This runs under the mode handler's lock and reads under the overlay's render
// lock, so the KDE and wlroots backends get what is left of ctx, capped at
// screenCaptureTimeoutMS. X11's single XGetImage reads no timeout and cannot be
// canceled, so there ctx is only checked. Every backend checks ctx again once
// the overlay is off. A Go deadline is only worth threading where the callee
// reads it (internal/app/modes/AGENTS.md).
//
// Privacy: the returned image is the only copy that outlives this call, and
// native buffers are wiped before release. Callers must never log it, derive
// log text from it, write it to disk, or hold it past the detection that asked
// for it.
func CaptureScreenRegion(
	ctx context.Context,
	backend string,
	region image.Rectangle,
) (*image.RGBA, error) {
	err := ctx.Err()
	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeContextCanceled, "screen capture canceled")
	}

	if backend == backendX11 {
		resolved, err := resolveCaptureRegion(region, x11ActiveScreenBounds)
		if err != nil {
			return nil, err
		}

		return withOwnWindowsHidden(func() (*image.RGBA, error) {
			err := captureCanceled(ctx)
			if err != nil {
				return nil, err
			}

			return x11CaptureRegion(resolved)
		})
	}

	// KDE and COSMIC are checked before the wlroots family. They share its
	// wl_output client stack, so the active screen resolves the same way, but
	// they advertise no screencopy and capture through the portal instead.
	if backendCapturesViaPortal(backend) {
		resolved, err := resolveCaptureRegion(region, wlrootsScreenBounds)
		if err != nil {
			return nil, err
		}

		return kdeCaptureRegion(ctx, resolved)
	}

	if backendUsesWlrClientStack(backend) {
		resolved, err := resolveCaptureRegion(region, wlrootsScreenBounds)
		if err != nil {
			return nil, err
		}

		return withOwnWindowsHidden(func() (*image.RGBA, error) {
			err := captureCanceled(ctx)
			if err != nil {
				return nil, err
			}

			return wlrootsCaptureRegion(resolved, captureBudgetMS(ctx))
		})
	}

	return nil, derrors.New(derrors.CodeNotSupported, unsupportedCaptureBackend(backend))
}

// ScreenCaptureSupported reports whether backend has a capture path at all,
// without capturing, for callers such as the vision health check that must not
// read the screen to learn which display server runs. The error is the
// CodeNotSupported sentence a capture would produce. It answers on the label
// alone. Whether KDE's consent gate is open is
// SystemPort.CheckScreenCapturePermission's question, and asking here would
// make a health check dial the portal.
func ScreenCaptureSupported(backend string) error {
	if backend == backendX11 || backendUsesWlrClientStack(backend) {
		return nil
	}

	return derrors.New(derrors.CodeNotSupported, unsupportedCaptureBackend(backend))
}

// unsupportedCaptureBackend explains which display server has no capture path,
// so an unsupported compositor reads differently from no session at all.
func unsupportedCaptureBackend(backend string) string {
	// "unknown" is DetectLinuxBackend finding nothing, and "" is an adapter
	// built before detection ran. Both mean there is no session.
	if backend == "" || backend == backendUnknown {
		return "screen capture is unavailable: no display backend detected; " +
			"start a session under X11 or a Wayland compositor"
	}

	return "screen capture is not implemented on linux backend " + backend +
		"; supported backends are x11, wayland-wlroots, wayland-kde, wayland-cosmic and wayland-gnome"
}

// resolveCaptureRegion turns the caller's request into the rectangle handed to
// a native backend. Only the zero rectangle means the whole active screen. A
// requested region that came out degenerate is refused rather than widened,
// because a caller asking for a window must never get the whole screen.
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
