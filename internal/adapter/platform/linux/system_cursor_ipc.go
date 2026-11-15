//go:build linux

package linux

import (
	"context"
	"image"
	"os"

	"github.com/y3owk1n/neru/internal/adapter/platform/compositorcli"
	"github.com/y3owk1n/neru/internal/ports"
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
