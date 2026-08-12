//go:build linux

package linux

import (
	"context"
	"encoding/json"
	"image"
	"os"
	"os/exec"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/platform/kwin"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Wayland focused-window bounds. A Wayland client cannot query another client's
// on-screen geometry, so the focused window's global bounds come from whatever
// the running compositor exposes: Hyprland (`hyprctl activewindow`), Sway
// (`swaymsg -t get_tree`) and niri (`niri msg focused-window/-output`) each have
// a CLI; KWin has none, and answers through the geometry script
// [github.com/y3owk1n/neru/internal/adapter/platform/kwin] installs — the same
// bridge the AT-SPI window-origin path reads, because it is the same fact.
// A compositor with none of those refuses rather than reporting no window, so a
// caller can tell "nobody to ask" from "nothing is focused". Every query is
// best-effort and bounded by a short timeout so a wedged compositor cannot
// stall hint activation.
//
// focusedWindowQueryTimeout bounds each compositor IPC call.
const focusedWindowQueryTimeout = 500 * time.Millisecond

// compositorCLIPipeGuard bounds how long Output may keep waiting on the CLI's
// stdout pipe after the context kills the process — a CLI that leaked a child
// inheriting stdout would otherwise hold the pipe open indefinitely.
const compositorCLIPipeGuard = time.Second

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
// callers must not read as an unfocused desktop. found=false with a nil error
// still covers two cases on the wlroots arm — no window is focused, and the
// compositor's CLI was asked but did not answer usably (a wedged query, or
// niri's tiled windows, which report no on-screen position at all). Separating
// those is work on the CLI sources rather than on this routing.
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
// with the user's first keystroke.
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
// handler's lock, where blocking on D-Bus would stop the keyboard.
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

// compositorJSON runs a compositor CLI and decodes its JSON stdout into dst.
// Returns false on spawn error, non-zero exit, timeout, or malformed JSON.
func compositorJSON(dst any, name string, args ...string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), focusedWindowQueryTimeout)
	defer cancel()

	return compositorJSONContext(ctx, dst, name, args...)
}

// compositorJSONContext is compositorJSON bounded by the caller's context, for
// call sites that already carry a deadline (the pre-activation cursor sync).
// The deadline is real: CommandContext kills the CLI when it expires, and the
// pipe guard keeps Output from waiting past the kill — this can run under the
// mode handler's lock, where an unbounded wait stops the keyboard.
func compositorJSONContext(ctx context.Context, dst any, name string, args ...string) bool {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = compositorCLIPipeGuard

	out, err := cmd.Output()
	if err != nil {
		return false
	}

	return json.Unmarshal(out, dst) == nil
}

// hyprlandFocusedWindowBounds reads `hyprctl -j activewindow`, whose "at" [x,y]
// and "size" [w,h] are already absolute screen pixels.
func hyprlandFocusedWindowBounds() (image.Rectangle, bool, error) {
	var win struct {
		At   []int `json:"at"`
		Size []int `json:"size"`
	}
	if !compositorJSON(&win, "hyprctl", "-j", "activewindow") {
		return image.Rectangle{}, false, nil
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
	if !compositorJSON(&tree, "swaymsg", "-t", "get_tree") {
		return image.Rectangle{}, false, nil
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
// (the output's logical origin). tile_pos_in_workspace_view is only populated
// for floating windows (niri#2381); tiled windows report not-found.
func niriFocusedWindowBounds() (image.Rectangle, bool, error) {
	var win struct {
		Layout struct {
			WindowSize             []int     `json:"window_size"`                //nolint:tagliatelle // niri wire format is snake_case.
			TilePosInWorkspaceView []float64 `json:"tile_pos_in_workspace_view"` //nolint:tagliatelle // niri wire format is snake_case.
		} `json:"layout"`
	}
	if !compositorJSON(&win, "niri", "msg", "-j", "focused-window") {
		return image.Rectangle{}, false, nil
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
	if !compositorJSON(&out, "niri", "msg", "-j", "focused-output") {
		return image.Rectangle{}, false, nil
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
