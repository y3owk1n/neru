//go:build linux

package gnomeshell

import (
	"errors"
	"image"
	"testing"

	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/unix"
)

const testOwner = ":1.5"

func TestWindowFromBody_ReadsTheStateTuple(t *testing.T) {
	tests := []struct {
		name      string
		body      []any
		wantOK    bool
		wantFound bool
		want      Window
	}{
		{
			name: "a focused window",
			body: []any{
				true,
				"org.gnome.Nautilus",
				"Home",
				int32(10),
				int32(20),
				int32(300),
				int32(400),
			},
			wantOK:    true,
			wantFound: true,
			want: Window{
				Rect:  image.Rect(10, 20, 310, 420),
				AppID: "org.gnome.Nautilus",
				Title: "Home",
			},
		},
		{
			name:   "nothing focused",
			body:   []any{false, "", "", int32(0), int32(0), int32(0), int32(0)},
			wantOK: true,
		},
		{name: "too few arguments", body: []any{true, "a", "b"}},
		{name: "wrong types", body: []any{true, "a", "b", 1, 2, 3, 4}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, found, wellFormed := windowFromBody(testCase.body)
			if wellFormed != testCase.wantOK || found != testCase.wantFound {
				t.Fatalf("windowFromBody() ok=%v found=%v, want ok=%v found=%v",
					wellFormed, found, testCase.wantOK, testCase.wantFound)
			}

			if wellFormed && got != testCase.want {
				t.Fatalf("windowFromBody() = %+v, want %+v", got, testCase.want)
			}
		})
	}
}

func TestOwnerFrom_ReadsOnlyTheExtensionsName(t *testing.T) {
	tests := []struct {
		name      string
		signal    *dbus.Signal
		wantOwner string
		wantOK    bool
	}{
		{name: "nil signal"},
		{name: "another name", signal: &dbus.Signal{Body: []any{"org.kde.KWin", "", testOwner}}},
		{
			name:      "arrival",
			signal:    &dbus.Signal{Body: []any{busName, "", testOwner}},
			wantOwner: testOwner,
			wantOK:    true,
		},
		{
			name:   "departure",
			signal: &dbus.Signal{Body: []any{busName, testOwner, ""}},
			wantOK: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			owner, wellFormed := ownerFrom(testCase.signal)
			if owner != testCase.wantOwner || wellFormed != testCase.wantOK {
				t.Fatalf(
					"ownerFrom() = (%q, %v), want (%q, %v)",
					owner,
					wellFormed,
					testCase.wantOwner,
					testCase.wantOK,
				)
			}
		})
	}
}

// A bridge nobody has connected reports that, not an unfocused desktop: the
// caller must be able to tell the two apart.
func TestBridge_Focused_ReportsNotConnectedBeforeTheFirstAttempt(t *testing.T) {
	bridge := newBridge(nil)

	_, found, err := bridge.Focused()
	if found || !errors.Is(err, errNotConnected) {
		t.Fatalf("Focused() = found=%v err=%v, want not connected", found, err)
	}
}

func TestBridge_Update_WakesTheFocusPipeAndClearsTheReason(t *testing.T) {
	bridge := newBridge(nil)

	eventFD, available := bridge.FocusEventFD()
	if !available {
		t.Fatal("FocusEventFD() unavailable")
	}

	window := Window{Rect: image.Rect(1, 2, 3, 4), AppID: "a"}
	bridge.update(window, true)

	got, found, err := bridge.Focused()
	if err != nil || !found || got != window {
		t.Fatalf("Focused() = (%+v, %v, %v), want the pushed window", got, found, err)
	}

	buf := make([]byte, 8)

	n, err := unix.Read(eventFD, buf)
	if err != nil || n == 0 {
		t.Fatalf("focus pipe read = (%d, %v), want a wake-up byte", n, err)
	}

	bridge.ownerChanged("")

	_, found, err = bridge.Focused()
	if found || !errors.Is(err, errExtensionAbsent) {
		t.Fatalf("Focused() after departure = found=%v err=%v, want extension absent", found, err)
	}
}
