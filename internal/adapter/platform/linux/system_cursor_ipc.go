//go:build linux

package linux

import (
	"context"
	"image"
	"os"

	"github.com/y3owk1n/neru/internal/adapter/platform/compositorcli"
)

// Wayland compositor-IPC cursor position. The wlroots client's cursor cache
// goes stale on any user mouse move, and the layer-shell discovery refresh
// needs a timely wl_pointer.enter, which Hyprland does not reliably send
// (#1279). Hyprland's CLI is authoritative, so it is asked first, with
// discovery as the fallback for Sway and niri.

// waylandCompositorCursorPosition returns the physical cursor position from
// the compositor's IPC, when it has one. ok=false sends the caller to
// layer-shell discovery, so a stale HYPRLAND_INSTANCE_SIGNATURE degrades
// cleanly. It is reached only behind waylandUsesWlrClientStack. The socket
// variable picks the CLI, never the backend (internal/adapter/platform/AGENTS.md).
func waylandCompositorCursorPosition(ctx context.Context) (image.Point, bool) {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return image.Point{}, false
	}

	return hyprlandCursorPosition(ctx)
}

// hyprlandCursorPosition reads `hyprctl -j cursorpos`, whose x/y are logical
// layout pixels, the same space the wlroots client fills screens in from
// zxdg_output_v1 logical position and size, so scaled monitors stay consistent.
//
// It deliberately treats a failed query like "no such query", unlike the
// bounds path (#1493), because its fallback is discovery of the same position,
// not a guess.
func hyprlandCursorPosition(ctx context.Context) (image.Point, bool) {
	var pos struct {
		X int `json:"x"`
		Y int `json:"y"`
	}

	err := compositorcli.QueryContext(ctx, &pos, "hyprctl", "-j", "cursorpos")
	if err != nil {
		return image.Point{}, false
	}

	return image.Point{X: pos.X, Y: pos.Y}, true
}
