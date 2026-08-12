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
// It is separated from the D-Bus handshake below it because the handshake is
// the part no test on this repository's development platform can drive — it
// needs a live xdg-desktop-portal with a KDE backend — while "which token do we
// present, and how many times do we ask" is a decision with three inputs and
// no I/O.

// errPortalRequestCanceled reports that the user answered the portal's consent
// dialog with "no". It is distinguished from every other failure because it is
// the one refusal that a second prompt would simply repeat back at them.
var errPortalRequestCanceled = errors.New("the remote desktop consent request was canceled")

// errPortalGrantYieldedNoDevices reports that the portal handed over a session
// but no input device ever came up on it. The grant itself was fine, so a
// second handshake through any other path would ask the same portal for the
// same devices and get the same answer — at the cost of another dialog.
var errPortalGrantYieldedNoDevices = errors.New(
	"the granted remote desktop session brought up no input device",
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

// portalOpener establishes a RemoteDesktop grant, restoring the session that
// restoreToken names when it is not empty.
type portalOpener func(ctx context.Context, restoreToken string) (portalGrant, error)

// establishPortalGrant opens a RemoteDesktop grant, reusing the stored one when
// there is one to reuse.
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
func establishPortalGrant(
	ctx context.Context,
	store restoreTokenStore,
	open portalOpener,
) (portalGrant, error) {
	stored := store.load()

	grant, err := open(ctx, stored)
	if err == nil {
		persistRestoreToken(store, grant.restoreToken)

		return grant, nil
	}

	// Nothing was presented, so no stored grant can be blamed and a second
	// attempt would repeat the first exactly. Nor when the failure was not the
	// token's fault — see storedGrantPresumedDead.
	if stored == "" || !storedGrantPresumedDead(err) {
		return portalGrant{}, err
	}

	// The stored token is presumed revoked or expired. Drop it, so the next
	// start begins clean rather than replaying a credential that failed, and
	// prompt exactly once.
	_ = store.clear()

	grant, err = open(ctx, "")
	if err != nil {
		return portalGrant{}, err
	}

	persistRestoreToken(store, grant.restoreToken)

	return grant, nil
}

// storedGrantPresumedDead reports whether a failed restore attempt should be
// blamed on the token it presented — which is what decides whether the token is
// thrown away and the user asked afresh.
//
// Only failures the portal itself produced count. Two do not, and treating
// them as the token's fault would throw away a grant that still works and buy
// a consent prompt on the next start with it:
//
//   - the user canceled the dialog, which says nothing about the token and
//     which a second prompt would only ask again;
//   - the session bus was unreachable or the handshake ran out of time, where
//     nothing ever reached the portal to refuse anything.
func storedGrantPresumedDead(err error) bool {
	switch {
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
