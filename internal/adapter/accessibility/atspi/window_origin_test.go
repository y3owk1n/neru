//go:build linux

package atspi

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/derrors"
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

// fakeCLIMode is what a fake compositor CLI is written with: executable by its
// owner and nobody else.
const fakeCLIMode = 0o700

// fakeCompositorCLIs points PATH at a directory holding only the compositor
// CLIs a case names, each a shell script printing what that case wants the
// compositor to say. A CLI the map omits is one this session does not have
// installed, which is a case in its own right.
//
// The same scaffolding stands in
// internal/adapter/platform/linux/system_focused_window_test.go, and stays
// duplicated on purpose: sharing it between two test packages in different
// trees would mean a test-only package in the production tree, which is a
// larger thing to own than sixty lines that answer to a compiler.
func fakeCompositorCLIs(t *testing.T, scripts map[string]string) {
	t.Helper()

	dir := t.TempDir()

	for name, body := range scripts {
		path := filepath.Join(dir, name)

		writeErr := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), fakeCLIMode)
		if writeErr != nil {
			t.Fatalf("writing the fake %s: %v", name, writeErr)
		}
	}

	t.Setenv("PATH", dir)
}

// frameW and frameH are the AT-SPI frame extents every origin case is checked
// against, matching the window size the fake compositors report.
const (
	frameW = 946
	frameH = 942
)

// The compositor CLIs a case installs a fake of, and the body of one that
// cannot reach its compositor.
const (
	cliHyprctl = "hyprctl"
	cliSwaymsg = "swaymsg"
	cliNiri    = "niri"

	cliExitsWithAnError = "exit 1"
)

// TestWindowOriginSource_ReportsAFailedQueryInsteadOfDroppingTheOffset is the
// AT-SPI half of #1493.
//
// A compositor CLI that is missing, refusing or wedged used to be
// indistinguishable from a compositor that answered with no origin, and both
// silently dropped the offset — which on Wayland means every hint is drawn at
// its window-relative coordinates, so a screenful of them lands in the wrong
// place with nothing said. Only the failure is a failure; a compositor that
// answered and has no origin to give is the ordinary degradation this path was
// built for.
func TestWindowOriginSource_ReportsAFailedQueryInsteadOfDroppingTheOffset(t *testing.T) {
	const niriFloating = `{"layout":{"window_size":[946,942],` +
		`"tile_pos_in_workspace_view":[958,52]}}`

	// wantNamed is what a case expects the refusal to name — the CLI that
	// failed, or the backend that has nothing to ask — and its absence means
	// the case expects no refusal at all.
	cases := []struct {
		name      string
		source    func(*zap.Logger) windowOriginSource
		scripts   map[string]string
		wantPoint image.Point
		wantOK    bool
		wantNamed string
	}{
		{
			name:      "hyprland reports the active window's position",
			source:    func(l *zap.Logger) windowOriginSource { return newHyprlandOriginSource(l) },
			scripts:   map[string]string{cliHyprctl: echoJSON(`{"at":[958,52],"size":[946,942]}`)},
			wantPoint: image.Pt(958, 52),
			wantOK:    true,
		},
		{
			name:      "a missing hyprctl is a failure, not a missing origin",
			source:    func(l *zap.Logger) windowOriginSource { return newHyprlandOriginSource(l) },
			wantNamed: cliHyprctl,
		},
		{
			name:    "a hyprctl that answered nothing usable is not a failure",
			source:  func(l *zap.Logger) windowOriginSource { return newHyprlandOriginSource(l) },
			scripts: map[string]string{cliHyprctl: echoJSON(`{}`)},
		},
		{
			name:      "sway reports the focused node's content origin",
			source:    func(l *zap.Logger) windowOriginSource { return newSwayOriginSource(l) },
			scripts:   map[string]string{cliSwaymsg: echoJSON(swayOriginTreeJSON)},
			wantPoint: image.Pt(958, 52),
			wantOK:    true,
		},
		{
			name:      "a swaymsg that could not reach the compositor is a failure",
			source:    func(l *zap.Logger) windowOriginSource { return newSwayOriginSource(l) },
			scripts:   map[string]string{cliSwaymsg: cliExitsWithAnError},
			wantNamed: cliSwaymsg,
		},
		{
			name:      "niri reports a floating window's origin",
			source:    func(l *zap.Logger) windowOriginSource { return newNiriOriginSource(l) },
			scripts:   niriCLI(niriFloating, `{"logical":{"x":100,"y":10}}`),
			wantPoint: image.Pt(1058, 62),
			wantOK:    true,
		},
		{
			name:      "a niri that answered with something undecodable is a failure",
			source:    func(l *zap.Logger) windowOriginSource { return newNiriOriginSource(l) },
			scripts:   map[string]string{cliNiri: "echo 'not json'"},
			wantNamed: cliNiri,
		},
		{
			// X11 is the one session where having no origin is the complete
			// answer: AT-SPI already reports screen coordinates there, so
			// nothing is missing and nothing failed.
			name: "x11 needs no origin and reports no failure",
			source: func(l *zap.Logger) windowOriginSource {
				return newNoWindowOrigin(l, platform.BackendX11)
			},
		},
		{
			// A Wayland session with no source is the opposite: hints are about
			// to be placed window-relative because there is nobody to ask, and
			// that has to be distinguishable from a compositor that answered.
			name: "a wayland backend with no source refuses and names itself",
			source: func(l *zap.Logger) windowOriginSource {
				return newNoWindowOrigin(l, platform.BackendWaylandGNOME)
			},
			wantNamed: platform.BackendWaylandGNOME.String(),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fakeCompositorCLIs(t, testCase.scripts)

			origin, known, err := testCase.source(zap.NewNop()).
				originFor(windowFrame{Width: frameW, Height: frameH})

			if testCase.wantNamed != "" {
				if err == nil {
					t.Fatalf("originFor() = (%v, %v, nil); a query that never "+
						"happened must not read as a window with no origin",
						origin, known)
				}

				if !strings.Contains(derrors.Message(err), testCase.wantNamed) {
					t.Errorf("error %q does not name %s, so a reader cannot tell "+
						"what could not answer", derrors.Message(err), testCase.wantNamed)
				}

				return
			}

			if err != nil {
				t.Fatalf("originFor() error = %v, want none", err)
			}

			if known != testCase.wantOK || (known && origin != testCase.wantPoint) {
				t.Errorf("originFor() = (%v, %v), want (%v, %v)",
					origin, known, testCase.wantPoint, testCase.wantOK)
			}
		})
	}
}

// TestClient_ReportOriginFailure_SaysOneSteadyReasonOnce keeps an honest
// contract from becoming a nuisance.
//
// A window-origin failure is almost always a property of the session, not of
// this activation: a compositor CLI that is not installed stays not installed.
// Warning on every hint activation would repeat one fact hundreds of times a
// day, which is how a log stops being read — so the reason is said out loud
// once, again when it changes, and again after the source has worked in
// between.
func TestClient_ReportOriginFailure_SaysOneSteadyReasonOnce(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	client := &Client{logger: zap.New(core)}

	missing := derrors.New(derrors.CodeBridgeFailed, "hyprctl could not be run")
	refused := derrors.New(derrors.CodeBridgeFailed, "hyprctl exited with an error")

	warnCount := func() int { return logs.FilterLevelExact(zapcore.WarnLevel).Len() }

	client.reportOriginFailure(missing)
	client.reportOriginFailure(missing)

	if got := warnCount(); got != 1 {
		t.Fatalf("a steady reason warned %d times, want once", got)
	}

	if logs.FilterLevelExact(zapcore.DebugLevel).Len() == 0 {
		t.Error("the repeat was not recorded at all; it still happened")
	}

	client.reportOriginFailure(refused)

	if got := warnCount(); got != 2 {
		t.Fatalf("a new reason warned %d times in total, want 2 — a different "+
			"failure is a different event", got)
	}

	client.clearOriginFailure()
	client.reportOriginFailure(refused)

	if got := warnCount(); got != 3 {
		t.Errorf("a failure after the source had worked warned %d times in total, "+
			"want 3 — it broke again and nobody was told", got)
	}
}

// TestNiriOriginSource_KeepsATiledWindowAnAnswer pins niri's exception on the
// hint-placement path too.
//
// tile_pos_in_workspace_view is populated for floating windows only
// (niri#2381), so a tiled window has no on-screen position and hints fall back
// to window-relative coordinates. That is the compositor answering, and warning
// about it would fire on every activation on the compositor's ordinary layout.
// The output query is not even made: nothing is left to add an origin to, so a
// broken one here — wired to fail below — must change nothing.
func TestNiriOriginSource_KeepsATiledWindowAnAnswer(t *testing.T) {
	fakeCompositorCLIs(t, niriCLI(`{"layout":{"window_size":[946,942]}}`, ""))

	origin, ok, err := newNiriOriginSource(zap.NewNop()).
		originFor(windowFrame{Width: frameW, Height: frameH})
	if ok || err != nil {
		t.Fatalf("originFor() = (%v, %v, %v), want no origin and no error — a "+
			"tiled window has none to give, and saying so must not warn on every "+
			"activation", origin, ok, err)
	}
}

// echoJSON makes a fake CLI print the given JSON. It goes through the shell's
// own echo rather than anything on disk, because PATH holds nothing but the
// fake CLIs themselves.
func echoJSON(body string) string {
	return "echo '" + body + "'"
}

// niriCLI is a fake `niri msg -j <what>` answering the two subcommands the
// origin query asks. An empty answer makes that subcommand exit non-zero, which
// is how a case proves a query was made — or never made.
func niriCLI(focusedWindow, focusedOutput string) map[string]string {
	answer := func(body string) string {
		if body == "" {
			return cliExitsWithAnError
		}

		return echoJSON(body)
	}

	return map[string]string{cliNiri: strings.Join([]string{
		`case "$3" in`,
		"focused-window) " + answer(focusedWindow) + " ;;",
		"focused-output) " + answer(focusedOutput) + " ;;",
		"*) exit 1 ;;",
		"esac",
	}, "\n")}
}

// swayOriginTreeJSON is a trimmed `swaymsg -t get_tree` whose focused node's
// content origin — rect plus window_rect — is (958,52) with a 946x942 content
// area, matching the frame extents the cases check against.
const swayOriginTreeJSON = `{"focused":false,"rect":{"x":0,"y":0,"width":1920,"height":1080},
 "nodes":[{"focused":true,"rect":{"x":956,"y":28,"width":960,"height":1000},
 "window_rect":{"x":2,"y":24,"width":946,"height":942},
 "nodes":[],"floating_nodes":[]}],"floating_nodes":[]}`

// TestNiriOriginTile covers the gate every niri case turns on — whether niri
// gave this window a position at all — and the arithmetic that places the
// position it did give.
func TestNiriOriginTile(t *testing.T) {
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

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tileX, tileY, floating := niriOriginTile(
				testCase.win, testCase.frameW, testCase.frameH, zap.NewNop(),
			)
			if floating != testCase.wantOK {
				t.Fatalf("niriOriginTile() ok = %v, want %v", floating, testCase.wantOK)
			}

			if !floating {
				return
			}

			origin := niriComputeOrigin(out, tileX, tileY)
			if origin != image.Pt(testCase.wantX, testCase.wantY) {
				t.Errorf("got %v, want (%d,%d)", origin, testCase.wantX, testCase.wantY)
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

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			origin, known := hyprlandComputeOrigin(
				testCase.win, testCase.frameW, testCase.frameH, zap.NewNop(),
			)

			want := image.Pt(testCase.wantX, testCase.wantY)
			if known != testCase.wantOK || (known && origin != want) {
				t.Errorf("got (%v, %v), want (%v, %v)", origin, known, want, testCase.wantOK)
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
	origin, known := swayComputeOrigin(&tree, 956, 1052, zap.NewNop())
	if !known || origin != image.Pt(962, 24) {
		t.Fatalf("focused content origin: got (%v, %v), want ((962,24), true)", origin, known)
	}

	// Size mismatch against the content size is rejected.
	if _, known := swayComputeOrigin(&tree, 500, 500, zap.NewNop()); known {
		t.Fatal("expected size mismatch to be rejected")
	}
}

func TestSwayFindFocusedNoneFocused(t *testing.T) {
	tree := swayNode{Focused: false, Nodes: []swayNode{{Focused: false}}}
	if findFocused(&tree) != nil {
		t.Fatal("expected no focused node")
	}
}
