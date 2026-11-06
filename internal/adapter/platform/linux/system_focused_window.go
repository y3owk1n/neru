//go:build linux

package linux

import (
	"image"
	"os"

	"github.com/y3owk1n/neru/internal/adapter/platform/compositorcli"
	"github.com/y3owk1n/neru/internal/adapter/platform/gnomeshell"
	"github.com/y3owk1n/neru/internal/adapter/platform/kwin"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Wayland focused-window bounds. A Wayland client cannot query another client's
// geometry, so the compositor answers. Hyprland, Sway and niri through their
// CLIs ([github.com/y3owk1n/neru/internal/adapter/platform/compositorcli]),
// KWin through the geometry script
// [github.com/y3owk1n/neru/internal/adapter/platform/kwin] installs, the same
// bridge the AT-SPI window-origin path reads. A compositor with no source, or
// a CLI that failed, refuses rather than reporting no window, so callers can
// tell "nobody to ask" from "nothing is focused". Each query has a short
// timeout so a wedged compositor cannot stall hint activation.

// coordPair is the length of an [x,y] / [w,h] JSON array.
const coordPair = 2

// focusedWindowSource names where a session's focused-window geometry comes
// from, so routing is testable without a compositor running.
type focusedWindowSource int

const (
	// focusedWindowSourceNone is a Wayland session with nothing to ask. It is
	// an answer, not a default.
	focusedWindowSourceNone focusedWindowSource = iota
	focusedWindowSourceKWin
	focusedWindowSourceNiri
	focusedWindowSourceSway
	focusedWindowSourceHyprland
	focusedWindowSourceCosmic
	focusedWindowSourceGNOME
)

// waylandFocusedWindowSource picks the geometry source for a Wayland session.
// The backend decides first and the environment only refines it, because a
// socket such as an inherited SWAYSOCK says which CLI is reachable, not which
// compositor this session runs.
func waylandFocusedWindowSource(backend string) focusedWindowSource {
	if backend == backendWaylandKDE {
		return focusedWindowSourceKWin
	}

	// cosmic-comp answers over its own toplevel protocol.
	if backend == backendWaylandCOSMIC {
		return focusedWindowSourceCosmic
	}

	// Mutter exposes no window geometry, so the Neru GNOME Shell extension
	// answers.
	if backend == backendWaylandGNOME {
		return focusedWindowSourceGNOME
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
// the source its session exposes. CodeNotSupported means no source, which
// callers must not read as an unfocused desktop. Any other error means the
// source did not answer. found=false with a nil error means no window has an
// on-screen position right now, such as an unfocused desktop or a tiled niri
// window.
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
	case focusedWindowSourceCosmic:
		return wlrootsFocusedWindowGeometry()
	case focusedWindowSourceGNOME:
		return gnomeFocusedWindowBounds()
	case focusedWindowSourceNone:
	}

	return image.Rectangle{}, false, derrors.New(
		derrors.CodeNotSupported,
		"no focused-window geometry source on linux backend "+backend+
			": this compositor exposes neither an IPC neru can query nor a scripting bridge",
	)
}

// warmFocusedWindowSource starts a session's geometry source once, when the
// adapter is built. The KWin script must be installed before it can answer,
// and until then the cache reports no window, which looks like an unfocused
// desktop. Starting at construction overlaps the install with daemon startup
// instead of the user's first keystroke, and the installer retries on its own.
//
// Nothing is exported, owned or written on a session not running KWin. The
// install probes the bus first (#1430). It only subscribes ahead of the probe,
// so it hears a compositor that starts after the daemon.
func warmFocusedWindowSource(backend string) {
	if backend == backendWaylandGNOME {
		gnomeshell.Shared(nil).EnsureStarted()

		return
	}

	if backend != backendWaylandKDE {
		return
	}

	kwin.Shared(nil).EnsureStarted()
}

// kwinFocusedWindowBounds reads the geometry KWin last pushed to the shared
// cache, installing the script if it is not yet installed, including after a
// failed attempt.
//
// The install is asynchronous, so an early call reports no window rather than
// waiting. This runs under the mode handler's lock, where blocking on D-Bus
// would stop the keyboard. A call during an attempt gets the last completed
// answer for the same reason.
//
// A failed install is CodeNotSupported, the same answer as a compositor with
// no source, rather than a failure that would send the user to check a
// permission.
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

// hyprlandFocusedWindowBounds reads `hyprctl -j activewindow`, whose "at" and
// "size" are absolute screen pixels. The `-j` matters, because JSON reports no
// active window as `{}` while plain text says "Invalid", which would read as a
// failed query on an idle session.
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
// tile position) with `niri msg -j focused-output` (the output's logical
// origin). tile_pos_in_workspace_view is only populated for floating windows
// (niri#2381), so a tiled window reports not-found with no error, which must
// not be reported as a failure.
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

// gnomeFocusedWindowBounds reads the extension's cache, the same rectangle the
// AT-SPI origin source offsets by. EnsureStarted runs on every call because
// the shell can reload the extension.
func gnomeFocusedWindowBounds() (image.Rectangle, bool, error) {
	bridge := gnomeshell.Shared(nil)
	bridge.EnsureStarted()

	bounds, found, err := bridge.Bounds()
	if err != nil {
		return image.Rectangle{}, false, derrors.Wrap(
			err,
			derrors.CodeNotSupported,
			"no GNOME focused-window geometry source",
		)
	}

	return bounds, found, nil
}
