//go:build linux

package linux

import (
	"context"
	"image"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The org.freedesktop.portal.ScreenCast handshake, spoken directly over the
// session bus. The transport underneath it is portal_bus.go, and the policy
// that decides which token to present is portal_session.go — the same one the
// input grant uses.
//
// This is KDE's only pixel source. X11 reads the root window back with
// XGetImage and the wlroots family implements wlr-screencopy, neither of which
// needs a consent picker in front of what becomes a hint refresh; KWin
// implements no screencopy protocol Neru can use, so KDE pays the portal
// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md). It pays it
// once: the grant is persisted with a restore token and restored on every later
// start, exactly as the input grant is.
//
// Privacy: nothing here ever holds a pixel. This file negotiates which PipeWire
// node the frames will arrive on and hands the socket to the native reader; the
// frame itself is read, cropped and released in pipewire_capture.c, and only
// dimensions, durations and booleans are ever logged anywhere on the path.

// The three ScreenCast methods one session needs, in the order they run, plus
// the plain call that opens the PipeWire connection frames arrive over.
const (
	screenCastCreateSession      = "org.freedesktop.portal.ScreenCast.CreateSession"
	screenCastSelectSources      = "org.freedesktop.portal.ScreenCast.SelectSources"
	screenCastStart              = "org.freedesktop.portal.ScreenCast.Start"
	screenCastOpenPipeWireRemote = "org.freedesktop.portal.ScreenCast.OpenPipeWireRemote"
)

// screenCastSourceMonitor is source type 1 from the ScreenCast specification's
// `types` bitmask: whole outputs.
//
// Windows (type 2) are deliberately not asked for. A caller's rectangle is in
// global screen coordinates, and a window stream carries no position, so a
// window source could not be mapped back onto the region that was asked for —
// and the region contract is what makes a captured frame placeable at all.
const screenCastSourceMonitor uint32 = 1

// screenCastCursorHidden is cursor_mode 1: the pointer is left out of the
// frames. It is the only honest choice for this caller — the frame exists to be
// read for on-screen text and elements, and a cursor drawn into it is a shape
// the detector would try to interpret. It is also the one cursor mode every
// portal backend is required to support.
const screenCastCursorHidden uint32 = 1

// screenCastStream is one monitor the granted session streams, and where that
// monitor sits.
type screenCastStream struct {
	// nodeID is the PipeWire node the frames arrive on.
	nodeID uint32
	// bounds is the monitor's place in Neru's shared coordinate space: global
	// origin, top-left, Y down, logical pixels. It is the zero rectangle when
	// the portal named no position or size for the stream, which is the one
	// case a region cannot be honored — see selectScreenCastStream.
	bounds image.Rectangle
}

// screenCastGrant is one established ScreenCast grant: the monitors it streams,
// the token that restores it next time, and the two things that outlive the
// handshake — a way to open a PipeWire connection for a frame, and a way to end
// the session.
type screenCastGrant struct {
	streams      []screenCastStream
	restoreToken string
	// openPipeWire returns a PipeWire remote file descriptor for one capture.
	// Ownership of the descriptor passes to the caller, which hands it to the
	// native reader; the reader closes it on every path.
	openPipeWire func(ctx context.Context) (int, error)
	// close ends the portal session and releases the D-Bus connection holding
	// it, which is what takes KDE's screen-sharing indicator back off the panel.
	close func()
}

// grantRestoreToken reports the token this grant handed back, which is what
// makes it usable by the shared restore policy.
func (g screenCastGrant) grantRestoreToken() string { return g.restoreToken }

var _ portalRestorableGrant = screenCastGrant{}

// openScreenCastSession runs the ScreenCast handshake and returns the grant it
// produced.
//
// When restoreToken is not empty the portal is asked to restore the grant it
// names, which is what keeps the source picker off the screen on a restart. A
// token the portal will not accept is not an error here: the specification says
// an unrestorable token is ignored and the user prompted normally, and the
// caller decides whether a prompt is acceptable on the path it is on.
//
// Every step is bounded by ctx, including the bus dial, which godbus cannot
// cancel on its own — see dialPortalBus.
func openScreenCastSession(ctx context.Context, restoreToken string) (screenCastGrant, error) {
	conn, err := dialPortalBus(ctx)
	if err != nil {
		return screenCastGrant{}, unrelatedToStoredGrant(err)
	}

	grant, err := negotiateScreenCast(ctx, conn, restoreToken)
	if err != nil {
		_ = conn.Close()

		return screenCastGrant{}, err
	}

	return grant, nil
}

// negotiateScreenCast runs CreateSession, SelectSources and Start on conn, in
// that order, and returns the grant they produced.
//
// OpenPipeWireRemote is deliberately *not* run here. It is one plain method
// call per capture rather than once per session, because a PipeWire connection
// is consumed by the reader that drains it — the descriptor cannot be reused,
// and holding a live stream open between captures would have the compositor
// pushing frames of the user's screen at Neru continuously for the sake of the
// handful it is asked for.
func negotiateScreenCast(
	ctx context.Context,
	conn *dbus.Conn,
	restoreToken string,
) (screenCastGrant, error) {
	// Everything up to and including CreateSession runs before SelectSources
	// presents the restore token, so each failure below is marked as one the
	// stored grant cannot be blamed for.
	requester, release, err := newPortalRequester(ctx, conn, portalNameScreenCast)
	if err != nil {
		return screenCastGrant{}, unrelatedToStoredGrant(err)
	}

	defer release()

	session, err := createPortalSession(ctx, requester, screenCastCreateSession)
	if err != nil {
		return screenCastGrant{}, unrelatedToStoredGrant(err)
	}

	// The token is in play across the next two calls: SelectSources carries it
	// and Start consumes it, so a refusal from either is the one signal there is
	// that the stored grant is no longer good. Their errors go back unmarked.
	_, err = requester.call(ctx, screenCastSelectSources,
		screenCastSelectSourcesOptions(restoreToken), session)
	if err != nil {
		closePortalSession(conn, session)

		return screenCastGrant{}, err
	}

	results, err := requester.call(ctx, screenCastStart,
		map[string]dbus.Variant{}, session, portalNoParentWindow)
	if err != nil {
		closePortalSession(conn, session)

		return screenCastGrant{}, err
	}

	newToken, _ := results[portalRestoreTokenKey].Value().(string)

	return screenCastGrant{
		streams:      decodeScreenCastStreams(results),
		restoreToken: newToken,
		openPipeWire: func(callCtx context.Context) (int, error) {
			return openPipeWireRemote(callCtx, portalObjectOn(conn), session)
		},
		close: func() {
			closePortalSession(conn, session)

			_ = conn.Close()
		},
	}, nil
}

// portalObjectOn builds the bare portal object a plain (non-Request) call needs.
// OpenPipeWireRemote answers with the descriptor directly, so it needs no
// signal subscription and must not depend on one — the handshake's subscription
// is gone by the time the first capture runs.
func portalObjectOn(conn *dbus.Conn) dbus.BusObject {
	return conn.Object(portalBusName, portalObjectPath)
}

// openPipeWireRemote asks the portal for a PipeWire connection to the granted
// session's streams. The descriptor it returns belongs to the caller.
func openPipeWireRemote(
	ctx context.Context,
	portal dbus.BusObject,
	session dbus.ObjectPath,
) (int, error) {
	var remote dbus.UnixFD

	err := portal.CallWithContext(
		ctx,
		screenCastOpenPipeWireRemote,
		noPortalCallFlags,
		session,
		map[string]dbus.Variant{},
	).Store(&remote)
	if err != nil {
		return 0, portalCallFailed(
			err,
			"the ScreenCast portal granted a session but would not open a PipeWire "+
				"connection to read frames from",
		)
	}

	return int(remote), nil
}

// screenCastSelectSourcesOptions builds the options for SelectSources: which
// sources are wanted, whether more than one may be picked, what to do about the
// pointer, how long the grant should last, and which grant to restore. The
// handle token is added by portalRequester.call, which owns it.
//
// multiple is true because a region can land on any monitor, and a grant that
// covers one display would refuse every capture on the others — a person with
// two screens would meet the picker again the first time they used hints on the
// second one. What the user shares is still their choice; asking for the option
// is what lets them say "all of them" once.
//
// The restore token is omitted rather than sent empty when there is nothing to
// restore, so a portal backend that validates the key it was handed does not
// refuse the ordinary first run.
func screenCastSelectSourcesOptions(restoreToken string) map[string]dbus.Variant {
	options := map[string]dbus.Variant{
		"types":        dbus.MakeVariant(screenCastSourceMonitor),
		"multiple":     dbus.MakeVariant(true),
		"cursor_mode":  dbus.MakeVariant(screenCastCursorHidden),
		"persist_mode": dbus.MakeVariant(portalPersistUntilRevoked),
	}

	if restoreToken != "" {
		options[portalRestoreTokenKey] = dbus.MakeVariant(restoreToken)
	}

	return options
}

// decodeScreenCastStreams reads the streams out of a Start reply.
//
// The reply is typed a(ua{sv}): a node id and a property bag per stream. The
// two properties that matter are position and size, which place the monitor in
// the same global top-left space the caller's region is in. A stream missing
// either is kept with zero bounds rather than dropped, so the refusal a caller
// gets names the missing geometry instead of pretending the session streams
// nothing.
func decodeScreenCastStreams(results map[string]dbus.Variant) []screenCastStream {
	raw, ok := results["streams"].Value().([][]any)
	if !ok {
		return nil
	}

	// A stream entry is (node id, properties); anything shorter is malformed.
	const streamFields = 2

	streams := make([]screenCastStream, 0, len(raw))

	for _, entry := range raw {
		if len(entry) < streamFields {
			continue
		}

		nodeID, isNode := entry[0].(uint32)
		if !isNode {
			continue
		}

		properties, _ := entry[1].(map[string]dbus.Variant)

		streams = append(streams, screenCastStream{
			nodeID: nodeID,
			bounds: streamBounds(properties),
		})
	}

	return streams
}

// streamBounds turns a stream's position and size properties into a rectangle,
// and reports the zero rectangle when either is absent or unreadable.
func streamBounds(properties map[string]dbus.Variant) image.Rectangle {
	originX, originY, hasPosition := portalIntPair(properties, "position")
	width, height, hasSize := portalIntPair(properties, "size")

	if !hasPosition || !hasSize || width <= 0 || height <= 0 {
		return image.Rectangle{}
	}

	return image.Rect(originX, originY, originX+width, originY+height)
}

// portalIntPair reads a D-Bus (ii) structure out of a property bag. godbus
// decodes a struct into a []any of its members, so the pair arrives as two
// int32 values.
func portalIntPair(properties map[string]dbus.Variant, key string) (int, int, bool) {
	const pairFields = 2

	value, present := properties[key]
	if !present {
		return 0, 0, false
	}

	pair, isStruct := value.Value().([]any)
	if !isStruct || len(pair) < pairFields {
		return 0, 0, false
	}

	first, firstOK := pair[0].(int32)
	second, secondOK := pair[1].(int32)

	if !firstOK || !secondOK {
		return 0, 0, false
	}

	return int(first), int(second), true
}

// selectScreenCastStream finds the streamed monitor that wholly contains region
// and expresses region in that monitor's local logical coordinates.
//
// Refusing a region no single monitor contains is the contract the X11 and
// wlroots backends already keep, for the same reason: the only other way to
// answer is to crop, and a cropped frame carries nothing that says where its
// own top-left is. A caller that asked for one window must never silently
// receive a different rectangle, and one that asked for a region spanning two
// displays must never receive half of it.
func selectScreenCastStream(
	streams []screenCastStream,
	region image.Rectangle,
) (screenCastStream, image.Rectangle, error) {
	if len(streams) == 0 {
		return screenCastStream{}, image.Rectangle{}, derrors.New(
			derrors.CodeActionFailed,
			"the approved screen-sharing session streams no monitor; re-approve it "+
				"and pick a screen",
		)
	}

	placed := 0

	for _, stream := range streams {
		if stream.bounds.Empty() {
			continue
		}

		placed++

		if region.In(stream.bounds) {
			local := region.Sub(stream.bounds.Min)

			return stream, local, nil
		}
	}

	if placed == 0 {
		return screenCastStream{}, image.Rectangle{}, derrors.New(
			derrors.CodeActionFailed,
			"the ScreenCast portal named no geometry for the monitors it is "+
				"streaming, so a captured frame could not be placed on screen",
		)
	}

	return screenCastStream{}, image.Rectangle{}, derrors.Newf(
		derrors.CodeActionFailed,
		"no shared monitor covers the requested region %v; a region that leaves a "+
			"screen or spans two of them cannot be captured",
		region,
	)
}
