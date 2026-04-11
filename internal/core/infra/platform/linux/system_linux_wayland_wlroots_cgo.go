//go:build linux && cgo

package linux

/*
#cgo linux LDFLAGS: -lwayland-client
#include <wayland-client.h>
#include <stdlib.h>
#include <string.h>

// Include wlroots protocol headers relative to this package.
#include "wlr_protocol/virtual-pointer.h"
#include "wlr_protocol/virtual-pointer.c"
#include "wlr_protocol/xdg-output.h"
#include "wlr_protocol/xdg-output.c"

// ---------- Forward declarations ----------

#define NERU_MAX_OUTPUTS 16

typedef struct {
	int x;
	int y;
	int w;
	int h;
	int state; // bitmask: 1=position, 2=size, 4=name
	char name[128];
	struct wl_output *wl_output;
	struct zxdg_output_v1 *xdg_output;
} NeruWaylandScreen;

typedef struct {
	struct wl_display *display;
	struct wl_registry *registry;
	struct wl_compositor *compositor;
	struct wl_shm *shm;
	struct wl_seat *seat;
	struct wl_pointer *pointer;

	struct zwlr_virtual_pointer_manager_v1 *vptr_mgr;
	struct zwlr_virtual_pointer_v1 *vptr;
	struct zxdg_output_manager_v1 *xdg_output_mgr;

	NeruWaylandScreen screens[NERU_MAX_OUTPUTS];
	int nr_screens;

	// Cursor position cache (updated by pointer enter/motion events).
	int cursor_x;
	int cursor_y;
	int cursor_initialized;

	int connected;
} NeruWlrootsClient;

// Pointer listener callbacks — update cursor cache.
static void neru_wlr_pointer_enter(void *data,
	struct wl_pointer *pointer,
	uint32_t serial,
	struct wl_surface *surface,
	wl_fixed_t sx, wl_fixed_t sy)
{
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	// pointer enter gives surface-local coords, but for our overlay surfaces
	// we track the logical position via the screen geometry.
	// For cursor initialization we use the enter event.
	if (!c->cursor_initialized) {
		// Will be refined by discover_pointer_location pattern.
		c->cursor_initialized = 1;
	}
}

static void neru_wlr_pointer_leave(void *data,
	struct wl_pointer *pointer,
	uint32_t serial,
	struct wl_surface *surface)
{
	// No-op.
}

static void neru_wlr_pointer_motion(void *data,
	struct wl_pointer *pointer,
	uint32_t time,
	wl_fixed_t sx, wl_fixed_t sy)
{
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;
	// Convert surface-local coords to absolute by adding screen offset
	// For now, just use the values as-is - proper mapping requires screen tracking
	c->cursor_x = wl_fixed_to_int(sx);
	c->cursor_y = wl_fixed_to_int(sy);
	c->cursor_initialized = 1;
}

static void neru_wlr_pointer_button(void *data,
	struct wl_pointer *pointer,
	uint32_t serial,
	uint32_t time,
	uint32_t button,
	uint32_t state)
{
	// No-op.
}

static void neru_wlr_pointer_axis(void *data,
	struct wl_pointer *pointer,
	uint32_t time,
	uint32_t axis,
	wl_fixed_t value)
{
	// No-op.
}

static void neru_wlr_pointer_frame(void *data,
	struct wl_pointer *pointer)
{
	// No-op.
}

static void neru_wlr_pointer_axis_source(void *data,
	struct wl_pointer *pointer,
	uint32_t axis_source)
{
	// No-op.
}

static void neru_wlr_pointer_axis_stop(void *data,
	struct wl_pointer *pointer,
	uint32_t time,
	uint32_t axis)
{
	// No-op.
}

static void neru_wlr_pointer_axis_discrete(void *data,
	struct wl_pointer *pointer,
	uint32_t axis,
	int32_t discrete)
{
	// No-op.
}

static const struct wl_pointer_listener neru_wlr_pointer_listener = {
	.enter = neru_wlr_pointer_enter,
	.leave = neru_wlr_pointer_leave,
	.motion = neru_wlr_pointer_motion,
	.button = neru_wlr_pointer_button,
	.axis = neru_wlr_pointer_axis,
	.frame = neru_wlr_pointer_frame,
	.axis_source = neru_wlr_pointer_axis_source,
	.axis_stop = neru_wlr_pointer_axis_stop,
	.axis_discrete = neru_wlr_pointer_axis_discrete,
};

// ---------- xdg_output listener ----------

static void neru_xdg_output_logical_position(void *data,
	struct zxdg_output_v1 *xdg_output,
	int32_t x, int32_t y)
{
	NeruWaylandScreen *scr = (NeruWaylandScreen *)data;
	scr->x = x;
	scr->y = y;
	scr->state |= 1;
}

static void neru_xdg_output_logical_size(void *data,
	struct zxdg_output_v1 *xdg_output,
	int32_t w, int32_t h)
{
	NeruWaylandScreen *scr = (NeruWaylandScreen *)data;
	scr->w = w;
	scr->h = h;
	scr->state |= 2;
}

static void neru_xdg_output_done(void *data,
	struct zxdg_output_v1 *xdg_output)
{
	// No-op for v3+.
}

static void neru_xdg_output_name(void *data,
	struct zxdg_output_v1 *xdg_output,
	const char *name)
{
	NeruWaylandScreen *scr = (NeruWaylandScreen *)data;
	if (name) {
		strncpy(scr->name, name, sizeof(scr->name) - 1);
		scr->name[sizeof(scr->name) - 1] = '\0';
	}
	scr->state |= 4;
}

static void neru_xdg_output_description(void *data,
	struct zxdg_output_v1 *xdg_output,
	const char *description)
{
	// No-op.
}

static const struct zxdg_output_v1_listener neru_xdg_output_listener = {
	.logical_position = neru_xdg_output_logical_position,
	.logical_size = neru_xdg_output_logical_size,
	.done = neru_xdg_output_done,
	.name = neru_xdg_output_name,
	.description = neru_xdg_output_description,
};

// ---------- Registry listener ----------

static void neru_wlr_registry_global(void *data,
	struct wl_registry *registry,
	uint32_t name, const char *interface,
	uint32_t version)
{
	NeruWlrootsClient *c = (NeruWlrootsClient *)data;

	if (strcmp(interface, "zwlr_virtual_pointer_manager_v1") == 0) {
		c->vptr_mgr = wl_registry_bind(registry, name,
			&zwlr_virtual_pointer_manager_v1_interface, 1);
	} else if (strcmp(interface, "wl_compositor") == 0) {
		c->compositor = wl_registry_bind(registry, name,
			&wl_compositor_interface, 4);
	} else if (strcmp(interface, "wl_shm") == 0) {
		c->shm = wl_registry_bind(registry, name,
			&wl_shm_interface, 1);
	} else if (strcmp(interface, "wl_seat") == 0) {
		c->seat = wl_registry_bind(registry, name,
			&wl_seat_interface, 7 < version ? 7 : version);
		c->pointer = wl_seat_get_pointer(c->seat);
		wl_pointer_add_listener(c->pointer,
			&neru_wlr_pointer_listener, c);
	} else if (strcmp(interface, "wl_output") == 0) {
		if (c->nr_screens < NERU_MAX_OUTPUTS) {
			NeruWaylandScreen *scr = &c->screens[c->nr_screens];
			memset(scr, 0, sizeof(*scr));
			scr->wl_output = wl_registry_bind(registry, name,
				&wl_output_interface, 3 < version ? 3 : version);
			c->nr_screens++;
		}
	} else if (strcmp(interface, "zxdg_output_manager_v1") == 0) {
		c->xdg_output_mgr = wl_registry_bind(registry, name,
			&zxdg_output_manager_v1_interface, 3 < version ? 3 : version);
	}
}

static void neru_wlr_registry_global_remove(void *data,
	struct wl_registry *registry,
	uint32_t name)
{
	// TODO: handle hotplug.
}

static const struct wl_registry_listener neru_wlr_registry_listener = {
	.global = neru_wlr_registry_global,
	.global_remove = neru_wlr_registry_global_remove,
};

// ---------- Connect & initialize ----------

static NeruWlrootsClient* neru_wlr_connect(void) {
	NeruWlrootsClient *c = calloc(1, sizeof(NeruWlrootsClient));
	if (!c) return NULL;

	c->display = wl_display_connect(NULL);
	if (!c->display) {
		free(c);
		return NULL;
	}

	c->registry = wl_display_get_registry(c->display);
	wl_registry_add_listener(c->registry, &neru_wlr_registry_listener, c);

	// First roundtrip: discover globals.
	wl_display_roundtrip(c->display);

	// Create virtual pointer if manager was found.
	if (c->vptr_mgr) {
		c->vptr = zwlr_virtual_pointer_manager_v1_create_virtual_pointer(
			c->vptr_mgr, c->seat);
	}

	// Initialize xdg_output for each screen.
	if (c->xdg_output_mgr) {
		for (int i = 0; i < c->nr_screens; i++) {
			NeruWaylandScreen *scr = &c->screens[i];
			scr->xdg_output = zxdg_output_manager_v1_get_xdg_output(
				c->xdg_output_mgr, scr->wl_output);
			zxdg_output_v1_add_listener(scr->xdg_output,
				&neru_xdg_output_listener, scr);
		}
		// Second roundtrip: receive xdg_output events.
		wl_display_roundtrip(c->display);
	}

	c->connected = 1;
	return c;
}

static void neru_wlr_disconnect(NeruWlrootsClient *c) {
	if (!c) return;

	if (c->vptr) {
		zwlr_virtual_pointer_v1_destroy(c->vptr);
	}
	for (int i = 0; i < c->nr_screens; i++) {
		if (c->screens[i].xdg_output) {
			zxdg_output_v1_destroy(c->screens[i].xdg_output);
		}
	}
	if (c->display) {
		wl_display_disconnect(c->display);
	}
	free(c);
}

// ---------- Input injection ----------

static int neru_wlr_move_absolute(NeruWlrootsClient *c, int x, int y) {
	if (!c || !c->vptr) return 0;

	// Compute the bounding box of all screens to get the virtual pointer extent.
	int minx = 0, miny = 0, maxx = 0, maxy = 0;
	for (int i = 0; i < c->nr_screens; i++) {
		NeruWaylandScreen *scr = &c->screens[i];
		if (i == 0 || scr->x < minx) minx = scr->x;
		if (i == 0 || scr->y < miny) miny = scr->y;
		int right = scr->x + scr->w;
		int bottom = scr->y + scr->h;
		if (i == 0 || right > maxx) maxx = right;
		if (i == 0 || bottom > maxy) maxy = bottom;
	}

	// Virtual pointer space starts at 0,0 even if compositor space has
	// negative origins (same workaround as warpd).
	zwlr_virtual_pointer_v1_motion_absolute(c->vptr, 0,
		wl_fixed_from_int(x - minx),
		wl_fixed_from_int(y - miny),
		wl_fixed_from_int(maxx - minx),
		wl_fixed_from_int(maxy - miny));
	zwlr_virtual_pointer_v1_frame(c->vptr);
	wl_display_flush(c->display);

	c->cursor_x = x;
	c->cursor_y = y;
	c->cursor_initialized = 1;

	return 1;
}

// Button codes for linux/input-event-codes.h
#define NERU_BTN_LEFT   0x110
#define NERU_BTN_RIGHT  0x111
#define NERU_BTN_MIDDLE 0x112

static int neru_wlr_button(NeruWlrootsClient *c, int button, int pressed) {
	if (!c || !c->vptr) return 0;

	zwlr_virtual_pointer_v1_button(c->vptr, 0, (uint32_t)button,
		pressed ? 1 : 0);
	zwlr_virtual_pointer_v1_frame(c->vptr);
	wl_display_flush(c->display);
	return 1;
}

static int neru_wlr_click(NeruWlrootsClient *c, int button) {
	if (!c || !c->vptr) return 0;

	zwlr_virtual_pointer_v1_button(c->vptr, 0, (uint32_t)button, 1);
	zwlr_virtual_pointer_v1_button(c->vptr, 0, (uint32_t)button, 0);
	zwlr_virtual_pointer_v1_frame(c->vptr);
	wl_display_flush(c->display);
	return 1;
}

static int neru_wlr_scroll(NeruWlrootsClient *c, int axis, int direction) {
	if (!c || !c->vptr) return 0;

	// axis: 0 = vertical, 1 = horizontal
	// direction: +1 = down/right, -1 = up/left
	zwlr_virtual_pointer_v1_axis_discrete(c->vptr, 0,
		(uint32_t)axis,
		wl_fixed_from_int(15 * direction),
		direction);
	zwlr_virtual_pointer_v1_frame(c->vptr);
	wl_display_flush(c->display);
	return 1;
}

static int neru_wlr_get_cursor(NeruWlrootsClient *c, int *x, int *y) {
	if (!c) return 0;
	*x = c->cursor_x;
	*y = c->cursor_y;
	return c->cursor_initialized;
}

static int neru_wlr_screen_count(NeruWlrootsClient *c) {
	if (!c) return 0;
	return c->nr_screens;
}

static int neru_wlr_screen_info(NeruWlrootsClient *c, int idx,
	int *x, int *y, int *w, int *h, char *name_out, int name_len)
{
	if (!c || idx < 0 || idx >= c->nr_screens) return 0;
	NeruWaylandScreen *scr = &c->screens[idx];
	*x = scr->x;
	*y = scr->y;
	*w = scr->w;
	*h = scr->h;
	strncpy(name_out, scr->name, (size_t)(name_len - 1));
	name_out[name_len - 1] = '\0';
	return 1;
}

static int neru_wlr_has_virtual_pointer(NeruWlrootsClient *c) {
	return c && c->vptr != NULL;
}
*/
import "C"

import (
	"fmt"
	"image"
	"os"
	"strings"
	"sync"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

type wlrootsScreen struct {
	Name   string
	Bounds image.Rectangle
}

type wlrootsState struct {
	mu sync.RWMutex

	client  *C.NeruWlrootsClient
	screens []wlrootsScreen
	ready   bool
}

var globalWlrootsState = &wlrootsState{}

func ensureWlrootsState() error {
	globalWlrootsState.mu.Lock()
	defer globalWlrootsState.mu.Unlock()

	if globalWlrootsState.ready {
		return nil
	}

	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return derrors.New(
			derrors.CodeNotSupported,
			"WAYLAND_DISPLAY is not set; wlroots backend is unavailable",
		)
	}

	client := C.neru_wlr_connect()
	if client == nil {
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to connect to Wayland compositor",
		)
	}

	if C.neru_wlr_has_virtual_pointer(client) == 0 {
		C.neru_wlr_disconnect(client)

		return derrors.New(
			derrors.CodeActionFailed,
			"Wayland compositor does not support zwlr_virtual_pointer_v1 protocol; "+
				"this protocol is required and is provided by wlroots-based compositors (Sway, Hyprland, niri, River)",
		)
	}

	// Populate screen list from the client.
	count := int(C.neru_wlr_screen_count(client))
	screens := make([]wlrootsScreen, 0, count)

	for i := range count {
		var x, y, w, h C.int

		nameBuf := make([]C.char, 128)
		if C.neru_wlr_screen_info(client, C.int(i), &x, &y, &w, &h, &nameBuf[0], 128) != 0 {
			name := C.GoString(&nameBuf[0])
			if name == "" {
				name = fmt.Sprintf("output-%d", i)
			}

			screens = append(screens, wlrootsScreen{
				Name: name,
				Bounds: image.Rect(
					int(x),
					int(y),
					int(x+w),
					int(y+h),
				),
			})
		}
	}

	// Fallback: if no screens were discovered via xdg_output, use a single
	// default screen so the rest of the system has something to work with.
	if len(screens) == 0 {
		screens = append(screens, wlrootsScreen{
			Name:   "wayland-0",
			Bounds: image.Rect(0, 0, 1920, 1080),
		})
	}

	globalWlrootsState.client = client
	globalWlrootsState.screens = screens
	globalWlrootsState.ready = true

	return nil
}

func wlrootsScreenBounds() (image.Rectangle, error) {
	if err := ensureWlrootsState(); err != nil {
		return image.Rectangle{}, err
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	// Return bounds of the screen containing the cursor.
	cursor, _ := wlrootsCursorPositionLocked()
	for _, screen := range globalWlrootsState.screens {
		if cursor.In(screen.Bounds) {
			return screen.Bounds, nil
		}
	}

	// Fallback to first screen.
	return globalWlrootsState.screens[0].Bounds, nil
}

func wlrootsScreenBoundsByName(name string) (image.Rectangle, bool, error) {
	if name == "" {
		return image.Rectangle{}, false, nil
	}

	if err := ensureWlrootsState(); err != nil {
		return image.Rectangle{}, false, err
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	for _, screen := range globalWlrootsState.screens {
		if strings.EqualFold(screen.Name, name) {
			return screen.Bounds, true, nil
		}
	}

	return image.Rectangle{}, false, nil
}

func wlrootsScreenNames() ([]string, error) {
	if err := ensureWlrootsState(); err != nil {
		return nil, err
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	names := make([]string, 0, len(globalWlrootsState.screens))
	for _, screen := range globalWlrootsState.screens {
		names = append(names, screen.Name)
	}

	return names, nil
}

func wlrootsCursorPosition() (image.Point, error) {
	if err := ensureWlrootsState(); err != nil {
		return image.Point{}, err
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	return wlrootsCursorPositionLocked()
}

// wlrootsCursorPositionLocked returns cursor position while holding at least RLock.
func wlrootsCursorPositionLocked() (image.Point, error) {
	client := globalWlrootsState.client
	if client == nil {
		return image.Point{}, nil
	}

	var x, y C.int
	initialized := C.neru_wlr_get_cursor(client, &x, &y)

	// If cursor was never initialized via motion events, log and return (0,0)
	if initialized == 0 {
		fmt.Fprintf(os.Stderr, "neru: cursor not initialized, screens=%d\n", len(globalWlrootsState.screens))
		if len(globalWlrootsState.screens) > 0 {
			scr := globalWlrootsState.screens[0]
			fmt.Fprintf(os.Stderr, "neru: screen[0] bounds=%v\n", scr.Bounds)
			fmt.Fprintf(os.Stderr, "neru: would use center: (%d, %d)\n",
				scr.Bounds.Min.X+scr.Bounds.Dx()/2,
				scr.Bounds.Min.Y+scr.Bounds.Dy()/2)
		}
		// Return (0,0) instead of center so it's obvious something is wrong
		return image.Point{X: 0, Y: 0}, nil
	}

	fmt.Fprintf(os.Stderr, "neru: cursor position: (%d, %d), initialized=%d\n", int(x), int(y), initialized)
	return image.Point{X: int(x), Y: int(y)}, nil
}

func wlrootsMoveCursorToPoint(point image.Point) error {
	if err := ensureWlrootsState(); err != nil {
		return err
	}

	globalWlrootsState.mu.RLock()
	client := globalWlrootsState.client
	globalWlrootsState.mu.RUnlock()

	if C.neru_wlr_move_absolute(client, C.int(point.X), C.int(point.Y)) == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to move wlroots virtual pointer to (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

// wlrootsClick performs a mouse click at the given position using the virtual pointer.
func wlrootsClick(point image.Point, button int) error {
	if err := ensureWlrootsState(); err != nil {
		return err
	}

	globalWlrootsState.mu.RLock()
	client := globalWlrootsState.client
	globalWlrootsState.mu.RUnlock()

	// Move to target.
	if C.neru_wlr_move_absolute(client, C.int(point.X), C.int(point.Y)) == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to move wlroots virtual pointer to (%d, %d)",
			point.X,
			point.Y,
		)
	}

	if C.neru_wlr_click(client, C.int(button)) == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform wlroots click (button %d) at (%d, %d)",
			button,
			point.X,
			point.Y,
		)
	}

	return nil
}

// wlrootsButtonEvent presses or releases a button at the given position.
func wlrootsButtonEvent(point image.Point, button int, pressed bool) error {
	if err := ensureWlrootsState(); err != nil {
		return err
	}

	globalWlrootsState.mu.RLock()
	client := globalWlrootsState.client
	globalWlrootsState.mu.RUnlock()

	// Move to target.
	if C.neru_wlr_move_absolute(client, C.int(point.X), C.int(point.Y)) == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to move wlroots virtual pointer to (%d, %d)",
			point.X,
			point.Y,
		)
	}

	pressedInt := 0
	if pressed {
		pressedInt = 1
	}

	if C.neru_wlr_button(client, C.int(button), C.int(pressedInt)) == 0 {
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to perform wlroots button event",
		)
	}

	return nil
}

// wlrootsButtonRelease releases a button at the current cursor position.
func wlrootsButtonRelease(button int) error {
	if err := ensureWlrootsState(); err != nil {
		return err
	}

	globalWlrootsState.mu.RLock()
	client := globalWlrootsState.client
	globalWlrootsState.mu.RUnlock()

	if C.neru_wlr_button(client, C.int(button), 0) == 0 {
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to release wlroots button",
		)
	}

	return nil
}

// wlrootsScroll sends a scroll event on the virtual pointer.
func wlrootsScroll(axis, direction int) error {
	if err := ensureWlrootsState(); err != nil {
		return err
	}

	globalWlrootsState.mu.RLock()
	client := globalWlrootsState.client
	globalWlrootsState.mu.RUnlock()

	if C.neru_wlr_scroll(client, C.int(axis), C.int(direction)) == 0 {
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to perform wlroots scroll event",
		)
	}

	return nil
}

// Exported button constants for use by the accessibility adapter.
const (
	WlrBtnLeft   = 0x110
	WlrBtnRight  = 0x111
	WlrBtnMiddle = 0x112
)
