//go:build linux

package linux

import (
	"context"
	"image"
	"os"

	"github.com/y3owk1n/neru/internal/adapter/platform/compositorcli"
	"github.com/y3owk1n/neru/internal/ports"
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

// hyprlandActiveScreenBounds picks the active screen from Hyprland's own state
// instead of the wlroots cursor cache, which only learns neru's own moves and
// is stale (or uninitialized, defaulting to the first screen) once the user has
// moved the real pointer. The monitor under the real cursor wins; when the
// cursor query fails the focused monitor stands in. ok=false means neither
// answered, and the caller keeps the cache-based selection.
//
// A successful cursor read is mirrored into the wlroots cache so the
// cache-based consumers (CursorPosition, MoveCursorInstantly deltas) agree with
// the screen just chosen.
func hyprlandActiveScreenBounds(ctx context.Context) (image.Rectangle, bool) {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return image.Rectangle{}, false
	}

	screens, err := wlrootsScreens()
	if err != nil || len(screens) == 0 {
		return image.Rectangle{}, false
	}

	cursor, ok := hyprlandCursorPosition(ctx)
	if ok {
		_ = wlrootsSetCursor(cursor)

		bounds, found := screenBoundsContaining(screens, cursor)
		if found {
			return bounds, true
		}
	}

	var monitors []hyprlandMonitor

	err = compositorcli.QueryContext(ctx, &monitors, "hyprctl", "-j", "monitors")
	if err != nil {
		return image.Rectangle{}, false
	}

	return focusedMonitorBounds(screens, monitors)
}

// hyprlandMonitor is the subset of `hyprctl -j monitors` needed to find the
// focused output.
type hyprlandMonitor struct {
	Name    string `json:"name"`
	Focused bool   `json:"focused"`
}

// screenBoundsContaining returns the bounds of the screen holding point.
func screenBoundsContaining(screens []ports.Screen, point image.Point) (image.Rectangle, bool) {
	for _, screen := range screens {
		if point.In(screen.Bounds) {
			return screen.Bounds, true
		}
	}

	return image.Rectangle{}, false
}

// focusedMonitorBounds returns the bounds of the screen named like the focused
// Hyprland monitor.
func focusedMonitorBounds(
	screens []ports.Screen,
	monitors []hyprlandMonitor,
) (image.Rectangle, bool) {
	for _, monitor := range monitors {
		if !monitor.Focused {
			continue
		}

		for _, screen := range screens {
			if screen.Name == monitor.Name {
				return screen.Bounds, true
			}
		}
	}

	return image.Rectangle{}, false
}
