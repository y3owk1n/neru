//go:build linux

package linux

import (
	"context"
	"image"
	"strings"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The org.freedesktop.portal.ScreenCast handshake over the session bus, on the
// transport in portal_bus.go and the restore policy in portal_session.go.
//
// This is KDE's only pixel source. X11 and wlroots capture without a consent
// picker, but KWin has no screencopy protocol Neru can use, so KDE pays the
// portal once and restores the grant on later starts
// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md).
//
// Privacy: nothing here holds a pixel. This file picks the PipeWire node and
// hands the socket to pipewire_capture.c, which reads, crops and releases the
// frame. Only dimensions, durations and booleans are logged on the path.

// The three ScreenCast methods one session needs, in the order they run, plus
// the plain call that opens the PipeWire connection frames arrive over.
const (
	screenCastCreateSession      = "org.freedesktop.portal.ScreenCast.CreateSession"
	screenCastSelectSources      = "org.freedesktop.portal.ScreenCast.SelectSources"
	screenCastStart              = "org.freedesktop.portal.ScreenCast.Start"
	screenCastOpenPipeWireRemote = "org.freedesktop.portal.ScreenCast.OpenPipeWireRemote"
)

// screenCastSourceMonitor is ScreenCast source type 1, whole outputs. Windows
// (type 2) are not requested, because a window stream carries no position and
// could not be mapped back onto the caller's global region.
const screenCastSourceMonitor uint32 = 1

// screenCastCursorHidden is cursor_mode 1, pointer left out. A drawn cursor is
// a shape the detector would try to interpret, and this mode is the one every
// portal backend must support.
const screenCastCursorHidden uint32 = 1

// screenCastStream is one monitor the granted session streams, and where that
// monitor sits.
type screenCastStream struct {
	// nodeID is the PipeWire node the frames arrive on.
	nodeID uint32
	// bounds is the monitor's place in Neru's shared space, in logical pixels.
	// It is zero when the portal named no size, the one case a region cannot be
	// honored (selectScreenCastStream).
	bounds image.Rectangle
	// positioned reports whether the portal said where the monitor sits.
	// Released Plasma (5.27 through 6.5) names only a size. Position arrived
	// with screencasting v6. An unpositioned stream sits at the origin until
	// placeScreenCastStreams finds its output.
	positioned bool
	// mappingID is the output name a Plasma speaking screencasting v5 or later
	// attaches to a monitor stream, and empty before that.
	mappingID string
}

// screenCastGrant is one established ScreenCast grant, with its streams,
// restore token, and the two functions that outlive the handshake.
type screenCastGrant struct {
	streams      []screenCastStream
	restoreToken string
	// openPipeWire returns a PipeWire remote fd for one capture. The caller
	// passes it to the native reader, which closes it on every path.
	openPipeWire func(ctx context.Context) (int, error)
	// close ends the portal session and releases the D-Bus connection holding
	// it, which is what takes KDE's screen-sharing indicator back off the panel.
	close func()
}

// grantRestoreToken implements portalRestorableGrant.
func (g screenCastGrant) grantRestoreToken() string { return g.restoreToken }

var _ portalRestorableGrant = screenCastGrant{}

// openScreenCastSession runs the ScreenCast handshake and returns its grant. A
// non-empty restoreToken asks the portal to restore that grant without the
// picker. The spec says an unrestorable token is ignored and the user
// prompted, and the caller decides whether a prompt is acceptable. Every step,
// including the bus dial, is bounded by ctx (see dialPortalBus).
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

// negotiateScreenCast runs CreateSession, SelectSources and Start on conn and
// returns the grant. OpenPipeWireRemote runs per capture instead, because the
// reader consumes the descriptor and a stream held open would push the user's
// screen at Neru between captures.
func negotiateScreenCast(
	ctx context.Context,
	conn *dbus.Conn,
	restoreToken string,
) (screenCastGrant, error) {
	// Everything through CreateSession runs before the token is presented, so
	// these failures cannot be blamed on the stored grant.
	requester, release, err := newPortalRequester(ctx, conn, portalNameScreenCast)
	if err != nil {
		return screenCastGrant{}, unrelatedToStoredGrant(err)
	}

	defer release()

	session, err := createPortalSession(ctx, requester, screenCastCreateSession)
	if err != nil {
		return screenCastGrant{}, unrelatedToStoredGrant(err)
	}

	// SelectSources carries the token and Start consumes it, so their errors go
	// back unmarked as the one signal that the stored grant is dead.
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

// portalObjectOn builds the bare portal object a plain call needs.
// OpenPipeWireRemote answers directly and must not depend on the handshake's
// signal subscription, which is gone by the first capture.
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

// screenCastSelectSourcesOptions builds the SelectSources options.
// portalRequester adds the handle token. multiple is true because a region can
// land on any monitor, and a one-display grant would show the picker again the
// first time hints ran on another. The restore token is omitted rather than
// sent empty, so a backend that validates it does not refuse the first run.
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

// decodeScreenCastStreams reads the a(ua{sv}) streams out of a Start reply. A
// stream without usable geometry is kept rather than dropped, so the caller's
// refusal names the missing geometry instead of claiming the session streams
// nothing.
func decodeScreenCastStreams(results map[string]dbus.Variant) []screenCastStream {
	raw, ok := results["streams"].Value().([][]any)
	if !ok {
		return nil
	}

	// A stream entry is (node id, properties). Anything shorter is malformed.
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
		bounds, positioned := streamBounds(properties)
		mappingID, _ := properties["mapping_id"].Value().(string)

		streams = append(streams, screenCastStream{
			nodeID:     nodeID,
			bounds:     bounds,
			positioned: positioned,
			mappingID:  mappingID,
		})
	}

	return streams
}

// streamBounds turns a stream's position and size into a rectangle and reports
// whether the position was named. Without a size there is no rectangle. Without
// a position it sits at the origin.
func streamBounds(properties map[string]dbus.Variant) (image.Rectangle, bool) {
	width, height, hasSize := portalIntPair(properties, "size")
	if !hasSize || width <= 0 || height <= 0 {
		return image.Rectangle{}, false
	}

	originX, originY, hasPosition := portalIntPair(properties, "position")
	if !hasPosition {
		return image.Rect(0, 0, width, height), false
	}

	return image.Rect(originX, originY, originX+width, originY+height), true
}

// portalIntPair reads a D-Bus (ii) struct from a property bag, which godbus
// decodes as a []any of two int32 values.
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

// screenCastOutput is one output the compositor reports over wl_output: its
// name (the connector, "DP-1" or "eDP-1") and its bounds in the shared space.
type screenCastOutput struct {
	name   string
	bounds image.Rectangle
}

// placeScreenCastStreams places every unpositioned stream on an output the
// compositor reports over wl_output, matching first by mapping_id (the output
// name, set by Plasma with screencasting v5 or later) and then by size, since
// a monitor stream is a whole output. Each output is claimed once, so identical
// monitors pair off in KWin's order, which both sides list alike. A stream
// neither places stays at the origin, right for a single display. Guessing
// beats refusing, which would mean no hints on every Plasma released today.
func placeScreenCastStreams(
	streams []screenCastStream,
	outputs []screenCastOutput,
) []screenCastStream {
	placed := make([]screenCastStream, len(streams))
	copy(placed, streams)

	used := make([]bool, len(outputs))

	// An output the portal already placed a stream on is spoken for, so a
	// sized stream of the same shape cannot land on it too.
	for _, stream := range placed {
		if !stream.positioned {
			continue
		}

		for i, output := range outputs {
			if output.bounds == stream.bounds {
				used[i] = true
			}
		}
	}

	claim := func(stream *screenCastStream, match func(screenCastOutput) bool) {
		for i, output := range outputs {
			if used[i] || !match(output) {
				continue
			}

			used[i] = true
			stream.bounds = output.bounds
			stream.positioned = true

			return
		}
	}

	// Named streams first, so an exact match is never pre-empted by a sized
	// guess that happened to be listed earlier.
	for i := range placed {
		stream := &placed[i]
		if stream.positioned || stream.bounds.Empty() || stream.mappingID == "" {
			continue
		}

		claim(stream, func(output screenCastOutput) bool {
			return strings.EqualFold(output.name, stream.mappingID)
		})
	}

	for i := range placed {
		stream := &placed[i]
		if stream.positioned || stream.bounds.Empty() {
			continue
		}

		claim(stream, func(output screenCastOutput) bool {
			return output.bounds.Size() == stream.bounds.Size()
		})
	}

	return placed
}

// selectScreenCastStream finds the streamed monitor that wholly contains region
// and returns region in that monitor's local logical coordinates. A region no
// single monitor contains is refused, as on X11 and wlroots, because a cropped
// frame cannot say where its top-left is.
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
			"the ScreenCast portal named no size for the monitors it is "+
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
