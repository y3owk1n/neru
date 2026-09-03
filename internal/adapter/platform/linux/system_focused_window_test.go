//go:build linux

package linux

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The compositor sockets the wlroots arm reads, named once so a case sets the
// same variable the reset loop clears.
const (
	niriSocketEnv        = "NIRI_SOCKET"
	swaySocketEnv        = "SWAYSOCK"
	hyprlandSignatureEnv = "HYPRLAND_INSTANCE_SIGNATURE"
)

// TestWaylandFocusedWindowSource_FollowsTheBackend pins which source answers
// for each session, including the two orderings that were wrong before: KDE
// fell through the socket switch to nothing at all, and a KDE session that
// inherited a stale wlroots socket asked that compositor's CLI about a window
// it has never seen.
func TestWaylandFocusedWindowSource_FollowsTheBackend(t *testing.T) {
	cases := []struct {
		name    string
		backend string
		sockets map[string]string
		want    focusedWindowSource
	}{
		{"kde asks the KWin bridge", backendWaylandKDE, nil, focusedWindowSourceKWin},
		{
			"kde ignores a stale wlroots socket",
			backendWaylandKDE,
			map[string]string{swaySocketEnv: "/run/sway.sock"},
			focusedWindowSourceKWin,
		},
		{
			"wlroots picks niri by its socket",
			backendWaylandWlroots,
			map[string]string{niriSocketEnv: "/run/niri.sock"},
			focusedWindowSourceNiri,
		},
		{
			"wlroots picks sway by its socket",
			backendWaylandWlroots,
			map[string]string{swaySocketEnv: "/run/sway.sock"},
			focusedWindowSourceSway,
		},
		{
			"wlroots picks Hyprland by its signature",
			backendWaylandWlroots,
			map[string]string{hyprlandSignatureEnv: "abc"},
			focusedWindowSourceHyprland,
		},
		{
			"a wlroots compositor with no IPC has no source",
			backendWaylandWlroots,
			nil,
			focusedWindowSourceNone,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, name := range []string{niriSocketEnv, swaySocketEnv, hyprlandSignatureEnv} {
				t.Setenv(name, testCase.sockets[name])
			}

			got := waylandFocusedWindowSource(testCase.backend)
			if got != testCase.want {
				t.Errorf("waylandFocusedWindowSource(%q) = %v, want %v",
					testCase.backend, got, testCase.want)
			}
		})
	}
}

// fakeCLIMode is what a fake compositor CLI is written with: executable by its
// owner and nobody else.
const fakeCLIMode = 0o700

// The compositor CLIs a case installs a fake of, and the body of one that
// cannot reach its compositor.
const (
	cliHyprctl = "hyprctl"
	cliSwaymsg = "swaymsg"
	cliNiri    = "niri"

	cliExitsWithAnError = "exit 1"
)

// fakeCompositorCLIs points PATH at a directory holding only the compositor
// CLIs a case names, each a shell script printing what that case wants the
// compositor to say. A CLI the map omits is one this session does not have
// installed, which is a case in its own right.
//
// Spawning a process is the mechanism under test rather than an implementation
// detail behind it: what separates "the compositor answered, and nothing is
// focused" from "the query never happened" is entirely in how that process
// behaved, and a stubbed decoder would pin neither.
//
// The same scaffolding stands in
// internal/adapter/accessibility/atspi/window_origin_test.go, and stays
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

// boundsAnswer is one focused-window answer, so a case can be compared with
// what it expected in one place.
type boundsAnswer struct {
	rect  image.Rectangle
	found bool
	err   error
}

// assertBoundsAnswer holds an arm to the contract all of them share: a failed
// query is an error naming the CLI that failed, and everything else is either a
// rectangle or a plain not-found.
func assertBoundsAnswer(t *testing.T, cli string, got, want boundsAnswer, wantErr bool) {
	t.Helper()

	if wantErr {
		if got.err == nil {
			t.Fatalf("got (%v, %v, nil); a failed query must not read as a "+
				"desktop with nothing focused", got.rect, got.found)
		}

		if got.found {
			t.Errorf("got found=true alongside the error %v", got.err)
		}

		if !strings.Contains(derrors.Message(got.err), cli) {
			t.Errorf("error %q does not name %s, so a reader cannot tell which "+
				"compositor failed", derrors.Message(got.err), cli)
		}

		return
	}

	if got.err != nil {
		t.Fatalf("got error %v, want a plain answer", got.err)
	}

	if got.found != want.found || (want.found && got.rect != want.rect) {
		t.Errorf("got (%v, %v), want (%v, %v)", got.rect, got.found, want.rect, want.found)
	}
}

// TestHyprlandFocusedWindowBounds_SeparatesAFailedQueryFromAnEmptyDesktop is
// the shape #1493 is about, on the arm with the simplest CLI.
//
// A hyprctl that is missing, wedged or broken used to answer exactly what a
// desktop with nothing focused answers — not-found with no error — so callers
// widened to the whole active screen and nothing said why.
func TestHyprlandFocusedWindowBounds_SeparatesAFailedQueryFromAnEmptyDesktop(t *testing.T) {
	cases := []struct {
		name      string
		scripts   map[string]string
		wantRect  image.Rectangle
		wantFound bool
		wantErr   bool
	}{
		{
			name:      "a focused window is reported",
			scripts:   map[string]string{cliHyprctl: echoJSON(`{"at":[958,52],"size":[946,942]}`)},
			wantRect:  image.Rect(958, 52, 1904, 994),
			wantFound: true,
		},
		{
			name:    "nothing focused is an answer, not a failure",
			scripts: map[string]string{cliHyprctl: echoJSON(`{}`)},
		},
		{
			name:    "hyprctl is not installed",
			scripts: nil,
			wantErr: true,
		},
		{
			name:    "hyprctl exited with an error",
			scripts: map[string]string{cliHyprctl: cliExitsWithAnError},
			wantErr: true,
		},
		{
			name:    "hyprctl answered with something undecodable",
			scripts: map[string]string{cliHyprctl: "echo 'Invalid request'"},
			wantErr: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fakeCompositorCLIs(t, testCase.scripts)

			rect, found, err := hyprlandFocusedWindowBounds()

			assertBoundsAnswer(t, cliHyprctl,
				boundsAnswer{rect, found, err},
				boundsAnswer{rect: testCase.wantRect, found: testCase.wantFound},
				testCase.wantErr)
		})
	}
}

// swayTreeWithFocus is a trimmed `swaymsg -t get_tree` whose focused node sits
// two levels down, so finding it means descending.
const swayTreeWithFocus = `{"focused":false,"rect":{"x":0,"y":0,"width":1920,"height":1080},
 "nodes":[{"focused":false,"rect":{"x":960,"y":0,"width":960,"height":1080},
 "nodes":[{"focused":true,"rect":{"x":960,"y":0,"width":960,"height":1080},
 "nodes":[],"floating_nodes":[]}],"floating_nodes":[]}],"floating_nodes":[]}`

// swayTreeWithNoFocus is the same tree with nothing focused in it — a real
// answer from a working compositor.
const swayTreeWithNoFocus = `{"focused":false,"rect":{"x":0,"y":0,"width":1920,"height":1080},
 "nodes":[],"floating_nodes":[]}`

// echoJSON makes a fake CLI print the given JSON. It goes through the shell's
// own echo rather than anything on disk, because PATH holds nothing but the
// fake CLIs themselves.
func echoJSON(body string) string {
	return "echo '" + body + "'"
}

// TestSwayFocusedWindowBounds_SeparatesAFailedQueryFromAnEmptyDesktop pins the
// same separation on Sway, where the empty answer is a whole tree with no
// focused node in it rather than an empty object.
func TestSwayFocusedWindowBounds_SeparatesAFailedQueryFromAnEmptyDesktop(t *testing.T) {
	cases := []struct {
		name      string
		scripts   map[string]string
		wantRect  image.Rectangle
		wantFound bool
		wantErr   bool
	}{
		{
			name:      "the focused node's rectangle is reported",
			scripts:   map[string]string{cliSwaymsg: echoJSON(swayTreeWithFocus)},
			wantRect:  image.Rect(960, 0, 1920, 1080),
			wantFound: true,
		},
		{
			name:    "a tree with nothing focused is an answer",
			scripts: map[string]string{cliSwaymsg: echoJSON(swayTreeWithNoFocus)},
		},
		{
			name:    "swaymsg is not installed",
			scripts: nil,
			wantErr: true,
		},
		{
			name:    "swaymsg could not reach the compositor",
			scripts: map[string]string{cliSwaymsg: cliExitsWithAnError},
			wantErr: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fakeCompositorCLIs(t, testCase.scripts)

			rect, found, err := swayFocusedWindowBounds()

			assertBoundsAnswer(t, cliSwaymsg,
				boundsAnswer{rect, found, err},
				boundsAnswer{rect: testCase.wantRect, found: testCase.wantFound},
				testCase.wantErr)
		})
	}
}

// niriCLI is a fake `niri msg -j <what>` answering the two subcommands the
// bounds query asks, each case supplying what it wants said. An empty answer
// makes that subcommand exit non-zero, which is how a case proves a query was
// made — or never made.
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

// TestNiriFocusedWindowBounds_KeepsATiledWindowAnAnswer is the constraint that
// decides the shape of this whole change.
//
// niri populates tile_pos_in_workspace_view for floating windows only
// (niri#2381), so a tiled window — the ordinary case on niri — genuinely has no
// on-screen position to report. That is the compositor answering, not a query
// that failed, and turning it into an error would warn a niri user on every
// activation. The second query is not made either: there is nothing left for an
// output origin to be added to.
func TestNiriFocusedWindowBounds_KeepsATiledWindowAnAnswer(t *testing.T) {
	// focused-output is wired to fail, so an error here would mean the tiled
	// answer had not stopped the query before it.
	fakeCompositorCLIs(t, niriCLI(`{"layout":{"window_size":[946,942]}}`, ""))

	rect, found, err := niriFocusedWindowBounds()
	if found || err != nil {
		t.Fatalf("niriFocusedWindowBounds() = (%v, %v, %v), want not-found with no "+
			"error — a tiled window has no on-screen position, and saying so must "+
			"not warn on every activation", rect, found, err)
	}
}

// TestNiriFocusedWindowBounds_SeparatesAFailedQueryFromAnEmptyDesktop pins the
// rest of the niri arm around that constraint: a floating window is reported,
// an unreachable niri is a failure, and neither of the two queries may swallow
// one.
func TestNiriFocusedWindowBounds_SeparatesAFailedQueryFromAnEmptyDesktop(t *testing.T) {
	const floating = `{"layout":{"window_size":[946,942],` +
		`"tile_pos_in_workspace_view":[958,52]}}`

	const output = `{"logical":{"x":100,"y":10}}`

	cases := []struct {
		name      string
		scripts   map[string]string
		wantRect  image.Rectangle
		wantFound bool
		wantErr   bool
	}{
		{
			name:      "a floating window sits at the output origin plus its tile",
			scripts:   niriCLI(floating, output),
			wantRect:  image.Rect(1058, 62, 2004, 1004),
			wantFound: true,
		},
		{
			name:    "no window focused at all is an answer",
			scripts: niriCLI(`null`, output),
		},
		{
			name:    "niri is not installed",
			scripts: nil,
			wantErr: true,
		},
		{
			name:    "the focused-window query failed",
			scripts: niriCLI("", output),
			wantErr: true,
		},
		{
			name:    "the focused-output query failed",
			scripts: niriCLI(floating, ""),
			wantErr: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fakeCompositorCLIs(t, testCase.scripts)

			rect, found, err := niriFocusedWindowBounds()

			assertBoundsAnswer(t, cliNiri,
				boundsAnswer{rect, found, err},
				boundsAnswer{rect: testCase.wantRect, found: testCase.wantFound},
				testCase.wantErr)
		})
	}
}
