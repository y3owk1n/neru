//go:build linux

package linux

import (
	"context"

	"github.com/godbus/dbus/v5"
)

// The org.freedesktop.portal.RemoteDesktop handshake over the session bus.
// portal_bus.go owns the transport (dial, Request/Response, teardown).
//
// liboeffis wraps this handshake but exposes no restore token
// (oeffis_create_session takes only a device mask), so every daemon start would
// show KDE's "Remote Control" dialog. The token lives in SelectDevices and
// Start, so this file owns the four D-Bus calls. Everything past the EIS
// socket is still libei's.

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
// socket libei injects through. A non-empty restoreToken asks the portal to
// restore that grant without a dialog. The spec says an unrestorable token is
// ignored and the user prompted, and the caller handles backends that refuse
// instead. Every step, including the bus dial, is bounded by ctx (see
// dialPortalBus).
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
	// Everything through CreateSession runs before the token is presented, so
	// these failures cannot be blamed on the stored grant.
	requester, release, err := newPortalRequester(ctx, conn, portalNameRemoteDesktop)
	if err != nil {
		return portalGrant{}, unrelatedToStoredGrant(err)
	}

	defer release()

	session, err := createPortalSession(ctx, requester, remoteDesktopCreateSession)
	if err != nil {
		return portalGrant{}, unrelatedToStoredGrant(err)
	}

	// SelectDevices carries the token and Start consumes it, so their errors go
	// back unmarked as the one signal that the stored grant is dead.
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

	// Past Start the grant is complete, so this failure is not the token's.
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

// startRemoteDesktopSession performs step 3, the call that shows the consent
// dialog when there is nothing to restore, and returns the restore token for
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

	// A grant without persistence returns no token. That is supported, and the
	// next start prompts again.
	token, _ := results[portalRestoreTokenKey].Value().(string)

	return token, nil
}

// connectToEIS performs step 4, a plain method call that returns the socket
// directly rather than via a Request. The caller owns the returned fd.
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

// portalSelectDevicesOptions builds the SelectDevices options. portalRequester
// adds the handle token. The restore token is omitted rather than sent empty,
// so a backend that validates it does not refuse the ordinary first run.
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

// Without this check a signature drift would silently take this grant off
// establishPortalGrant and prompt on every start.
var _ portalRestorableGrant = portalGrant{}
