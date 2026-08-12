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

// The half of xdg-desktop-portal that is the same whichever interface is being
// spoken: dialing the session bus, issuing a call whose answer arrives as a
// Request's Response signal, and ending a session.
//
// Two interfaces are spoken on KDE and both are here because KWin implements
// neither of the protocols the blessed stack uses — org.freedesktop.portal
// .RemoteDesktop for input injection (portal_remotedesktop.go) and
// .ScreenCast for screen capture (portal_screencast.go). They are separate
// sessions on separate connections: a grant is what the user consented to, and
// folding capture into the input session would mean a person who only wants
// hints and grid mode being asked to share their screen.
//
// The connections are deliberately private rather than the process-wide
// dbus.SessionBus() the notifier and the tray share: the portal ties a
// session's lifetime to the connection that created it, so closing that
// connection is how a session is ended, and it must not be a connection
// anything else needs.
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

// portalPersistUntilRevoked is persist_mode 2: the grant survives until the
// user revokes it. Mode 1 persists only while the application runs, which is
// precisely the restart the restore token exists to survive.
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

// portalRestoreTokenKey is the option that presents a stored grant, and the key
// the same token comes back under from Start. It is spelled once so the two
// interfaces cannot disagree about it.
const portalRestoreTokenKey = "restore_token"

// portalNoParentWindow is the parent_window identifier Start takes. Neru has no
// toplevel to parent the dialog to — its overlays are layer-shell surfaces —
// so the portal places the dialog itself.
const portalNoParentWindow = ""

// noPortalCallFlags sends a plain method call: the reply is what carries the
// Request handle, so NO_REPLY_EXPECTED is exactly what must not be set.
const noPortalCallFlags = 0

// portalSignalBuffer sizes the channel Response signals arrive on. Four is one
// per call in the longest handshake here with room to spare; godbus never
// blocks on a full channel, so this only decides how often it defers a
// delivery.
const portalSignalBuffer = 4

// portalHandleTokenBytes is the entropy behind one handle token. The token only
// has to be unique among this connection's in-flight requests, so this is
// generous rather than load-bearing.
const portalHandleTokenBytes = 8

// The portal interfaces Neru speaks, as they are named in error messages. A
// failure has to say which grant it was about: "the ScreenCast portal refused"
// and "the RemoteDesktop portal refused" send a user to two different rows of
// their system settings.
const (
	portalNameRemoteDesktop = "RemoteDesktop"
	portalNameScreenCast    = "ScreenCast"
)

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
//
// That cap is shared by both grants, which couples them: while a ScreenCast
// dial is outstanding, a RemoteDesktop dial is refused, and the other way
// round. It is deliberate — what is being capped is one wedged session bus,
// which is process-wide however many portal interfaces are speaking to it — and
// it is bounded, because inFlight clears when ConnectSessionBus returns rather
// than when the handshake behind it finishes. A consent dialog left open for
// two minutes therefore blocks nothing.
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
				"no reachable D-Bus session bus, so xdg-desktop-portal cannot be "+
					"asked for a session",
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
				"so xdg-desktop-portal cannot be reached yet",
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
// will answer on and returns the requester, plus the function that drops the
// subscription again.
//
// The subscription is in place before any call goes out, which is what the
// portal specification prescribes to close the race where the Response beats
// the method reply. It is dropped when the handshake ends rather than left
// delivering into a channel with no reader for the daemon's lifetime — nothing
// after the handshake answers on a Request.
//
// name is the portal interface being spoken, and it reaches the user in every
// failure this requester produces.
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

// call makes one portal call and waits for the Request it creates to answer.
//
// It mints the handle token itself and writes it into options, because the
// object path the answer arrives on is derived from that token: a caller that
// chose one and then passed a different one in the options would wait on a path
// nothing ever answers.
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

// createPortalSession performs the CreateSession step, which every portal
// interface opens with, and returns the session handle the rest of the
// handshake addresses.
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

// closePortalSession ends a session explicitly. The portal would notice the
// connection closing eventually, but a session left open until then holds a
// grant the user can see in their portal settings — and, for a ScreenCast
// session, a "screen is being shared" indicator they can see on their panel.
//
// It expects no reply, which is what makes it safe to call from anywhere: a
// teardown reaches this from the suspend/resume reset that runs on the
// goroutine holding the keyboard grab, and waiting for a wedged portal to
// acknowledge a close there would freeze every hotkey for as long as it took.
// There is nothing to learn from the reply anyway — the session is being
// abandoned either way.
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
// branches on. A canceled request is separated from every other failure because
// it is the user's own answer, and a second dialog would only ask them again.
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
