//go:build linux

package linux

import (
	"context"
	"errors"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The policy that turns a stored restore token into a session without a
// consent prompt, and decides what to do when the stored token no longer
// works.
//
// It is separated from the D-Bus handshakes beside it because a handshake is
// the part no test on this repository's development platform can drive — it
// needs a live xdg-desktop-portal with a KDE backend — while "which token do we
// present, and how many times do we ask" is a decision with three inputs and
// no I/O.
//
// It is written once for both grants KDE needs. RemoteDesktop (input) and
// ScreenCast (capture) are separate sessions with separate tokens, because they
// are separate consents, but the question they ask of a stored token is
// identical and answering it twice is how the two would drift apart.

// errPortalRequestCanceled reports that the user answered the portal's consent
// dialog with "no". It is distinguished from every other failure because it is
// the one refusal that a second prompt would simply repeat back at them.
var errPortalRequestCanceled = errors.New("the portal consent request was canceled")

// errPortalFailureUnrelatedToGrant reports a handshake failure the stored
// restore token cannot be blamed for, so the token is kept.
var errPortalFailureUnrelatedToGrant = errors.New(
	"the portal handshake failed for a reason unrelated to the stored grant",
)

// unrelatedError marks such a failure. It is transparent on purpose: the
// message and the derrors code are the wrapped error's, so the only new thing
// is the extra question this type answers.
type unrelatedError struct {
	err error
}

func (e unrelatedError) Error() string { return e.err.Error() }

func (e unrelatedError) Unwrap() error { return e.err }

func (e unrelatedError) Is(target error) bool { return target == errPortalFailureUnrelatedToGrant }

// unrelatedToStoredGrant marks err as a failure that says nothing about the
// stored token. Two stretches of a handshake qualify, at either end of it:
//
//   - everything before the call that presents the token — the bus dial, the
//     reply subscription, CreateSession — because the token has not been sent
//     yet, so nothing has been shown it to refuse;
//   - everything after Start, because by then the grant is complete: the portal
//     has started the session and handed back a fresh token, and a socket or a
//     stream it will not produce is not a credential problem.
//
// What is left in between is the pair of calls that carry the token and consume
// it — SelectDevices/SelectSources and Start. A failure there is treated as the
// token's fault, deliberately erring toward one extra consent prompt rather
// than toward a daemon that can never establish a session again.
func unrelatedToStoredGrant(err error) error {
	return unrelatedError{err: err}
}

// errPortalGrantYieldedNoDevices reports that the portal handed over a session
// but no input device ever came up on it. The grant itself was fine, so a
// second handshake through any other path would ask the same portal for the
// same devices and get the same answer — at the cost of another dialog.
var errPortalGrantYieldedNoDevices = errors.New(
	"the granted remote desktop session brought up no input device",
)

// errNoScreenCastConsent reports that no ScreenCast grant is held and none can
// be restored, on a path that is not allowed to put a dialog on screen. It is
// the answer a capture gives when the user has never approved screen sharing:
// the prompt belongs to the permission request, not to a hint refresh.
var errNoScreenCastConsent = errors.New(
	"no screen-sharing consent has been granted to Neru yet",
)

// portalGrant is one established RemoteDesktop grant: the EIS socket libei
// injects through, and the token that restores the same grant next time.
type portalGrant struct {
	// eisFD is the file descriptor of the EIS socket. Ownership passes to the
	// caller, which hands it to libei.
	eisFD int
	// restoreToken restores this grant on a later start, and is empty when the
	// portal declined to persist it. It is a credential: see
	// portal_restore_token.go for what that means in this package.
	restoreToken string
	// close ends the portal session and releases the D-Bus connection holding
	// it. The EIS fd is not closed here — libei owns it once it is handed over.
	close func()
}

// grantRestoreToken reports the token this grant handed back. It is what makes
// portalGrant usable by the shared restore policy below.
func (g portalGrant) grantRestoreToken() string { return g.restoreToken }

// portalRestorableGrant is the one thing the restore policy needs to know about
// a grant: which token restores it next time. Everything else a grant carries —
// an EIS socket, a set of PipeWire nodes — belongs to the interface that
// negotiated it and never reaches this file.
type portalRestorableGrant interface {
	grantRestoreToken() string
}

// establishPortalGrant opens a portal grant, reusing the stored one when there
// is one to reuse.
//
// It attempts the portal at most twice, ever. The first attempt presents
// whatever token is stored; if that is refused the token is dropped and one
// fresh attempt prompts the user. There is no third attempt and no backoff:
// a portal that refuses a prompt the user just answered will refuse it again,
// and looping on it would sit in front of the daemon's startup.
//
// Whatever token the grant hands back is stored, because a restore token is
// invalidated by the use that consumes it — keeping the presented one would
// restore exactly once and prompt on every start after that.
func establishPortalGrant[T portalRestorableGrant](
	ctx context.Context,
	store restoreTokenStore,
	open func(ctx context.Context, restoreToken string) (T, error),
) (T, error) {
	var none T

	stored := store.load()

	grant, err := open(ctx, stored)
	if err == nil {
		persistRestoreToken(store, grant.grantRestoreToken())

		return grant, nil
	}

	// Nothing was presented, so no stored grant can be blamed and a second
	// attempt would repeat the first exactly. Nor when the failure was not the
	// token's fault — see storedGrantPresumedDead.
	if stored == "" || !storedGrantPresumedDead(err) {
		return none, err
	}

	// The stored token is presumed revoked or expired. Drop it, so the next
	// start begins clean rather than replaying a credential that failed, and
	// prompt exactly once.
	_ = store.clear()

	grant, err = open(ctx, "")
	if err != nil {
		return none, err
	}

	persistRestoreToken(store, grant.grantRestoreToken())

	return grant, nil
}

// restorePortalGrant opens a portal grant without ever putting a consent dialog
// on screen: it presents the stored token and accepts whatever the portal makes
// of it.
//
// This is the variant the capture path uses. A capture runs while the user is
// waiting for hints to appear, so a dialog there is both a several-second stall
// and a question asked at the worst possible moment; the prompt belongs to
// SystemPort.RequestScreenCapturePermission, which runs off the mode handler's
// lock with a budget sized for a human. With nothing stored there is nothing to
// present, so the caller is told to ask for consent rather than sent into a
// handshake whose only possible outcome is a dialog.
//
// A token the portal refuses is still dropped. Leaving it would have every
// later capture replay a credential that cannot work, and the next permission
// request would present it before falling back to a prompt — one refused round
// trip in front of a dialog the user is going to see anyway.
func restorePortalGrant[T portalRestorableGrant](
	ctx context.Context,
	store restoreTokenStore,
	open func(ctx context.Context, restoreToken string) (T, error),
) (T, error) {
	var none T

	stored := store.load()
	if stored == "" {
		return none, derrors.Wrap(
			errNoScreenCastConsent,
			derrors.CodeNotSupported,
			"the screen cannot be captured until screen sharing is approved once",
		)
	}

	grant, err := open(ctx, stored)
	if err != nil {
		if storedGrantPresumedDead(err) {
			_ = store.clear()
		}

		return none, err
	}

	persistRestoreToken(store, grant.grantRestoreToken())

	return grant, nil
}

// storedGrantPresumedDead reports whether a failed restore attempt should be
// blamed on the token it presented — which is what decides whether the token is
// thrown away and the user asked afresh.
//
// Only a refusal by one of the two calls that carry the token counts. Three
// failures do not, and treating any of them as the token's fault would throw
// away a grant that still works and buy a consent prompt on the next start
// with it:
//
//   - a step that never had the token in its hands, at either end of the
//     handshake — see unrelatedToStoredGrant;
//   - the user canceled the dialog, which says nothing about the token and
//     which a second prompt would only ask again;
//   - the call ran out of time or the session bus was unreachable, which is the
//     portal not answering rather than the portal saying no.
func storedGrantPresumedDead(err error) bool {
	switch {
	case errors.Is(err, errPortalFailureUnrelatedToGrant):
		return false
	case errors.Is(err, errPortalRequestCanceled):
		return false
	case derrors.IsCode(err, derrors.CodeTimeout), derrors.IsNotSupported(err):
		return false
	default:
		return true
	}
}

// promptingFallbackAllowed reports whether a failed handshake may be followed
// by a second attempt that would put a consent dialog on screen.
//
// Two failures forbid it, and both for the same reason — the dialog would buy
// nothing. A request the user canceled has already been answered, and asking
// again is the consent fatigue this whole file exists to remove; a grant that
// came up with no input device was not a consent problem at all, and a second
// handshake would ask the same portal for the same devices.
//
// It is a predicate rather than an inline check because the fallback it guards
// lives in the cgo-only file, where no test can reach it.
func promptingFallbackAllowed(err error) bool {
	return !errors.Is(err, errPortalRequestCanceled) &&
		!errors.Is(err, errPortalGrantYieldedNoDevices)
}

// persistRestoreToken records the token this grant handed back, or clears the
// store when it handed back none — a portal that granted the session without
// persisting it leaves the presented token spent, and a spent token is worse
// than no token because it costs a refused attempt before the prompt.
//
// A store that cannot be written is not an error the caller should see: the
// session in hand is live and usable, and the cost of the failure is one
// consent prompt on the next start, which is the behavior this change replaces
// rather than a new failure. Refusing a grant the user just approved because a
// state directory is read-only would be the worse trade.
func persistRestoreToken(store restoreTokenStore, token string) {
	if token == "" {
		_ = store.clear()

		return
	}

	_ = store.save(token)
}
