//go:build linux

package linux

import (
	"context"
	"encoding/json"
	"image"
	"os"
	"os/exec"
	"time"
)

// Wayland focused-window bounds. A Wayland client cannot query another client's
// on-screen geometry, so the focused window's global bounds come from the
// running compositor's IPC: Hyprland (`hyprctl activewindow`), Sway
// (`swaymsg -t get_tree`), and niri (`niri msg focused-window/-output`). KWin and
// GNOME expose no simple CLI for this here, so they report not-found and callers
// fall back to the active screen. Every query is best-effort and bounded by a
// short timeout so a wedged compositor cannot stall hint activation.
//
// focusedWindowQueryTimeout bounds each compositor IPC call.
const focusedWindowQueryTimeout = 500 * time.Millisecond

// compositorCLIPipeGuard bounds how long Output may keep waiting on the CLI's
// stdout pipe after the context kills the process — a CLI that leaked a child
// inheriting stdout would otherwise hold the pipe open indefinitely.
const compositorCLIPipeGuard = time.Second

// coordPair is the length of an [x,y] / [w,h] JSON array.
const coordPair = 2

// waylandFocusedWindowBounds returns the focused window's global bounds by
// querying the running wlroots-family compositor. found=false (nil error) when
// the compositor is unknown/unsupported or the query fails.
func waylandFocusedWindowBounds() (image.Rectangle, bool, error) {
	switch {
	case os.Getenv("NIRI_SOCKET") != "":
		return niriFocusedWindowBounds()
	case os.Getenv("SWAYSOCK") != "":
		return swayFocusedWindowBounds()
	case os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "":
		return hyprlandFocusedWindowBounds()
	default:
		return image.Rectangle{}, false, nil
	}
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
