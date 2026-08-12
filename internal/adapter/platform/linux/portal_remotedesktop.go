//go:build linux

package linux

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The org.freedesktop.portal.RemoteDesktop handshake, spoken directly over the
// session bus.
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
//
// The connection is deliberately private rather than the process-wide
// dbus.SessionBus() the notifier and the tray share: the portal ties the
// session's lifetime to the connection that created it, so closing this one is
// how a session is ended, and it must not be a connection anything else needs.
const (
	// portalBusName is the well-known name xdg-desktop-portal owns.
	portalBusName = "org.freedesktop.portal.Desktop"
	// portalObjectPath is the object every portal interface is exported on.
	portalObjectPath = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	// portalRequestPathPrefix opens the object path a Request is exported on.
	// The remainder is our unique bus name and the handle token we chose, which
	// is what lets a listener be in place before the call is even made.
	portalRequestPathPrefix = "/org/freedesktop/portal/desktop/request/"
	// portalRequestNamespace is the path namespace every Request lives under,
	// matched once so all four calls below share one match rule.
	portalRequestNamespace = dbus.ObjectPath("/org/freedesktop/portal/desktop/request")
	// portalRequestInterface carries the Response signal a Request answers with.
	portalRequestInterface = "org.freedesktop.portal.Request"
	// portalResponseSignal is that signal's member name.
	portalResponseSignal = "Response"
	// portalSessionCloseMethod ends a session explicitly, rather than leaving
	// the portal to notice the connection went away.
	portalSessionCloseMethod = "org.freedesktop.portal.Session.Close"
)

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

// portalPersistUntilRevoked is persist_mode 2: the grant survives until the
// user revokes it. Mode 1 persists only while the application runs, which is
// precisely the restart this file exists to survive.
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

// portalNoParentWindow is the parent_window identifier Start takes. Neru has no
// toplevel to parent the dialog to — its overlays are layer-shell surfaces —
// so the portal places the dialog itself.
const portalNoParentWindow = ""

// noPortalCallFlags sends a plain method call: the reply is what carries the
// Request handle, so NO_REPLY_EXPECTED is exactly what must not be set.
const noPortalCallFlags = 0

// portalSignalBuffer sizes the channel Response signals arrive on. Four is one
// per call in the handshake with room to spare; godbus never blocks on a full
// channel, so this only decides how often it defers a delivery.
const portalSignalBuffer = 4

// portalHandleTokenBytes is the entropy behind one handle token. The token only
// has to be unique among this connection's in-flight requests, so this is
// generous rather than load-bearing.
const portalHandleTokenBytes = 8

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
		return portalGrant{}, failedBeforePresenting(err)
	}

	grant, err := negotiateRemoteDesktop(ctx, conn, restoreToken)
	if err != nil {
		_ = conn.Close()

		return portalGrant{}, err
	}

	return grant, nil
}

// dialedBus is one outcome of the uncancellable session-bus dial.
type dialedBus struct {
	conn *dbus.Conn
	err  error
}

// portalDialer holds the one bus dial that may be outstanding at a time.
type portalDialer struct {
	mu       sync.Mutex
	inFlight bool
}

// portalBusDialer is process-wide because the thing it caps is process-wide:
// one wedged session bus, however many callers walk into it.
var portalBusDialer = &portalDialer{}

// dialPortalBus connects to the session bus without letting an unresponsive bus
// hold the caller, and without starting a second dial while one is stuck.
//
// dbus.ConnectSessionBus takes no context: the connect, the auth handshake and
// the Hello round trip are all uncancellable, so a bus that accepts a
// connection and then stops answering would block until it answers. The mid-
// action input path reaches this while holding the keyboard grab, so blocking
// there freezes every global hotkey — the dial therefore runs on a goroutine
// the caller can abandon, and whatever it eventually produces is closed rather
// than leaked.
//
// Abandoning is not enough on its own. Every mid-action input operation reaches
// this while no session is established, so a wedged bus would otherwise buy one
// more orphaned goroutine and half-open connection per keypress. The same rule
// the capability probes follow applies here — one outstanding attempt, never
// restarted while it is stuck — so a caller that arrives to find a dial already
// hanging is refused immediately rather than adding to the pile.
func dialPortalBus(ctx context.Context) (*dbus.Conn, error) {
	done, err := portalBusDialer.start()
	if err != nil {
		return nil, err
	}

	select {
	case result := <-done:
		if result.err != nil {
			return nil, derrors.Wrap(
				result.err,
				derrors.CodeNotSupported,
				"no reachable D-Bus session bus, so the RemoteDesktop portal "+
					"cannot be asked for an input session",
			)
		}

		return result.conn, nil
	case <-ctx.Done():
		go func() {
			result := <-done
			if result.conn != nil {
				_ = result.conn.Close()
			}
		}()

		return nil, derrors.Wrap(
			ctx.Err(),
			derrors.CodeTimeout,
			"the D-Bus session bus did not accept a connection in time",
		)
	}
}

// start launches the dial, or refuses when one is already outstanding.
func (d *portalDialer) start() (<-chan dialedBus, error) {
	d.mu.Lock()

	if d.inFlight {
		d.mu.Unlock()

		return nil, derrors.New(
			derrors.CodeTimeout,
			"an earlier connection to the D-Bus session bus is still unanswered, "+
				"so the RemoteDesktop portal cannot be reached yet",
		)
	}

	d.inFlight = true

	d.mu.Unlock()

	done := make(chan dialedBus, 1)

	go func() {
		conn, err := dbus.ConnectSessionBus()

		d.mu.Lock()
		d.inFlight = false
		d.mu.Unlock()

		done <- dialedBus{conn: conn, err: err}
	}()

	return done, nil
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
	names := conn.Names()
	if len(names) == 0 {
		return portalGrant{}, failedBeforePresenting(derrors.New(
			derrors.CodeActionFailed,
			"the D-Bus session bus assigned no unique name, so no portal request "+
				"could be listened for",
		))
	}

	requester := &portalRequester{
		conn:    conn,
		portal:  conn.Object(portalBusName, portalObjectPath),
		sender:  names[0],
		signals: make(chan *dbus.Signal, portalSignalBuffer),
	}

	conn.Signal(requester.signals)

	// One match rule covers all three Request-answering calls, because every
	// Request the portal creates for us lives under the same path namespace.
	err := conn.AddMatchSignalContext(ctx,
		dbus.WithMatchInterface(portalRequestInterface),
		dbus.WithMatchMember(portalResponseSignal),
		dbus.WithMatchPathNamespace(portalRequestNamespace),
	)
	if err != nil {
		conn.RemoveSignal(requester.signals)

		return portalGrant{}, failedBeforePresenting(derrors.Wrap(
			err,
			derrors.CodeActionFailed,
			"could not subscribe to the RemoteDesktop portal's replies",
		))
	}

	// Nothing after the handshake answers on a Request, so the subscription is
	// dropped either way rather than left delivering into a channel with no
	// reader for the daemon's lifetime.
	defer func() {
		_ = conn.RemoveMatchSignal(
			dbus.WithMatchInterface(portalRequestInterface),
			dbus.WithMatchMember(portalResponseSignal),
			dbus.WithMatchPathNamespace(portalRequestNamespace),
		)

		conn.RemoveSignal(requester.signals)
	}()

	session, err := createRemoteDesktopSession(ctx, requester)
	if err != nil {
		return portalGrant{}, failedBeforePresenting(err)
	}

	// From here the token is in play: SelectDevices is the call that carries it.
	err = selectRemoteDesktopDevices(ctx, requester, session, restoreToken)
	if err != nil {
		closeRemoteDesktopSession(conn, session)

		return portalGrant{}, err
	}

	newToken, err := startRemoteDesktopSession(ctx, requester, session)
	if err != nil {
		closeRemoteDesktopSession(conn, session)

		return portalGrant{}, err
	}

	eisFD, err := connectToEIS(ctx, requester, session)
	if err != nil {
		closeRemoteDesktopSession(conn, session)

		return portalGrant{}, err
	}

	return portalGrant{
		eisFD:        eisFD,
		restoreToken: newToken,
		close: func() {
			closeRemoteDesktopSession(conn, session)

			_ = conn.Close()
		},
	}, nil
}

// createRemoteDesktopSession performs step 1 and returns the session handle the
// remaining three calls address.
func createRemoteDesktopSession(
	ctx context.Context,
	requester *portalRequester,
) (dbus.ObjectPath, error) {
	results, err := requester.call(ctx, remoteDesktopCreateSession,
		map[string]dbus.Variant{
			"session_handle_token": dbus.MakeVariant(portalHandleToken()),
		})
	if err != nil {
		return "", err
	}

	session, ok := portalSessionHandle(results)
	if !ok {
		return "", derrors.New(
			derrors.CodeActionFailed,
			"the RemoteDesktop portal created a session but named no session handle",
		)
	}

	return session, nil
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
	token, _ := results["restore_token"].Value().(string)

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

// portalCallFailed phrases a failed portal method call.
//
// It deliberately does not wrap the bus error itself. A D-Bus error body is
// written by the portal backend and can quote the option it rejected, and one
// of the options sent here is a restore token — a credential that must not
// reach an error a caller may print or log. The error *name* says what went
// wrong without quoting anything we sent, so that is what is carried.
//
// A call that ran out of time is reported as a timeout rather than a refusal,
// which is what keeps a slow portal from being mistaken for one that rejected
// the stored grant (see storedGrantPresumedDead).
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

// closeRemoteDesktopSession ends a session explicitly. The portal would notice
// the connection closing eventually, but a session left open until then holds a
// grant the user can see in their portal settings.
//
// It expects no reply, which is what makes it safe to call from anywhere: a
// teardown reaches this from the suspend/resume reset that runs on the
// goroutine holding the keyboard grab, and waiting for a wedged portal to
// acknowledge a close there would freeze every hotkey for as long as it took.
// There is nothing to learn from the reply anyway — the session is being
// abandoned either way.
func closeRemoteDesktopSession(conn *dbus.Conn, session dbus.ObjectPath) {
	_ = conn.Object(portalBusName, session).
		Call(portalSessionCloseMethod, dbus.FlagNoReplyExpected).Err
}

// portalRequester issues portal calls whose answer arrives as a Request
// Response signal rather than as the method's own reply.
type portalRequester struct {
	conn    *dbus.Conn
	portal  dbus.BusObject
	sender  string
	signals chan *dbus.Signal
}

// call makes one portal call and waits for the Request it creates to answer.
//
// It mints the handle token itself and writes it into options, because the
// object path the answer arrives on is derived from that token: a caller that
// chose one and then passed a different one in the options would wait on a path
// nothing ever answers. The subscription covering that path is already in place
// before the call goes out, which is what the portal specification prescribes to
// close the race where the Response beats the method reply.
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
		return nil, portalCallFailed(err, "the RemoteDesktop portal refused "+method)
	}

	// The portal answers on the path it chose. That is normally the derived one
	// — which is why the subscription could be set up first — but the returned
	// handle is what is authoritative, so it is what the wait matches on.
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
				"the RemoteDesktop portal did not answer %s in time",
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

			code, results, err := decodePortalResponse(signal)
			if err != nil {
				return nil, err
			}

			err = portalResponseError(code)
			if err != nil {
				return nil, err
			}

			return results, nil
		}
	}
}

// decodePortalResponse reads the (response code, results) pair a Response
// signal carries.
func decodePortalResponse(signal *dbus.Signal) (uint32, map[string]dbus.Variant, error) {
	const responseBodyLength = 2

	if len(signal.Body) < responseBodyLength {
		return 0, nil, derrors.New(
			derrors.CodeActionFailed,
			"the RemoteDesktop portal sent a malformed reply",
		)
	}

	code, ok := signal.Body[0].(uint32)
	if !ok {
		return 0, nil, derrors.New(
			derrors.CodeActionFailed,
			"the RemoteDesktop portal sent a reply with no response code",
		)
	}

	results, _ := signal.Body[1].(map[string]dbus.Variant)

	return code, results, nil
}

// portalResponseError turns a Response code into the error the restore policy
// branches on. A canceled request is separated from every other failure because
// it is the user's own answer, and a second dialog would only ask them again.
func portalResponseError(code uint32) error {
	switch code {
	case portalResponseSuccess:
		return nil
	case portalResponseCanceled:
		return errPortalRequestCanceled
	case portalResponseEnded:
		return derrors.New(
			derrors.CodeActionFailed,
			"the RemoteDesktop portal ended the request before it was answered",
		)
	default:
		return derrors.Newf(
			derrors.CodeActionFailed,
			"the RemoteDesktop portal answered with an unknown response code %d",
			code,
		)
	}
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
		options["restore_token"] = dbus.MakeVariant(restoreToken)
	}

	return options
}

// portalSessionHandle reads the session handle out of a CreateSession reply.
// The specification types it as a string, and some portal implementations send
// an object path instead, so both are accepted.
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

// portalHandleToken returns a token unique among this connection's requests and
// legal as a D-Bus object path element — the portal pastes it straight into
// one, so anything outside [A-Za-z0-9_] would have the call rejected.
func portalHandleToken() string {
	buffer := make([]byte, portalHandleTokenBytes)

	// crypto/rand.Read is documented never to fail on any supported platform.
	_, _ = rand.Read(buffer)

	return "neru_" + hex.EncodeToString(buffer)
}
