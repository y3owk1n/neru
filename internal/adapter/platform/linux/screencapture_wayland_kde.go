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

// KDE Plasma's screen-capture backend: the portal's ScreenCast session, read
// over PipeWire.
//
// KWin implements no screencopy protocol Neru can use, so unlike X11 and the
// wlroots family this backend is behind a consent gate. The gate is paid once —
// the grant is persisted with a restore token and restored silently on every
// later start — and it is paid where a person can answer it, through
// SystemPort.RequestScreenCapturePermission, which the mode handler runs off
// its lock with a budget sized for a human reading a dialog. A capture never
// raises one: the handshake it may run presents the stored token and nothing
// else (restorePortalGrant).
//
// The session is established once and reused. What is *not* reused is the
// PipeWire connection: one is opened per capture and drained to a single frame,
// because a stream held open between captures would have KWin pushing the
// user's screen at Neru continuously for the sake of the handful of frames it
// is actually asked for.

// screenCastConsentTimeout bounds the establishment that is allowed to prompt.
// It is long for the same reason libeiWarmupTimeoutMs is: the dialog appears
// while no overlay is up, and the user needs a comfortable window to find and
// approve it. Nothing holds the mode handler's lock across it.
const screenCastConsentTimeout = 120 * time.Second

// screenCastRestoreTimeout bounds an establishment that may not prompt. Three
// portal round trips against a stored token is a fraction of a second on a
// healthy session; this is the point at which a portal that is not answering
// stops being worth waiting for.
const screenCastRestoreTimeout = 5 * time.Second

// screenCastRemoteTimeout bounds the OpenPipeWireRemote call that opens the
// connection a frame arrives over. The wait for the frame itself is bounded
// separately, by screenCaptureTimeoutMS inside the native reader, so a capture
// costs at most the two together — the same budget the wlroots backend gives a
// screencopy exchange, twice, for a backend that needs one round trip more. A
// portal or compositor that stops answering must surface as a failed capture
// rather than a wedged hint refresh either way.
const screenCastRemoteTimeout = screenCaptureTimeoutMS * time.Millisecond

// pipewireCaptureError maps a native capture status onto the shared error
// vocabulary, for the three statuses whose wlroots sentence would be a lie
// here.
//
// captureError says "no display server", "does not implement
// wlr-screencopy-unstable-v1" and "offered a pixel format Neru cannot read",
// and reports all three as CodeNotSupported — right for a compositor Neru talks
// to directly, where each of them is a permanent property of the display
// server. On this backend the compositor was already reached; it granted the
// session. What can fail instead is the PipeWire connection the portal handed
// over, and a buffer whose stride, offset or dimensions this reader will not
// touch is a live failure of one frame rather than a statement about KWin — so
// they are CodeActionFailed, and callers retry instead of degrading for good.
// Everything else a capture can fail with means the same thing on all three
// backends.
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
			"PipeWire refused the video stream KDE Plasma's screen-sharing session "+
				"named",
		)
	case captureStatusFormat:
		return derrors.New(
			derrors.CodeActionFailed,
			"the PipeWire stream delivered a frame KDE Plasma's screen-sharing "+
				"session described in a layout Neru cannot read",
		)
	case captureStatusOK,
		captureStatusNoOutput,
		captureStatusRegion,
		captureStatusAlloc,
		captureStatusFailed,
		captureStatusTimeout:
		return captureError(status, captureLabelKDE)
	default:
		return captureError(status, captureLabelKDE)
	}
}

// screenCastState owns the ScreenCast grant this backend captures through.
//
// Its mutex sits below the mode handler's in the lock order
// (internal/app/modes/AGENTS.md): a capture and a permission check both reach
// it from under `h.mu`, and both take it with TryLock, because the consent path
// holds it for as long as a user takes to answer a dialog. Nothing holding this
// mutex ever calls back into the handler, so the edge is one-way.
type screenCastState struct {
	mu    sync.Mutex
	grant screenCastGrant
	// ready is written only under mu and read without it by
	// screenCastConsentHeld, which runs on the activation path and must answer
	// wait-free. A TryLock there would have answered "no consent" whenever
	// another goroutine held the mutex — including the instant after the
	// consent it is being asked about was granted, which is exactly when
	// ports.SystemPort requires the answer to be yes.
	ready atomic.Bool
}

var globalScreenCastState = &screenCastState{}

// kdeCaptureRegion reads region back through the granted ScreenCast session.
//
// region is in Neru's shared coordinate space, already resolved to a concrete
// rectangle by resolveCaptureRegion. It is honored rather than approximated:
// the monitor that wholly contains it is the one whose node is read, and a
// region that leaves the screen or spans two monitors fails.
//
// On a scaled output PipeWire delivers physical pixels, so what comes back can
// be larger than the logical region by the output's scale factor — the same
// thing the wlroots backend and a Retina capture on macOS do.
//
// ctx is the caller's budget and every step here is derived from it. Unlike the
// other two backends this one is three round trips deep, so its own timeouts
// are ceilings on each step rather than the whole cost — a hint activation
// holds the mode handler's lock across this call, and what it may cost has to
// be the caller's decision.
func kdeCaptureRegion(ctx context.Context, region image.Rectangle) (*image.RGBA, error) {
	if !nativeBackendsCompiledIn {
		return nil, derrors.New(
			derrors.CodeNotSupported,
			"KDE screen capture requires CGO-enabled Linux builds: frames arrive "+
				"over PipeWire, which is a native library",
		)
	}

	state := globalScreenCastState

	// TryLock rather than Lock: a capture reaches this from the hints activation
	// the mode handler holds its lock across, and the consent path below can
	// hold this mutex for as long as a user takes to answer a dialog. Waiting
	// there would freeze key handling for the same two minutes.
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

	stream, local, err := selectScreenCastStream(state.grant.streams, region)
	if err != nil {
		return nil, err
	}

	remoteCtx, cancel := boundedByCaller(ctx, screenCastRemoteTimeout)
	defer cancel()

	remoteFD, err := state.grant.openPipeWire(remoteCtx)
	if err != nil {
		// A session that will not open a connection is a session that is gone —
		// revoked, or ended when the compositor restarted. Drop it so the next
		// permission check reports honestly and a fresh grant can be negotiated.
		state.resetLocked()

		return nil, err
	}

	return pipewireCaptureNode(
		remoteFD,
		stream.nodeID,
		local,
		stream.bounds.Dx(),
		stream.bounds.Dy(),
		frameBudgetMS(ctx),
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

// frameBudgetMS is how long the native reader may wait for a frame: whatever is
// left of the caller's budget, capped at the same ceiling the wlroots backend
// gives a screencopy exchange.
//
// The floor is one millisecond because the C side reads a non-positive timeout
// as "use the default". A caller with nothing left gets a wait that gives up
// immediately, which is what was wanted; the alternative would be a two-second
// wait bolted onto a budget that had already run out.
func frameBudgetMS(ctx context.Context) int {
	budget := screenCaptureTimeoutMS

	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline {
		budget = min(budget, int(time.Until(deadline).Milliseconds()))
	}

	return max(budget, 1)
}

// ensureLocked brings the grant up if it is not up already. The caller holds mu.
//
// allowPrompt is the whole difference between the two callers. The permission
// request may put KDE's source picker on screen; a capture may not, and
// presents the stored token or gives up.
//
// The handshake is bounded by ctx *and* by timeout, so a caller that cancels —
// a daemon shutting down while the picker is on screen — is not left waiting
// out the two minutes a human was given to answer it.
func (s *screenCastState) ensureLocked(
	ctx context.Context,
	timeout time.Duration,
	allowPrompt bool,
) error {
	if s.ready.Load() {
		return nil
	}

	store, err := newFileRestoreTokenStore(screenCastTokenFileName)
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

// screenCastSessionError phrases the failure for the user, carrying the
// portal's own reason as the cause. Nothing here can name the restore token:
// the token never reaches an error value, by construction
// (portal_restore_token.go).
func screenCastSessionError(cause error) error {
	// A build with no consent yet is not a broken session — it is the ordinary
	// state before the user has been asked, and the sentence that says so is
	// already the cause's.
	if derrors.IsNotSupported(cause) {
		return cause
	}

	return derrors.Wrap(
		cause,
		derrors.CodeActionFailed,
		"could not establish a screen-sharing session via the ScreenCast portal; "+
			"approve the one-time screen-sharing prompt (KDE Plasma reads the "+
			"screen through xdg-desktop-portal because KWin implements no "+
			"screencopy protocol)",
	)
}

// screenCastConsentHeld reports whether a capture can be taken right now
// without asking the user anything.
//
// It answers on the live session alone, and deliberately not on whether a
// restore token happens to be stored. A stored token means the user has
// consented before, not that the session is up — and the caller's next move on
// a false answer is RequestScreenCapturePermission, which restores that token
// silently. Reporting true on a token that has not been redeemed would instead
// send a full portal handshake down the capture path, which runs under the mode
// handler's lock.
//
// It never dials the bus, opens a session, reads a frame, or takes a lock: it
// is called on every vision-strategy activation, from under the mode handler's
// lock, and — because ports.SystemPort requires a Granted consent to be
// followed by a true answer here — it must not be able to say "no" merely
// because the goroutine that just granted the consent has not finished
// unwinding.
func screenCastConsentHeld() bool {
	return globalScreenCastState.ready.Load()
}

// requestScreenCastConsent establishes the ScreenCast grant, showing KDE's
// source picker when there is nothing stored to restore, and reports what the
// user decided.
//
// A failure that is not the user's answer is reported as a cancelation. The
// consent vocabulary has three words and none of them means "the portal broke";
// Canceled is the one that tells the caller to abandon the operation and keep
// running, which is what a daemon that cannot reach xdg-desktop-portal should
// do. Granted would be worse than useless: the caller re-checks the permission
// on Granted, would find it still unheld, and would drop the activation anyway
// after a warning about a consent it never received. The reason the handshake
// failed does not survive that mapping, which is why the caller logs the
// refusal rather than exiting the mode silently (app/modes/hints.go).
//
// It takes the mutex with TryLock, so a second activation while the picker is
// still on screen is answered immediately instead of parking a goroutine behind
// the first — a user pressing the hotkey again during a two-minute dialog would
// otherwise queue one waiter per press, each of which would go on to ask for
// its own fresh two minutes.
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
