//go:build linux

package linux

import (
	"context"
	"errors"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The policy that turns a stored restore token into a session without a
// consent prompt, and decides what to do when the token no longer works. It is
// kept apart from the D-Bus handshakes, which need a live KDE
// xdg-desktop-portal, so it can be tested without I/O. RemoteDesktop (input)
// and ScreenCast (capture) are separate consents with separate tokens, but
// share this policy so the two cannot drift apart.

// errPortalRequestCanceled reports that the user declined the portal's consent
// dialog, the one refusal a second prompt would only repeat.
var errPortalRequestCanceled = errors.New("the portal consent request was canceled")

// errPortalFailureUnrelatedToGrant reports a handshake failure the stored
// restore token cannot be blamed for, so the token is kept.
var errPortalFailureUnrelatedToGrant = errors.New(
	"the portal handshake failed for a reason unrelated to the stored grant",
)

// unrelatedError marks such a failure. It is transparent, keeping the wrapped
// error's message and derrors code.
type unrelatedError struct {
	err error
}

func (e unrelatedError) Error() string { return e.err.Error() }

func (e unrelatedError) Unwrap() error { return e.err }

func (e unrelatedError) Is(target error) bool { return target == errPortalFailureUnrelatedToGrant }

// unrelatedToStoredGrant marks err as saying nothing about the stored token.
// That covers everything before the token is presented (bus dial, reply
// subscription, CreateSession) and everything after Start, when the grant is
// complete. A failure in SelectDevices/SelectSources or Start, the calls that
// carry and consume the token, is blamed on the token, erring toward one extra
// prompt rather than a daemon that can never establish a session again.
func unrelatedToStoredGrant(err error) error {
	return unrelatedError{err: err}
}

// errPortalGrantYieldedNoDevices reports a granted session on which no input
// device came up. The grant was fine, so another handshake would get the same
// answer at the cost of another dialog.
var errPortalGrantYieldedNoDevices = errors.New(
	"the granted remote desktop session brought up no input device",
)

// errNoScreenCastConsent reports that no ScreenCast grant is held or
// restorable on a path that may not prompt, such as a capture before the user
// ever approved sharing. The prompt belongs to the permission request.
var errNoScreenCastConsent = errors.New(
	"no screen-sharing consent has been granted to Neru yet",
)

// portalGrant is one established RemoteDesktop grant: the EIS socket libei
// injects through, and the token that restores the same grant next time.
type portalGrant struct {
	// eisFD is the file descriptor of the EIS socket. Ownership passes to the
	// caller, which hands it to libei.
	eisFD int
	// restoreToken restores this grant on a later start, empty when the portal
	// declined to persist it. It is a credential (portal_restore_token.go).
	restoreToken string
	// close ends the portal session and its D-Bus connection. It leaves the EIS
	// fd alone, because libei owns it.
	close func()
}

// grantRestoreToken implements portalRestorableGrant.
func (g portalGrant) grantRestoreToken() string { return g.restoreToken }

// portalRestorableGrant is all the restore policy needs from a grant, the
// token that restores it next time.
type portalRestorableGrant interface {
	grantRestoreToken() string
}

// establishPortalGrant opens a portal grant, reusing the stored one when there
// is one. It tries the portal at most twice. The first attempt presents the
// stored token, and if that is refused the token is dropped and one fresh
// attempt prompts. No third attempt or backoff, because a portal refusing a
// prompt the user just answered will refuse again and would stall startup.
//
// The returned token is always stored, because using a token invalidates it.
// Keeping the presented one would restore once and prompt on every later start.
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

	// With nothing presented, or a failure that was not the token's fault, a
	// second attempt would only repeat the first.
	if stored == "" || !storedGrantPresumedDead(err) {
		return none, err
	}

	// The token is presumed revoked or expired. Drop it so the next start does
	// not replay it, and prompt exactly once.
	_ = store.clear()

	grant, err = open(ctx, "")
	if err != nil {
		return none, err
	}

	persistRestoreToken(store, grant.grantRestoreToken())

	return grant, nil
}

// restorePortalGrant opens a portal grant without ever showing a consent
// dialog, presenting the stored token and accepting the portal's answer. The
// capture path uses it, because a dialog while the user waits for hints is a
// stall at the worst moment. With nothing stored, the caller is told to ask for
// consent through SystemPort.RequestScreenCapturePermission instead.
//
// A refused token is still dropped, so later captures and the next permission
// request do not replay a credential that cannot work.
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

// storedGrantPresumedDead reports whether a failed restore should be blamed on
// the presented token, which decides whether it is thrown away and the user
// asked again. Only a refusal by one of the two token-carrying calls counts.
// These do not, because blaming them would discard a working grant:
//
//   - a step that never held the token (see unrelatedToStoredGrant).
//   - the user canceling the dialog, which a second prompt would only repeat.
//   - a timeout or unreachable session bus, the portal not answering rather
//     than saying no.
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
// by an attempt that shows a consent dialog. A canceled request was already
// answered, and a grant with no input device was never a consent problem, so a
// dialog buys nothing for either. It is a predicate because the fallback it
// guards lives in the cgo-only file, where no test reaches it.
func promptingFallbackAllowed(err error) bool {
	return !errors.Is(err, errPortalRequestCanceled) &&
		!errors.Is(err, errPortalGrantYieldedNoDevices)
}

// persistRestoreToken stores the returned token, or clears the store when there
// is none, because a spent token costs a refused attempt before the prompt. A
// failed write is swallowed. The session is live and the cost is one prompt on
// the next start, better than refusing a grant the user just approved over a
// read-only state directory.
func persistRestoreToken(store restoreTokenStore, token string) {
	if token == "" {
		_ = store.clear()

		return
	}

	_ = store.save(token)
}
