//go:build linux

package linux

import (
	"context"
	"image"
	"os"

	"github.com/y3owk1n/neru/internal/adapter/platform/compositorcli"
)

// Wayland compositor-IPC cursor position. A Wayland client cannot query the
// global pointer position, so the wlroots client keeps a cache that a plain
// user-driven mouse move invalidates (wlroots_client.c). The layer-shell
// discovery refresh corrects it, but depends on the compositor delivering a
// wl_pointer.enter to a freshly mapped surface within its budget — which
// Hyprland does not reliably do (#1279). Hyprland exposes the authoritative
// position over its CLI instead, so the sync path asks it first and keeps
// discovery as the fallback for the compositors that expose no such query
// (Sway and niri today).

// waylandCompositorCursorPosition returns the physical cursor position from
// the running compositor's IPC, when it exposes one. ok=false means "no such
// query here" — the caller falls back to layer-shell discovery, so a stale
// HYPRLAND_INSTANCE_SIGNATURE in a non-Hyprland session degrades to the
// pre-IPC behavior (hyprctl fails to connect and reports nothing).
//
// Reached only behind waylandUsesWlrClientStack, which tests the backend the
// adapter was built with; the socket variable picks the CLI, never the backend
// (internal/adapter/platform/AGENTS.md).
func waylandCompositorCursorPosition(ctx context.Context) (image.Point, bool) {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return image.Point{}, false
	}

	return hyprlandCursorPosition(ctx)
}

// hyprlandCursorPosition reads `hyprctl -j cursorpos`, whose x/y are global
// layout (logical) pixels. That is the space the screen list already lives in:
// the wlroots client fills each screen's origin and size from
// zxdg_output_v1.logical_position/logical_size (neru_xdg_output_listener,
// wlroots_client.c), so on a scaled monitor both sides shrink together and
// containment checks against screen bounds stay consistent — the same
// agreement hyprlandFocusedWindowBounds already relies on for `activewindow`.
//
// This is the one caller that keeps a failed query and "no such query here" as
// the same answer, and it is not the conflation #1493 removed from the bounds
// path. There the fallback is a guess — the whole active screen, reported as
// though it were the window — so a caller has to know it is guessing. Here the
// fallback is layer-shell discovery, another way of learning the same position,
// and both answers lead to it: a reason would change nothing the caller does.
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
