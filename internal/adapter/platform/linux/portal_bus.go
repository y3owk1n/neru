//go:build linux

package linux

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The interface-independent half of xdg-desktop-portal: dialing the session
// bus, issuing calls answered by a Request's Response signal, and ending a
// session. KDE needs RemoteDesktop for input (portal_remotedesktop.go) and
// ScreenCast for capture (portal_screencast.go), because KWin implements
// neither blessed protocol. They are separate sessions on separate
// connections, so a user who only wants hints and grid is never asked to share
// their screen.
//
// The connections are private, not the shared dbus.SessionBus(). The portal
// ends a session when its connection closes, so that connection must be one
// nothing else needs.
const (
	// portalBusName is the well-known name xdg-desktop-portal owns.
	portalBusName = "org.freedesktop.portal.Desktop"
	// portalObjectPath is the object every portal interface is exported on.
	portalObjectPath = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	// portalRequestPathPrefix opens a Request's object path. The rest is our
	// unique name and chosen handle token, so a listener can be in place first.
	portalRequestPathPrefix = "/org/freedesktop/portal/desktop/request/"
	// portalRequestNamespace is the path namespace every Request lives under,
	// matched once so every call on one connection shares one match rule.
	portalRequestNamespace = dbus.ObjectPath("/org/freedesktop/portal/desktop/request")
	// portalRequestInterface carries the Response signal a Request answers with.
	portalRequestInterface = "org.freedesktop.portal.Request"
	// portalResponseSignal is that signal's member name.
	portalResponseSignal = "Response"
	// portalSessionCloseMethod ends a session explicitly, rather than leaving
	// the portal to notice the connection went away.
	portalSessionCloseMethod = "org.freedesktop.portal.Session.Close"
)

// portalPersistUntilRevoked is persist_mode 2. Mode 1 lasts only while the app
// runs, which is exactly the restart a restore token must survive.
const portalPersistUntilRevoked uint32 = 2

// Response codes from the portal's Request::Response signal.
const (
	portalResponseSuccess  uint32 = 0
	portalResponseCanceled uint32 = 1
	portalResponseEnded    uint32 = 2
)

// portalHandleTokenKey is the option every Request-answering call carries: the
// token the portal builds the Request's object path from.
const portalHandleTokenKey = "handle_token"

// portalSessionHandleTokenKey is the option CreateSession carries on every
// portal interface: the token the session's own object path is built from.
const portalSessionHandleTokenKey = "session_handle_token"

// portalRestoreTokenKey presents a stored grant and is the key Start returns
// the new token under, spelled once for both interfaces.
const portalRestoreTokenKey = "restore_token"

// portalNoParentWindow is Start's parent_window. Neru's overlays are
// layer-shell surfaces with no toplevel, so the portal places the dialog.
const portalNoParentWindow = ""

// noPortalCallFlags sends a plain method call. The reply is what carries the
// Request handle, so NO_REPLY_EXPECTED must not be set.
const noPortalCallFlags = 0

// portalSignalBuffer sizes the Response channel, one per call in the longest
// handshake with room to spare. godbus defers rather than blocks when full.
const portalSignalBuffer = 4

// portalHandleTokenBytes is the entropy behind one handle token, which only
// has to be unique among this connection's in-flight requests.
const portalHandleTokenBytes = 8

// Portal interface names used in errors, so a user knows which grant, and
// which settings row, a failure is about.
const (
	portalNameRemoteDesktop = "RemoteDesktop"
	portalNameScreenCast    = "ScreenCast"
)

// portalBusDialer is process-wide because what it caps is, one wedged session
// bus. Its connections are private, so the dialer closes one no caller still
// waits for.
var portalBusDialer = &sessionBusDialer{
	connect: connectPrivateSessionBus,
	policy:  refuseWhileDialing,
}

// dialPortalBus connects to the session bus without letting an unresponsive bus
// hold the caller. The mid-action input path reaches this holding the keyboard
// grab, and dbus.ConnectSessionBus cannot be canceled, so the dial runs on an
// abandonable goroutine (session_bus_dial.go).
//
// Like the capability probes, there is one outstanding attempt, never restarted
// while stuck, so a wedged bus refuses later callers at once instead of piling
// up a goroutine and half-open connection per keypress. Both grants share the
// cap on purpose, since the wedged bus is process-wide. A dial stops counting
// when ConnectSessionBus returns, not when the handshake ends, so an open
// consent dialog blocks nothing.
func dialPortalBus(ctx context.Context) (*dbus.Conn, error) {
	return portalBusConnection(ctx, portalBusDialer)
}

// portalBusConnection phrases one bounded dial for the user, saying which of
// three things went wrong: nothing to dial, a dial that outlasted the caller,
// or a dial the caller may not start.
func portalBusConnection(ctx context.Context, dialer *sessionBusDialer) (*dbus.Conn, error) {
	conn, err := dialer.dial(ctx)

	switch {
	case err == nil:
		return conn, nil
	case errors.Is(err, errSessionBusDialInFlight):
		return nil, derrors.New(
			derrors.CodeTimeout,
			"an earlier connection to the D-Bus session bus is still unanswered, "+
				"so xdg-desktop-portal cannot be reached yet",
		)
	case errors.Is(err, errSessionBusDialAbandoned):
		return nil, derrors.Wrap(
			ctx.Err(),
			derrors.CodeTimeout,
			"the D-Bus session bus did not accept a connection in time",
		)
	default:
		return nil, derrors.Wrap(
			err,
			derrors.CodeNotSupported,
			"no reachable D-Bus session bus, so xdg-desktop-portal cannot be "+
				"asked for a session",
		)
	}
}

// portalRequester issues portal calls whose answer arrives as a Request
// Response signal rather than as the method's own reply.
type portalRequester struct {
	conn    *dbus.Conn
	portal  dbus.BusObject
	sender  string
	name    string
	signals chan *dbus.Signal
}

// newPortalRequester subscribes to the Response signals conn's portal requests
// answer on, and returns the requester and a release func. Subscribing before
// any call goes out closes the race the portal spec warns of, where a Response
// beats the method reply. Release drops it when the handshake ends, since
// nothing after answers on a Request. name reaches the user in every failure.
func newPortalRequester(
	ctx context.Context,
	conn *dbus.Conn,
	name string,
) (*portalRequester, func(), error) {
	names := conn.Names()
	if len(names) == 0 {
		return nil, nil, derrors.New(
			derrors.CodeActionFailed,
			"the D-Bus session bus assigned no unique name, so no portal request "+
				"could be listened for",
		)
	}

	requester := &portalRequester{
		conn:    conn,
		portal:  conn.Object(portalBusName, portalObjectPath),
		sender:  names[0],
		name:    name,
		signals: make(chan *dbus.Signal, portalSignalBuffer),
	}

	conn.Signal(requester.signals)

	// One match rule covers every Request-answering call, because every Request
	// the portal creates for us lives under the same path namespace.
	err := conn.AddMatchSignalContext(ctx,
		dbus.WithMatchInterface(portalRequestInterface),
		dbus.WithMatchMember(portalResponseSignal),
		dbus.WithMatchPathNamespace(portalRequestNamespace),
	)
	if err != nil {
		conn.RemoveSignal(requester.signals)

		return nil, nil, derrors.Wrapf(
			err,
			derrors.CodeActionFailed,
			"could not subscribe to the %s portal's replies",
			name,
		)
	}

	release := func() {
		_ = conn.RemoveMatchSignal(
			dbus.WithMatchInterface(portalRequestInterface),
			dbus.WithMatchMember(portalResponseSignal),
			dbus.WithMatchPathNamespace(portalRequestNamespace),
		)

		conn.RemoveSignal(requester.signals)
	}

	return requester, release, nil
}

// call makes one portal call and waits for its Request to answer. It mints the
// handle token itself, because the answer's path derives from it and a
// mismatched token would wait on a path nothing answers.
func (r *portalRequester) call(
	ctx context.Context,
	method string,
	options map[string]dbus.Variant,
	args ...any,
) (map[string]dbus.Variant, error) {
	handleToken := portalHandleToken()
	options[portalHandleTokenKey] = dbus.MakeVariant(handleToken)

	var handle dbus.ObjectPath

	err := r.portal.CallWithContext(
		ctx,
		method,
		noPortalCallFlags,
		append(append([]any{}, args...), options)...,
	).Store(&handle)
	if err != nil {
		return nil, portalCallFailed(err, "the "+r.name+" portal refused "+method)
	}

	// The returned handle is authoritative. The derived path is only a fallback
	// for a portal that returns none.
	if handle == "" {
		handle = portalRequestPath(r.sender, handleToken)
	}

	return r.await(ctx, handle, method)
}

// await blocks until the Request at handle answers, or ctx runs out.
func (r *portalRequester) await(
	ctx context.Context,
	handle dbus.ObjectPath,
	method string,
) (map[string]dbus.Variant, error) {
	responseName := portalRequestInterface + "." + portalResponseSignal

	for {
		select {
		case <-ctx.Done():
			return nil, derrors.Wrapf(
				ctx.Err(),
				derrors.CodeTimeout,
				"the %s portal did not answer %s in time",
				r.name,
				method,
			)
		case signal, open := <-r.signals:
			if !open {
				return nil, derrors.Newf(
					derrors.CodeActionFailed,
					"the D-Bus session bus closed while waiting for %s",
					method,
				)
			}

			if signal.Path != handle || signal.Name != responseName {
				continue
			}

			code, results, err := decodePortalResponse(r.name, signal)
			if err != nil {
				return nil, err
			}

			err = portalResponseError(r.name, code)
			if err != nil {
				return nil, err
			}

			return results, nil
		}
	}
}

// createPortalSession performs CreateSession, which opens every portal
// interface, and returns the session handle.
func createPortalSession(
	ctx context.Context,
	requester *portalRequester,
	method string,
) (dbus.ObjectPath, error) {
	results, err := requester.call(ctx, method,
		map[string]dbus.Variant{
			portalSessionHandleTokenKey: dbus.MakeVariant(portalHandleToken()),
		})
	if err != nil {
		return "", err
	}

	session, ok := portalSessionHandle(results)
	if !ok {
		return "", derrors.Newf(
			derrors.CodeActionFailed,
			"the %s portal created a session but named no session handle",
			requester.name,
		)
	}

	return session, nil
}

// portalCallFailed phrases a failed portal call. It never wraps the bus error,
// because the backend writes the error body and can quote a rejected option,
// such as the restore token, a credential. The error name is carried instead.
// A call that ran out of time is a timeout, not a refusal, so a slow portal is
// not mistaken for one that rejected the stored grant (storedGrantPresumedDead).
func portalCallFailed(err error, message string) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return derrors.New(derrors.CodeTimeout, message+", and the request ran out of time")
	}

	name := busErrorName(err)
	if name != "" {
		return derrors.New(derrors.CodeActionFailed, message+" ("+name+")")
	}

	return derrors.New(derrors.CodeActionFailed, message)
}

// busErrorName reduces a D-Bus error to its name, and reports "" for anything
// that is not one. godbus yields dbus.Error by value on a client call and by
// pointer from NewError, so both are matched.
func busErrorName(err error) string {
	var busError dbus.Error

	var busErrorPtr *dbus.Error

	switch {
	case errors.As(err, &busError):
		return busError.Name
	case errors.As(err, &busErrorPtr):
		return busErrorPtr.Name
	default:
		return ""
	}
}

// closePortalSession ends a session explicitly rather than waiting for the
// portal to notice the connection close, until which the user would see a held
// grant and, for ScreenCast, a "screen is being shared" indicator. It expects
// no reply, because the suspend/resume reset calls it on the goroutine holding
// the keyboard grab, where waiting on a wedged portal would freeze hotkeys.
func closePortalSession(conn *dbus.Conn, session dbus.ObjectPath) {
	_ = conn.Object(portalBusName, session).
		Call(portalSessionCloseMethod, dbus.FlagNoReplyExpected).Err
}

// decodePortalResponse reads the (response code, results) pair a Response
// signal carries.
func decodePortalResponse(
	name string,
	signal *dbus.Signal,
) (uint32, map[string]dbus.Variant, error) {
	const responseBodyLength = 2

	if len(signal.Body) < responseBodyLength {
		return 0, nil, derrors.Newf(
			derrors.CodeActionFailed,
			"the %s portal sent a malformed reply",
			name,
		)
	}

	code, ok := signal.Body[0].(uint32)
	if !ok {
		return 0, nil, derrors.Newf(
			derrors.CodeActionFailed,
			"the %s portal sent a reply with no response code",
			name,
		)
	}

	results, _ := signal.Body[1].(map[string]dbus.Variant)

	return code, results, nil
}

// portalResponseError turns a Response code into the error the restore policy
// branches on. Cancel is the user's own answer, so it stays distinct.
func portalResponseError(name string, code uint32) error {
	switch code {
	case portalResponseSuccess:
		return nil
	case portalResponseCanceled:
		return errPortalRequestCanceled
	case portalResponseEnded:
		return derrors.Newf(
			derrors.CodeActionFailed,
			"the %s portal ended the request before it was answered",
			name,
		)
	default:
		return derrors.Newf(
			derrors.CodeActionFailed,
			"the %s portal answered with an unknown response code %d",
			name,
			code,
		)
	}
}

// portalSessionHandle reads a CreateSession reply's session handle. The spec
// types it as a string but some portals send an object path, so both pass.
func portalSessionHandle(results map[string]dbus.Variant) (dbus.ObjectPath, bool) {
	value, ok := results["session_handle"]
	if !ok {
		return "", false
	}

	switch handle := value.Value().(type) {
	case string:
		return dbus.ObjectPath(handle), handle != ""
	case dbus.ObjectPath:
		return handle, handle != ""
	default:
		return "", false
	}
}

// portalRequestPath derives the object path a Request answers on, per the
// portal specification: the sender's unique name with its leading colon removed
// and its dots replaced by underscores, followed by the handle token.
func portalRequestPath(sender, handleToken string) dbus.ObjectPath {
	element := strings.ReplaceAll(strings.TrimPrefix(sender, ":"), ".", "_")

	return dbus.ObjectPath(portalRequestPathPrefix + element + "/" + handleToken)
}

// portalHandleToken returns a token unique among this connection's requests.
// The portal pastes it into an object path, so it stays within [A-Za-z0-9_].
func portalHandleToken() string {
	buffer := make([]byte, portalHandleTokenBytes)

	// crypto/rand.Read is documented never to fail on any supported platform.
	_, _ = rand.Read(buffer)

	return "neru_" + hex.EncodeToString(buffer)
}
