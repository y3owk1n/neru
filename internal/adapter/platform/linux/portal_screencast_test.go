//go:build linux

package linux

import (
	"context"
	"image"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Two monitors side by side, the second one to the right of the first. The
// shapes are the ones a real dual-head KDE session reports: logical position
// and logical size, in the same global top-left space Neru uses everywhere.
func twoMonitorStreams() []screenCastStream {
	return []screenCastStream{
		{nodeID: 41, bounds: image.Rect(0, 0, 1920, 1080)},
		{nodeID: 42, bounds: image.Rect(1920, 0, 3840, 1080)},
	}
}

// TestSelectScreenCastStream_PicksTheMonitorHoldingTheRegion is the region
// contract's first half: a caller asking for a window on the second monitor
// gets that monitor's node, with the region translated into its local
// coordinates so the crop lands on the right pixels.
func TestSelectScreenCastStream_PicksTheMonitorHoldingTheRegion(t *testing.T) {
	stream, local, err := selectScreenCastStream(
		twoMonitorStreams(),
		image.Rect(2020, 100, 2420, 400),
	)
	if err != nil {
		t.Fatalf("selectScreenCastStream() error = %v", err)
	}

	if stream.nodeID != 42 {
		t.Errorf("stream.nodeID = %d, want 42", stream.nodeID)
	}

	want := image.Rect(100, 100, 500, 400)
	if local != want {
		t.Errorf("local region = %v, want %v", local, want)
	}
}

// TestSelectScreenCastStream_RefusesARegionNoMonitorContains is the other half,
// and the contract #1459 set: a rectangle that leaves the screen, or that spans
// two monitors, fails rather than coming back clipped. A clipped frame carries
// nothing that says where its own top-left is.
func TestSelectScreenCastStream_RefusesARegionNoMonitorContains(t *testing.T) {
	tests := []struct {
		name   string
		region image.Rectangle
	}{
		{name: "spans both monitors", region: image.Rect(1820, 100, 2020, 400)},
		{name: "runs off the right edge", region: image.Rect(3700, 0, 4000, 200)},
		{name: "starts above the top edge", region: image.Rect(10, -50, 110, 50)},
		{name: "on no monitor at all", region: image.Rect(5000, 5000, 5100, 5100)},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, _, err := selectScreenCastStream(twoMonitorStreams(), testCase.region)
			if err == nil {
				t.Fatal("selectScreenCastStream() error = nil, want a refusal")
			}

			if !derrors.IsCode(err, derrors.CodeActionFailed) {
				t.Errorf("error code = %q, want %q",
					derrors.GetCode(err), derrors.CodeActionFailed)
			}
		})
	}
}

// TestSelectScreenCastStream_RefusesAStreamWithNoGeometry pins what happens
// when the portal hands over a node but names no position or size for it.
// Without geometry there is no way to say which pixels of the frame the
// caller's rectangle is, so the capture refuses instead of guessing that the
// stream starts at the origin.
func TestSelectScreenCastStream_RefusesAStreamWithNoGeometry(t *testing.T) {
	streams := []screenCastStream{{nodeID: 7}}

	_, _, err := selectScreenCastStream(streams, image.Rect(0, 0, 100, 100))
	if err == nil {
		t.Fatal("selectScreenCastStream() error = nil, want a refusal")
	}

	if !strings.Contains(err.Error(), "geometry") {
		t.Errorf("error = %q, want it to name the missing geometry", err.Error())
	}
}

// TestSelectScreenCastStream_RefusesASessionStreamingNothing covers the session
// the user approved with no monitor selected: there is nothing to read, and
// saying so beats a nil dereference or an empty frame.
func TestSelectScreenCastStream_RefusesASessionStreamingNothing(t *testing.T) {
	_, _, err := selectScreenCastStream(nil, image.Rect(0, 0, 100, 100))
	if err == nil {
		t.Fatal("selectScreenCastStream() error = nil, want a refusal")
	}
}

// TestScreenCastSelectSourcesOptions_AsksForPersistentCursorlessMonitors pins
// the four values the whole backend rests on. Monitors rather than windows,
// because only a monitor stream carries a position and a region has to be
// placeable; every monitor the user is willing to share, because a region can
// land on any of them; no cursor, because the frame exists to be read for text
// and a pointer drawn into it is a shape the detector would try to interpret;
// and a persistence mode that outlives the process, since persist_mode 1 would
// keep the grant only while this daemon runs — precisely the restart the
// restore token exists to survive.
func TestScreenCastSelectSourcesOptions_AsksForPersistentCursorlessMonitors(t *testing.T) {
	options := screenCastSelectSourcesOptions(storedToken)

	types, isUint := options["types"].Value().(uint32)
	if !isUint || types != screenCastSourceMonitor {
		t.Errorf("types = %v, want monitors only (%d)", options["types"], screenCastSourceMonitor)
	}

	multiple, isBool := options["multiple"].Value().(bool)
	if !isBool || !multiple {
		t.Errorf("multiple = %v, want true so a region on any screen can be captured",
			options["multiple"])
	}

	cursor, isUint := options["cursor_mode"].Value().(uint32)
	if !isUint || cursor != screenCastCursorHidden {
		t.Errorf("cursor_mode = %v, want hidden (%d)", options["cursor_mode"],
			screenCastCursorHidden)
	}

	persist, isUint := options["persist_mode"].Value().(uint32)
	if !isUint || persist != portalPersistUntilRevoked {
		t.Errorf("persist_mode = %v, want %d", options["persist_mode"], portalPersistUntilRevoked)
	}

	token, isString := options[portalRestoreTokenKey].Value().(string)
	if !isString || token != storedToken {
		t.Errorf("restore_token = %v, want the stored token", options[portalRestoreTokenKey])
	}
}

// TestScreenCastSelectSourcesOptions_OmitsTheRestoreTokenWhenThereIsNothingToRestore
// keeps an empty string out of the options map, for the same reason the input
// grant does: a portal that validates the key it was given would refuse the
// call outright, turning the ordinary first run into a hard failure.
func TestScreenCastSelectSourcesOptions_OmitsTheRestoreTokenWhenThereIsNothingToRestore(
	t *testing.T,
) {
	options := screenCastSelectSourcesOptions("")

	if _, ok := options[portalRestoreTokenKey]; ok {
		t.Errorf("options carry a restore_token key with nothing to restore: %v", options)
	}
}

// portalStreamReply builds the streams entry of a Start reply the way godbus
// decodes one: a(ua{sv}) arrives as a slice of two-element slices, and each
// (ii) member as a slice of int32.
func portalStreamReply(entries ...[]any) map[string]dbus.Variant {
	return map[string]dbus.Variant{"streams": dbus.MakeVariant(entries)}
}

func portalStreamEntry(nodeID uint32, x, y, width, height int32) []any {
	return []any{
		nodeID,
		map[string]dbus.Variant{
			"position": dbus.MakeVariant([]any{x, y}),
			"size":     dbus.MakeVariant([]any{width, height}),
		},
	}
}

// TestDecodeScreenCastStreams_ReadsEachMonitorsPlaceOnScreen is what makes a
// captured frame placeable: the portal's position and size are the only source
// for where a streamed monitor sits, and without them a crop could not be
// mapped back to the caller's global coordinates.
func TestDecodeScreenCastStreams_ReadsEachMonitorsPlaceOnScreen(t *testing.T) {
	streams := decodeScreenCastStreams(portalStreamReply(
		portalStreamEntry(41, 0, 0, 1920, 1080),
		portalStreamEntry(42, 1920, 0, 1920, 1080),
	))

	want := twoMonitorStreams()
	if len(streams) != len(want) {
		t.Fatalf("decoded %d streams, want %d", len(streams), len(want))
	}

	for i, stream := range streams {
		if stream != want[i] {
			t.Errorf("stream %d = %+v, want %+v", i, stream, want[i])
		}
	}
}

// TestDecodeScreenCastStreams_KeepsAStreamWhoseGeometryIsMissing pins the
// choice between two ways of being wrong. A stream the portal named no position
// for is useless for placing a region, but dropping it would leave the caller
// with an empty list and the sentence "this session streams no monitor", which
// is a different problem with a different remedy. It is kept with zero bounds
// so the refusal names the geometry that is missing.
func TestDecodeScreenCastStreams_KeepsAStreamWhoseGeometryIsMissing(t *testing.T) {
	streams := decodeScreenCastStreams(portalStreamReply(
		[]any{uint32(7), map[string]dbus.Variant{}},
	))

	if len(streams) != 1 {
		t.Fatalf("decoded %d streams, want 1", len(streams))
	}

	if streams[0].nodeID != 7 {
		t.Errorf("nodeID = %d, want 7", streams[0].nodeID)
	}

	if !streams[0].bounds.Empty() {
		t.Errorf("bounds = %v, want the zero rectangle", streams[0].bounds)
	}
}

// TestDecodeScreenCastStreams_SurvivesAReplyItCannotRead keeps a malformed
// answer from panicking the capture path. Every field here comes off the bus
// as an interface value, so a portal backend that types one differently must
// produce an empty list rather than a type assertion that fails hard.
func TestDecodeScreenCastStreams_SurvivesAReplyItCannotRead(t *testing.T) {
	tests := []struct {
		name    string
		results map[string]dbus.Variant
	}{
		{name: "no streams key", results: map[string]dbus.Variant{}},
		{
			name:    "streams is not a list of structs",
			results: map[string]dbus.Variant{"streams": dbus.MakeVariant("nonsense")},
		},
		{name: "a struct with no members", results: portalStreamReply([]any{})},
		{
			name:    "a node id that is not a number",
			results: portalStreamReply([]any{"41", map[string]dbus.Variant{}}),
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if got := decodeScreenCastStreams(testCase.results); len(got) != 0 {
				t.Errorf("decoded %+v, want nothing", got)
			}
		})
	}
}

// screenCastAnswer builds a portal answer carrying a fresh restore token, for
// the restore-policy tests below.
func screenCastAnswer(token string) screenCastGrant {
	return screenCastGrant{
		streams:      twoMonitorStreams(),
		restoreToken: token,
	}
}

// TestRestorePortalGrant_NeverPromptsWhenNothingIsStored is the promise the
// capture path makes: it establishes a session out of a stored grant or it
// refuses, and it never reaches a handshake whose only possible outcome is a
// dialog. The opener is scripted to fail the test if it is called at all.
func TestRestorePortalGrant_NeverPromptsWhenNothingIsStored(t *testing.T) {
	store := &fakeTokenStore{}

	attempts := 0
	open := func(context.Context, string) (screenCastGrant, error) {
		attempts++

		return screenCastGrant{}, nil
	}

	_, err := restorePortalGrant(context.Background(), store, open)
	if err == nil {
		t.Fatal("restorePortalGrant() error = nil, want a refusal with nothing stored")
	}

	if attempts != 0 {
		t.Errorf("portal attempts = %d, want 0 — a prompt must never come from a capture",
			attempts)
	}

	if !derrors.IsNotSupported(err) {
		t.Errorf("error code = %q, want %q", derrors.GetCode(err), derrors.CodeNotSupported)
	}
}

// TestRestorePortalGrant_RestoresTheStoredGrantAndKeepsTheNextToken is the
// happy path, and it shares every line of its policy with the input grant: the
// stored token is presented, and the one the portal hands back replaces it —
// a restore token is invalidated by the use that consumes it.
func TestRestorePortalGrant_RestoresTheStoredGrantAndKeepsTheNextToken(t *testing.T) {
	store := &fakeTokenStore{token: storedToken}

	var presented []string

	open := func(_ context.Context, token string) (screenCastGrant, error) {
		presented = append(presented, token)

		return screenCastAnswer(nextToken), nil
	}

	grant, err := restorePortalGrant(context.Background(), store, open)
	if err != nil {
		t.Fatalf("restorePortalGrant() error = %v", err)
	}

	if len(grant.streams) != 2 {
		t.Errorf("grant carries %d streams, want 2", len(grant.streams))
	}

	if len(presented) != 1 || presented[0] != storedToken {
		t.Errorf("portal attempts = %q, want one presenting the stored token", presented)
	}

	if len(store.saved) != 1 || store.saved[0] != nextToken {
		t.Errorf("tokens saved = %q, want the token the grant returned", store.saved)
	}
}

// TestRestorePortalGrant_DropsATokenThePortalRefused keeps a dead credential
// from being replayed on every later capture.
func TestRestorePortalGrant_DropsATokenThePortalRefused(t *testing.T) {
	store := &fakeTokenStore{token: revokedToken}

	attempts := 0
	open := func(context.Context, string) (screenCastGrant, error) {
		attempts++

		return screenCastGrant{}, errNotRestorable
	}

	_, err := restorePortalGrant(context.Background(), store, open)
	if err == nil {
		t.Fatal("restorePortalGrant() error = nil, want the portal's refusal")
	}

	if attempts != 1 {
		t.Errorf("portal attempts = %d, want exactly 1 — no prompting retry here", attempts)
	}

	if store.cleared != 1 {
		t.Errorf("stored token cleared %d times, want 1", store.cleared)
	}
}

// TestRestorePortalGrant_KeepsATokenTheFailureWasNotAbout is the other side of
// that: a bus that could not be reached says nothing about the grant, and
// throwing the token away would cost the user a fresh consent dialog for a
// failure the token had no part in.
func TestRestorePortalGrant_KeepsATokenTheFailureWasNotAbout(t *testing.T) {
	store := &fakeTokenStore{token: storedToken}

	open := func(context.Context, string) (screenCastGrant, error) {
		return screenCastGrant{}, derrors.New(derrors.CodeTimeout, "no answer in time")
	}

	_, err := restorePortalGrant(context.Background(), store, open)
	if err == nil {
		t.Fatal("restorePortalGrant() error = nil, want the timeout")
	}

	if store.cleared != 0 {
		t.Errorf("stored token cleared %d times, want 0", store.cleared)
	}
}
