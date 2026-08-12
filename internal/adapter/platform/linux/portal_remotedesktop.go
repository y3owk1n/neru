//go:build linux

package linux

import (
	"context"

	"github.com/godbus/dbus/v5"
)

// The org.freedesktop.portal.RemoteDesktop handshake, spoken directly over the
// session bus. The transport underneath it — the bus dial, the Request/Response
// dance, session teardown — is portal_bus.go.
//
// liboeffis already wraps this handshake, and the libei client used it until
// this file existed. It is not used for the session establishment path any
// more, for one reason: its API exposes no restore token
// (oeffis_create_session takes a device mask and nothing else), so every
// session it opens is a fresh grant and every daemon start shows KDE's "Remote
// Control" consent dialog. The token is negotiated in SelectDevices and handed
// back by Start, both of which liboeffis owns, so reusing a grant means owning
// the four D-Bus calls here. Everything past the EIS socket — device binding,
// event emission — is still libei's.

// The four RemoteDesktop methods one session needs, in the order they run.
const (
	remoteDesktopCreateSession = "org.freedesktop.portal.RemoteDesktop.CreateSession"
	remoteDesktopSelectDevices = "org.freedesktop.portal.RemoteDesktop.SelectDevices"
	remoteDesktopStart         = "org.freedesktop.portal.RemoteDesktop.Start"
	remoteDesktopConnectToEIS  = "org.freedesktop.portal.RemoteDesktop.ConnectToEIS"
)

// Device types from the RemoteDesktop specification's `types` bitmask.
const (
	portalDeviceKeyboard uint32 = 1
	portalDevicePointer  uint32 = 2
)

// openRemoteDesktopSession runs the RemoteDesktop handshake and returns the EIS
// socket libei injects through.
//
// When restoreToken is not empty the portal is asked to restore the grant it
// names, which is what keeps the consent dialog off the screen on a restart. A
// token the portal will not accept is not an error here: the specification says
// an unrestorable token is ignored and the user prompted normally, and the
// caller handles the backends that refuse instead.
//
// Every step is bounded by ctx, including the bus dial, which godbus cannot
// cancel on its own — see dialPortalBus.
func openRemoteDesktopSession(ctx context.Context, restoreToken string) (portalGrant, error) {
	conn, err := dialPortalBus(ctx)
	if err != nil {
		return portalGrant{}, unrelatedToStoredGrant(err)
	}

	grant, err := negotiateRemoteDesktop(ctx, conn, restoreToken)
	if err != nil {
		_ = conn.Close()

		return portalGrant{}, err
	}

	return grant, nil
}

// negotiateRemoteDesktop runs CreateSession, SelectDevices, Start and
// ConnectToEIS on conn, in that order, and returns the grant they produced.
func negotiateRemoteDesktop(
	ctx context.Context,
	conn *dbus.Conn,
	restoreToken string,
) (portalGrant, error) {
	// Everything up to and including CreateSession runs before SelectDevices
	// presents the restore token, so each failure below is marked as one the
	// stored grant cannot be blamed for.
	requester, release, err := newPortalRequester(ctx, conn, portalNameRemoteDesktop)
	if err != nil {
		return portalGrant{}, unrelatedToStoredGrant(err)
	}

	defer release()

	session, err := createPortalSession(ctx, requester, remoteDesktopCreateSession)
	if err != nil {
		return portalGrant{}, unrelatedToStoredGrant(err)
	}

	// The token is in play across the next two calls: SelectDevices carries it
	// and Start consumes it, so a refusal from either is the one signal there is
	// that the stored grant is no longer good. Their errors go back unmarked.
	err = selectRemoteDesktopDevices(ctx, requester, session, restoreToken)
	if err != nil {
		closePortalSession(conn, session)

		return portalGrant{}, err
	}

	newToken, err := startRemoteDesktopSession(ctx, requester, session)
	if err != nil {
		closePortalSession(conn, session)

		return portalGrant{}, err
	}

	// Past Start the grant is complete and a fresh token is already in hand, so
	// a socket the portal will not produce says nothing about the stored one.
	eisFD, err := connectToEIS(ctx, requester, session)
	if err != nil {
		closePortalSession(conn, session)

		return portalGrant{}, unrelatedToStoredGrant(err)
	}

	return portalGrant{
		eisFD:        eisFD,
		restoreToken: newToken,
		close: func() {
			closePortalSession(conn, session)

			_ = conn.Close()
		},
	}, nil
}

// selectRemoteDesktopDevices performs step 2, which is where the restore token
// is presented and where persistence is asked for.
func selectRemoteDesktopDevices(
	ctx context.Context,
	requester *portalRequester,
	session dbus.ObjectPath,
	restoreToken string,
) error {
	_, err := requester.call(ctx, remoteDesktopSelectDevices,
		portalSelectDevicesOptions(restoreToken), session)

	return err
}

// startRemoteDesktopSession performs step 3 — the call that shows the consent
// dialog when there is nothing to restore — and returns the restore token for
// next time, which is empty when the portal declined to persist the grant.
func startRemoteDesktopSession(
	ctx context.Context,
	requester *portalRequester,
	session dbus.ObjectPath,
) (string, error) {
	results, err := requester.call(ctx, remoteDesktopStart,
		map[string]dbus.Variant{},
		session, portalNoParentWindow)
	if err != nil {
		return "", err
	}

	// A portal that grants the session without persisting it returns no token.
	// That is a supported answer, not a failure: input works, and the next
	// start prompts again.
	token, _ := results[portalRestoreTokenKey].Value().(string)

	return token, nil
}

// connectToEIS performs step 4. Unlike the three before it this is a plain
// method call — it answers with the socket directly instead of through a
// Request — and the file descriptor it returns belongs to the caller.
func connectToEIS(
	ctx context.Context,
	requester *portalRequester,
	session dbus.ObjectPath,
) (int, error) {
	var socket dbus.UnixFD

	err := requester.portal.CallWithContext(
		ctx,
		remoteDesktopConnectToEIS,
		noPortalCallFlags,
		session,
		map[string]dbus.Variant{},
	).Store(&socket)
	if err != nil {
		return 0, portalCallFailed(
			err,
			"the RemoteDesktop portal granted a session but would not hand over an "+
				"EIS socket to inject through",
		)
	}

	return int(socket), nil
}

// portalSelectDevicesOptions builds the options for SelectDevices: which
// devices are wanted, how long the grant should last, and which grant to
// restore. The handle token is added by portalRequester.call, which owns it.
//
// The restore token is omitted rather than sent empty when there is nothing to
// restore, so a portal backend that validates the key it was handed does not
// refuse the ordinary first run.
func portalSelectDevicesOptions(restoreToken string) map[string]dbus.Variant {
	options := map[string]dbus.Variant{
		"types":        dbus.MakeVariant(portalDeviceKeyboard | portalDevicePointer),
		"persist_mode": dbus.MakeVariant(portalPersistUntilRevoked),
	}

	if restoreToken != "" {
		options[portalRestoreTokenKey] = dbus.MakeVariant(restoreToken)
	}

	return options
}

// Compile-time proof that the RemoteDesktop grant satisfies the shared restore
// policy's contract. Without it a signature drift would silently take this
// grant off establishPortalGrant and leave the consent prompt on every start.
var _ portalRestorableGrant = portalGrant{}
