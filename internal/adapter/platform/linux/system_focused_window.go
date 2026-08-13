//go:build linux

package linux

import (
	"image"
	"os"

	"github.com/y3owk1n/neru/internal/adapter/platform/compositorcli"
	"github.com/y3owk1n/neru/internal/adapter/platform/kwin"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Wayland focused-window bounds. A Wayland client cannot query another client's
// on-screen geometry, so the focused window's global bounds come from whatever
// the running compositor exposes: Hyprland (`hyprctl activewindow`), Sway
// (`swaymsg -t get_tree`) and niri (`niri msg focused-window/-output`) each have
// a CLI, asked through
// [github.com/y3owk1n/neru/internal/adapter/platform/compositorcli]; KWin has
// none, and answers through the geometry script
// [github.com/y3owk1n/neru/internal/adapter/platform/kwin] installs — the same
// bridge the AT-SPI window-origin path reads, because it is the same fact.
// A compositor with none of those refuses rather than reporting no window, so a
// caller can tell "nobody to ask" from "nothing is focused", and a CLI that
// could not be run, refused or never answered says so for the same reason. Each
// query is bounded by a short timeout so a wedged compositor cannot stall hint
// activation.

// coordPair is the length of an [x,y] / [w,h] JSON array.
const coordPair = 2

// focusedWindowSource names where a session's focused-window geometry comes
// from. It exists so the routing decision can be read — and tested — without a
// compositor running: the sources differ in mechanism (a CLI, a KWin script)
// but the choice between them is one function of the backend and the
// environment.
type focusedWindowSource int

const (
	// focusedWindowSourceNone is a Wayland session with nothing to ask. It is
	// an answer, not a default: callers are told so rather than shown an
	// absent window.
	focusedWindowSourceNone focusedWindowSource = iota
	focusedWindowSourceKWin
	focusedWindowSourceNiri
	focusedWindowSourceSway
	focusedWindowSourceHyprland
)

// waylandFocusedWindowSource picks the geometry source for a Wayland session.
//
// The backend decides first and the environment only refines it. A compositor
// socket says which CLI is *reachable*, not which compositor this session runs:
// a KDE session that inherited SWAYSOCK from a user unit would otherwise be
// asked about a sway tree it has no window in.
func waylandFocusedWindowSource(backend string) focusedWindowSource {
	if backend == backendWaylandKDE {
		return focusedWindowSourceKWin
	}

	switch {
	case os.Getenv("NIRI_SOCKET") != "":
		return focusedWindowSourceNiri
	case os.Getenv("SWAYSOCK") != "":
		return focusedWindowSourceSway
	case os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "":
		return focusedWindowSourceHyprland
	default:
		return focusedWindowSourceNone
	}
}

// waylandFocusedWindowBounds returns the focused window's global bounds from
// the source its session exposes.
//
// A CodeNotSupported error means this session has no source to ask, which
// callers must not read as an unfocused desktop. Any other error means the
// source was asked and did not answer. found=false with a nil error is the one
// remaining case: the compositor answered, and its answer is that no window has
// an on-screen position right now — an unfocused desktop, or a niri window
// whose tiling gives it none.
func waylandFocusedWindowBounds(backend string) (image.Rectangle, bool, error) {
	switch waylandFocusedWindowSource(backend) {
	case focusedWindowSourceKWin:
		return kwinFocusedWindowBounds()
	case focusedWindowSourceNiri:
		return niriFocusedWindowBounds()
	case focusedWindowSourceSway:
		return swayFocusedWindowBounds()
	case focusedWindowSourceHyprland:
		return hyprlandFocusedWindowBounds()
	case focusedWindowSourceNone:
	}

	return image.Rectangle{}, false, derrors.New(
		derrors.CodeNotSupported,
		"no focused-window geometry source on linux backend "+backend+
			": this compositor exposes neither an IPC neru can query nor a scripting bridge",
	)
}

// warmFocusedWindowSource starts a session's geometry source before anything
// asks it anything, and is called once when the adapter is built.
//
// Only KDE has a source that must be installed before it can answer, and until
// it is installed the cache reports no window — which is indistinguishable from
// a focused desktop, so the first hint activation or `move_mouse --window` after
// startup would get the silent fallback this file exists to remove. Doing it at
// construction overlaps the install with the rest of daemon startup instead of
// with the user's first keystroke, and the installer retries a failed attempt
// on its own, so a session bus that was not up yet is usually resolved before
// anything asks rather than at the expense of whoever asks first.
//
// Nothing leaves this process on a session that is not running KWin: the
// install probes for it on the bus before it exports, owns a name or writes a
// script (#1430).
func warmFocusedWindowSource(backend string) {
	if backend != backendWaylandKDE {
		return
	}

	kwin.Shared(nil).EnsureStarted()
}

// kwinFocusedWindowBounds reads the geometry KWin last pushed to the shared
// cache, installing the script if nothing has installed it yet — including
// after an earlier attempt failed, so a bus that was not up at daemon start
// does not cost the whole run.
//
// The install is asynchronous by contract, so a call that arrives before KWin
// has answered reports no window rather than waiting: this runs under the mode
// handler's lock, where blocking on D-Bus would stop the keyboard. A call that
// arrives while an attempt is in flight gets the last completed attempt's
// answer for the same reason — and waiting for the in-flight one would not help
// it anyway, since a script that loads this instant still has to call back
// before there is a rectangle to report.
//
// A script that could not be installed is CodeNotSupported rather than a
// failure, because that is what the caller has to do about it: this session
// cannot report focused-window geometry at all, the same answer a compositor
// with no source gives, and the sentence names why. Reporting it as an
// accessibility failure would send the user to check a permission.
func kwinFocusedWindowBounds() (image.Rectangle, bool, error) {
	geometry := kwin.Shared(nil)
	geometry.EnsureStarted()

	rect, found, err := geometry.Bounds()
	if err != nil {
		return image.Rectangle{}, false, derrors.Wrap(
			err,
			derrors.CodeNotSupported,
			"the KWin focused-window geometry script is not installed",
		)
	}

	return rect, found, nil
}

// hyprlandFocusedWindowBounds reads `hyprctl -j activewindow`, whose "at" [x,y]
// and "size" [w,h] are already absolute screen pixels.
//
// The `-j` is what keeps an empty desktop an answer here: asked for JSON,
// Hyprland reports no active window as `{}`, and only its plain-text format
// answers with the word "Invalid" — which would decode as nothing and be
// reported as a failed query on a session that is merely idle.
func hyprlandFocusedWindowBounds() (image.Rectangle, bool, error) {
	var win struct {
		At   []int `json:"at"`
		Size []int `json:"size"`
	}

	err := compositorcli.Query(&win, "hyprctl", "-j", "activewindow")
	if err != nil {
		return image.Rectangle{}, false, err
	}

	if len(win.At) < coordPair || len(win.Size) < coordPair || win.Size[0] <= 0 ||
		win.Size[1] <= 0 {
		return image.Rectangle{}, false, nil
	}

	return image.Rect(win.At[0], win.At[1], win.At[0]+win.Size[0], win.At[1]+win.Size[1]), true, nil
}

type swayGeomRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type swayGeomNode struct {
	Focused       bool           `json:"focused"`
	Rect          swayGeomRect   `json:"rect"`
	Nodes         []swayGeomNode `json:"nodes"`
	FloatingNodes []swayGeomNode `json:"floating_nodes"` //nolint:tagliatelle // sway wire format is snake_case.
}

func swayFindFocused(node *swayGeomNode) *swayGeomNode {
	if node.Focused {
		return node
	}

	for i := range node.Nodes {
		if hit := swayFindFocused(&node.Nodes[i]); hit != nil {
			return hit
		}
	}

	for i := range node.FloatingNodes {
		if hit := swayFindFocused(&node.FloatingNodes[i]); hit != nil {
			return hit
		}
	}

	return nil
}

// swayFocusedWindowBounds reads `swaymsg -t get_tree` and returns the focused
// node's absolute geometry (rect), which is already in global coordinates.
func swayFocusedWindowBounds() (image.Rectangle, bool, error) {
	var tree swayGeomNode

	err := compositorcli.Query(&tree, "swaymsg", "-t", "get_tree")
	if err != nil {
		return image.Rectangle{}, false, err
	}

	focused := swayFindFocused(&tree)
	if focused == nil || focused.Rect.Width <= 0 || focused.Rect.Height <= 0 {
		return image.Rectangle{}, false, nil
	}

	return image.Rect(
		focused.Rect.X,
		focused.Rect.Y,
		focused.Rect.X+focused.Rect.Width,
		focused.Rect.Y+focused.Rect.Height,
	), true, nil
}

// niriFocusedWindowBounds combines `niri msg -j focused-window` (window size and
// tile position within the workspace view) with `niri msg -j focused-output`
// (the output's logical origin).
//
// tile_pos_in_workspace_view is only populated for floating windows
// (niri#2381), so a tiled window — the ordinary case on niri — reports
// not-found with no error. niri answered; the answer is that this window has no
// on-screen position, which is not a failure and must not be reported as one.
// The output query is skipped there, since there is nothing left to add an
// origin to.
func niriFocusedWindowBounds() (image.Rectangle, bool, error) {
	var win struct {
		Layout struct {
			WindowSize             []int     `json:"window_size"`                //nolint:tagliatelle // niri wire format is snake_case.
			TilePosInWorkspaceView []float64 `json:"tile_pos_in_workspace_view"` //nolint:tagliatelle // niri wire format is snake_case.
		} `json:"layout"`
	}

	winErr := compositorcli.Query(&win, "niri", "msg", "-j", "focused-window")
	if winErr != nil {
		return image.Rectangle{}, false, winErr
	}

	tile := win.Layout.TilePosInWorkspaceView
	if len(tile) < coordPair || len(win.Layout.WindowSize) < coordPair ||
		win.Layout.WindowSize[0] <= 0 || win.Layout.WindowSize[1] <= 0 {
		return image.Rectangle{}, false, nil
	}

	var out struct {
		Logical struct {
			X int `json:"x"`
			Y int `json:"y"`
		} `json:"logical"`
	}

	outErr := compositorcli.Query(&out, "niri", "msg", "-j", "focused-output")
	if outErr != nil {
		return image.Rectangle{}, false, outErr
	}

	originX := out.Logical.X + int(tile[0])
	originY := out.Logical.Y + int(tile[1])

	return image.Rect(
		originX,
		originY,
		originX+win.Layout.WindowSize[0],
		originY+win.Layout.WindowSize[1],
	), true, nil
}
