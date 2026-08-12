//go:build linux

package atspi

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform"
)

// TestNewWindowOriginSourceStartsNoBridgeOffWayland pins the user-visible half
// of #1430: on a session the backend detector called X11, starting the
// window-origin source must touch nothing outside the process.
//
// Before the fix the source was picked from the compositor sockets alone, and a
// plain X11 session set none of them, so it fell through to the KWin bridge and
// started it: session bus, exported object, bus name, and a KWin script written
// into $XDG_RUNTIME_DIR on a desktop that runs no KWin.
func TestNewWindowOriginSourceStartsNoBridgeOffWayland(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)

	newWindowOriginSource(platform.BackendX11, zap.NewNop()).start()

	entries, readErr := os.ReadDir(runtimeDir)
	if readErr != nil {
		t.Fatalf("ReadDir(%s) error = %v", runtimeDir, readErr)
	}

	for _, entry := range entries {
		t.Errorf(
			"starting the X11 window-origin source wrote %s into XDG_RUNTIME_DIR; "+
				"no compositor bridge may start on a backend that did not identify it",
			entry.Name(),
		)
	}
}

// noOriginType is the type name newWindowOriginSource returns for a backend
// with no compositor to ask, spelled as %T prints it.
const noOriginType = "atspi.noWindowOrigin"

// swaySocket is a plausible SWAYSOCK value; nothing ever connects to it,
// because the selection under test reads only whether it is set.
const swaySocket = "/run/sway.sock"

// The compositor sockets the selection reads, named once so a case sets the
// same variable the reset loop clears.
const (
	niriSocketEnv        = "NIRI_SOCKET"
	swaySocketEnv        = "SWAYSOCK"
	hyprlandSignatureEnv = "HYPRLAND_INSTANCE_SIGNATURE"
)

// TestNewWindowOriginSourceFollowsTheBackend pins which source each backend
// gets, including the two orderings that used to go wrong: a compositor socket
// left in the environment of a session running something else never picks a
// source, and a backend with no source of its own reports no origin instead of
// falling through to the KWin bridge.
func TestNewWindowOriginSourceFollowsTheBackend(t *testing.T) {
	cases := []struct {
		name    string
		backend platform.LinuxBackend
		sockets map[string]string
		want    string
	}{
		{"x11 has no compositor to ask", platform.BackendX11, nil, noOriginType},
		{
			"x11 with a stale wlroots socket still asks nobody",
			platform.BackendX11,
			map[string]string{swaySocketEnv: swaySocket},
			noOriginType,
		},
		{"kde uses the KWin bridge", platform.BackendWaylandKDE, nil, "*atspi.kwinOriginSource"},
		{
			"kde ignores a wlroots socket",
			platform.BackendWaylandKDE,
			map[string]string{swaySocketEnv: swaySocket},
			"*atspi.kwinOriginSource",
		},
		{
			"wlroots picks niri by its socket",
			platform.BackendWaylandWlroots,
			map[string]string{niriSocketEnv: "/run/niri.sock"},
			"*atspi.niriOriginSource",
		},
		{
			"wlroots picks sway by its socket",
			platform.BackendWaylandWlroots,
			map[string]string{swaySocketEnv: swaySocket},
			"*atspi.swayOriginSource",
		},
		{
			"wlroots picks Hyprland by its signature",
			platform.BackendWaylandWlroots,
			map[string]string{hyprlandSignatureEnv: "abc"},
			"*atspi.hyprlandOriginSource",
		},
		{
			"a wlroots compositor with no IPC source reports no origin",
			platform.BackendWaylandWlroots,
			nil,
			noOriginType,
		},
		{"gnome has no source", platform.BackendWaylandGNOME, nil, noOriginType},
		{"other wayland has no source", platform.BackendWaylandOther, nil, noOriginType},
		{"an unknown backend has no source", platform.BackendUnknown, nil, noOriginType},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, name := range []string{niriSocketEnv, swaySocketEnv, hyprlandSignatureEnv} {
				t.Setenv(name, testCase.sockets[name])
			}

			got := fmt.Sprintf("%T", newWindowOriginSource(testCase.backend, zap.NewNop()))
			if got != testCase.want {
				t.Errorf(
					"newWindowOriginSource(%v) = %s, want %s",
					testCase.backend, got, testCase.want,
				)
			}
		})
	}
}

func TestNiriComputeOrigin(t *testing.T) {
	out := niriOutput{}
	out.Logical.X = 100
	out.Logical.Y = 10

	makeWindow := func(size []int, tile []float64) niriWindow {
		var w niriWindow

		w.Layout.WindowSize = size
		w.Layout.TilePosInWorkspaceView = tile

		return w
	}

	cases := []struct {
		name           string
		win            niriWindow
		frameW, frameH int
		wantX, wantY   int
		wantOK         bool
	}{
		{
			"floating window offset by output+tile",
			makeWindow([]int{946, 942}, []float64{958, 52}),
			946,
			942,
			1058,
			62,
			true,
		},
		{"tiled window has no tile_pos", makeWindow([]int{946, 942}, nil), 946, 942, 0, 0, false},
		{
			"size mismatch rejected",
			makeWindow([]int{946, 942}, []float64{958, 52}),
			500,
			942,
			0,
			0,
			false,
		},
		{
			"size within tolerance accepted",
			makeWindow([]int{946, 942}, []float64{958, 52}),
			950,
			940,
			1058,
			62,
			true,
		},
		{
			"missing window_size rejected",
			makeWindow(nil, []float64{958, 52}),
			946,
			942,
			0,
			0,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x, y, ok := niriComputeOrigin(tc.win, out, tc.frameW, tc.frameH, zap.NewNop())
			if ok != tc.wantOK || (ok && (x != tc.wantX || y != tc.wantY)) {
				t.Errorf("got (%d,%d,%v), want (%d,%d,%v)", x, y, ok, tc.wantX, tc.wantY, tc.wantOK)
			}
		})
	}
}

func TestHyprlandComputeOrigin(t *testing.T) {
	cases := []struct {
		name           string
		win            hyprlandWindow
		frameW, frameH int
		wantX, wantY   int
		wantOK         bool
	}{
		{
			"active window at/size",
			hyprlandWindow{At: []int{958, 52}, Size: []int{946, 942}},
			946,
			942,
			958,
			52,
			true,
		},
		{
			"size mismatch rejected",
			hyprlandWindow{At: []int{958, 52}, Size: []int{946, 942}},
			500,
			500,
			0,
			0,
			false,
		},
		{"missing at rejected", hyprlandWindow{Size: []int{946, 942}}, 946, 942, 0, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x, y, ok := hyprlandComputeOrigin(tc.win, tc.frameW, tc.frameH, zap.NewNop())
			if ok != tc.wantOK || (ok && (x != tc.wantX || y != tc.wantY)) {
				t.Errorf("got (%d,%d,%v), want (%d,%d,%v)", x, y, ok, tc.wantX, tc.wantY, tc.wantOK)
			}
		})
	}
}

// swayTreeJSON is a trimmed `swaymsg -t get_tree` with a focused window nested
// under output → workspace → container, plus a decoration offset in window_rect.
const swayTreeJSON = `{
  "focused": false, "rect": {"x":0,"y":0,"width":1920,"height":1080},
  "window_rect": {"x":0,"y":0,"width":0,"height":0},
  "nodes": [
    {"focused": false, "rect": {"x":960,"y":0,"width":960,"height":1080},
     "window_rect": {"x":0,"y":0,"width":0,"height":0},
     "nodes": [
       {"focused": true, "rect": {"x":960,"y":0,"width":960,"height":1080},
        "window_rect": {"x":2,"y":24,"width":956,"height":1052}, "nodes": [], "floating_nodes": []}
     ], "floating_nodes": []}
  ], "floating_nodes": []
}`

func TestSwayComputeOrigin(t *testing.T) {
	var tree swayNode

	err := json.Unmarshal([]byte(swayTreeJSON), &tree)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Content origin = rect(960,0) + window_rect(2,24) = (962,24); content size
	// = window_rect 956x1052.
	x, y, ok := swayComputeOrigin(&tree, 956, 1052, zap.NewNop())
	if !ok || x != 962 || y != 24 {
		t.Fatalf("focused content origin: got (%d,%d,%v), want (962,24,true)", x, y, ok)
	}

	// Size mismatch against the content size is rejected.
	if _, _, ok := swayComputeOrigin(&tree, 500, 500, zap.NewNop()); ok {
		t.Fatal("expected size mismatch to be rejected")
	}
}

func TestSwayFindFocusedNoneFocused(t *testing.T) {
	tree := swayNode{Focused: false, Nodes: []swayNode{{Focused: false}}}
	if findFocused(&tree) != nil {
		t.Fatal("expected no focused node")
	}
}
