//go:build linux

package linux

import (
	"context"
	"image"
	"sync"
	"sync/atomic"
	"time"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// KDE Plasma's screen-capture backend reads the portal's ScreenCast session
// over PipeWire. KWin has no usable screencopy protocol, so this backend sits
// behind a consent gate. The grant is paid once, persisted with a restore token
// and restored silently on later starts, and only through
// SystemPort.RequestScreenCapturePermission, which the mode handler runs off
// its lock. A capture never prompts. It presents the stored token or nothing
// (restorePortalGrant).
//
// The session is reused but the PipeWire connection is not. Each capture opens
// one and drains a single frame, so KWin never streams the user's screen at
// Neru between captures.

// screenCastConsentTimeout bounds the establishment allowed to prompt. It is
// long, like libeiWarmupTimeoutMs, so the user can find and approve the dialog.
// Nothing holds the mode handler's lock across it.
const screenCastConsentTimeout = 120 * time.Second

// screenCastRestoreTimeout bounds an establishment that may not prompt. A
// healthy portal answers its three round trips in a fraction of a second.
const screenCastRestoreTimeout = 5 * time.Second

// screenCastRemoteTimeout bounds OpenPipeWireRemote. The frame wait is bounded
// separately by screenCaptureTimeoutMS in the native reader, so a capture costs
// at most the two together.
const screenCastRemoteTimeout = screenCaptureTimeoutMS * time.Millisecond

// pipewireCaptureError rewords the three statuses whose wlroots sentence would
// be wrong here. captureError reports them as CodeNotSupported, a permanent
// property of a compositor Neru talks to directly. Here the compositor already
// granted the session, so they are failures of the portal's PipeWire connection
// or of one frame. They are CodeActionFailed, so callers retry instead of
// degrading for good.
func pipewireCaptureError(status captureStatus) error {
	switch status {
	case captureStatusNoDisplay:
		return derrors.New(
			derrors.CodeActionFailed,
			"the screen-sharing session named a PipeWire connection Neru could not "+
				"open; check that the pipewire service is running",
		)
	case captureStatusNoProtocol:
		return derrors.New(
			derrors.CodeActionFailed,
			"PipeWire refused the video stream the portal's screen-sharing session named",
		)
	case captureStatusFormat:
		return derrors.New(
			derrors.CodeActionFailed,
			"the PipeWire stream delivered a frame the portal's screen-sharing "+
				"session described in a layout Neru cannot read",
		)
	case captureStatusOK,
		captureStatusNoOutput,
		captureStatusRegion,
		captureStatusAlloc,
		captureStatusFailed,
		captureStatusTimeout:
		return captureError(status, captureLabelPortal)
	default:
		return captureError(status, captureLabelPortal)
	}
}

// screenCastState owns the ScreenCast grant. Its mutex sits below the mode
// handler's in the lock order (internal/app/modes/AGENTS.md). Callers under
// `h.mu` take it with TryLock, because the consent path holds it while a user
// answers a dialog. Nothing holding it ever calls back into the handler.
type screenCastState struct {
	mu    sync.Mutex
	grant screenCastGrant
	// ready is written only under mu and read without it by
	// screenCastConsentHeld, which must answer wait-free. A TryLock there would
	// answer "no consent" the instant after consent was granted, exactly when
	// ports.SystemPort requires yes.
	ready atomic.Bool
}

var globalScreenCastState = &screenCastState{}

// kdeCaptureRegion reads region, already resolved by resolveCaptureRegion,
// through the granted ScreenCast session. The node read is the monitor that
// wholly contains region, and a region that leaves the screen or spans two
// monitors fails. On a scaled output the result is in physical pixels.
//
// Every step derives from ctx, the caller's budget. This backend is three round
// trips deep, so its own timeouts cap each step rather than the whole cost, and
// a hint activation holds the mode handler's lock across the call.
func kdeCaptureRegion(ctx context.Context, region image.Rectangle) (*image.RGBA, error) {
	if !nativeBackendsCompiledIn {
		return nil, derrors.New(
			derrors.CodeNotSupported,
			"portal screen capture requires CGO-enabled Linux builds: frames arrive "+
				"over PipeWire, which is a native library",
		)
	}

	state := globalScreenCastState

	// TryLock, because the consent path can hold mu for a whole dialog, and
	// waiting here under the mode handler's lock would freeze key handling.
	if !state.mu.TryLock() {
		return nil, derrors.New(
			derrors.CodeActionFailed,
			"the screen-sharing session is still being approved; approve the "+
				"prompt, then try again",
		)
	}

	defer state.mu.Unlock()

	err := state.ensureLocked(ctx, screenCastRestoreTimeout, false)
	if err != nil {
		return nil, err
	}

	// Placement is per capture because outputs change while the grant lives. If
	// the compositor will not list outputs, streams stay at the origin, which is
	// right for a single monitor.
	outputs, _ := wlrootsScreenOutputs()

	stream, local, err := selectScreenCastStream(
		placeScreenCastStreams(state.grant.streams, outputs), region)
	if err != nil {
		return nil, err
	}

	remoteCtx, cancel := boundedByCaller(ctx, screenCastRemoteTimeout)
	defer cancel()

	remoteFD, err := state.grant.openPipeWire(remoteCtx)
	if err != nil {
		// A session that will not open a connection is gone (revoked, or the
		// compositor restarted). Drop it so the next permission check is honest.
		state.resetLocked()

		return nil, err
	}

	// Only the frame read hides the overlay. The round trips above leave it up.
	return pipewireCaptureNode(
		ctx,
		remoteFD,
		stream.nodeID,
		local,
		stream.bounds.Dx(),
		stream.bounds.Dy(),
	)
}

// boundedByCaller applies this package's ceiling for a step on top of whatever
// the caller already allowed, so a step can only ever shorten the budget.
func boundedByCaller(
	ctx context.Context,
	ceiling time.Duration,
) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, ceiling)
}

// ensureLocked brings the grant up if it is not already. The caller holds mu.
// Only the permission request sets allowPrompt and may show KDE's source
// picker. A capture presents the stored token or gives up. Bounding by ctx as
// well as timeout lets a shutdown abandon the picker without waiting it out.
func (s *screenCastState) ensureLocked(
	ctx context.Context,
	timeout time.Duration,
	allowPrompt bool,
) error {
	if s.ready.Load() {
		return nil
	}

	store, err := newFileRestoreTokenStore(screenCastTokenBase)
	if err != nil {
		return derrors.Wrap(
			err,
			derrors.CodeActionFailed,
			"could not resolve where to keep the ScreenCast portal grant",
		)
	}

	handshake, cancel := boundedByCaller(ctx, timeout)
	defer cancel()

	establish := restorePortalGrant[screenCastGrant]
	if allowPrompt {
		establish = establishPortalGrant[screenCastGrant]
	}

	grant, err := establish(handshake, store, openScreenCastSession)
	if err != nil {
		return screenCastSessionError(err)
	}

	s.grant = grant
	s.ready.Store(true)

	return nil
}

// resetLocked ends the grant and forgets it. The caller holds mu.
func (s *screenCastState) resetLocked() {
	if !s.ready.Load() {
		return
	}

	if s.grant.close != nil {
		s.grant.close()
	}

	s.grant = screenCastGrant{}
	s.ready.Store(false)
}

// screenCastSessionError phrases the failure for the user with the portal's
// reason as the cause. The restore token never reaches an error value, by
// construction (portal_restore_token.go).
func screenCastSessionError(cause error) error {
	// No consent yet is the ordinary state before the user is asked, not a
	// broken session, and the cause already says so.
	if derrors.IsNotSupported(cause) {
		return cause
	}

	return derrors.Wrap(
		cause,
		derrors.CodeActionFailed,
		"could not establish a screen-sharing session via the ScreenCast portal; "+
			"approve the one-time screen-sharing prompt (KDE Plasma and COSMIC "+
			"read the screen through xdg-desktop-portal because their compositors "+
			"implement no screencopy protocol)",
	)
}

// screenCastConsentHeld reports whether a capture can be taken now without
// asking the user anything. It answers on the live session, not on a stored
// restore token. On false the caller runs RequestScreenCapturePermission, which
// redeems the token silently, whereas true on an unredeemed token would send a
// full portal handshake down the capture path under the mode handler's lock.
//
// It runs on every vision activation under that lock, so it never dials, opens
// a session, or takes a lock. ports.SystemPort requires a Granted consent to be
// followed by true here, even before the granting goroutine has unwound.
func screenCastConsentHeld() bool {
	return globalScreenCastState.ready.Load()
}

// requestScreenCastConsent establishes the ScreenCast grant, showing KDE's
// source picker when nothing is stored to restore, and reports the decision.
//
// A failure that is not the user's answer reports Canceled, the only consent
// word that tells the caller to abandon the operation and keep running. Granted
// would make the caller re-check, find no consent, and drop the activation
// anyway. The reason is lost here, so the caller logs the refusal
// (app/modes/hints.go).
//
// TryLock answers a repeat activation during the picker at once, instead of
// queuing one waiter per hotkey press, each asking for its own two minutes.
func requestScreenCastConsent(ctx context.Context) ports.ScreenCaptureConsent {
	state := globalScreenCastState

	if !state.mu.TryLock() {
		return ports.ScreenCaptureCanceled
	}

	defer state.mu.Unlock()

	err := state.ensureLocked(ctx, screenCastConsentTimeout, true)
	if err != nil {
		return ports.ScreenCaptureCanceled
	}

	return ports.ScreenCaptureGranted
}
