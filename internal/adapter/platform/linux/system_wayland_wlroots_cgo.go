//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: wayland-client xkbcommon
#cgo linux CFLAGS: -DWLR_CPLUSPLUS
#include <stdlib.h>
#include "wlroots_client.h"
*/
import "C"

import (
	"fmt"
	"image"
	"os"
	"sync"
	"time"
	"unsafe"

	// Blank-import to link the wayland-scanner generated protocol objects.
	_ "github.com/y3owk1n/neru/internal/adapter/platform/linux/wlr_protocol"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

const (
	wlrootsScreenNameBufferSize = 128
	wlrootsDefaultWidth         = 1920
	wlrootsDefaultHeight        = 1080
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

	// hasVirtualPointer reports zwlr_virtual_pointer_v1. Without it (KWin),
	// pointer input goes through libei (system_wayland_kde_cgo.go).
	hasVirtualPointer bool
}

var globalWlrootsState = &wlrootsState{}

// readWlrootsScreens reads the current output list from the C client. There is
// no C-side lock, because the fields are only mutated on the dispatch thread
// and read here best-effort. With no outputs known it returns one default
// screen.
func readWlrootsScreens(client *C.NeruWlrootsClient) []wlrootsScreen {
	count := int(C.neru_wlr_screen_count(client))
	screens := make([]wlrootsScreen, 0, count)

	for index := range count {
		var posX, posY, width, height C.int

		nameBuf := make([]C.char, wlrootsScreenNameBufferSize)
		if C.neru_wlr_screen_info(
			client,
			C.int(index),
			&posX,
			&posY,
			&width,
			&height,
			&nameBuf[0],
			wlrootsScreenNameBufferSize,
		) != 0 {
			name := C.GoString(&nameBuf[0])
			if name == "" {
				name = fmt.Sprintf("output-%d", index)
			}

			screens = append(screens, wlrootsScreen{
				Name: name,
				Bounds: image.Rect(
					int(posX),
					int(posY),
					int(posX+width),
					int(posY+height),
				),
			})
		}
	}

	if len(screens) == 0 {
		screens = append(screens, wlrootsScreen{
			Name:   "wayland-0",
			Bounds: image.Rect(0, 0, wlrootsDefaultWidth, wlrootsDefaultHeight),
		})
	}

	return screens
}

// wlrootsRefreshScreens re-reads the output list into the Go cache after a
// hotplug. It must run before the app re-queries ScreenBounds and friends on a
// screen-change event.
func wlrootsRefreshScreens() {
	err := ensureWlrootsState()
	if err != nil {
		return
	}

	globalWlrootsState.mu.Lock()
	defer globalWlrootsState.mu.Unlock()

	if globalWlrootsState.client == nil {
		return
	}

	globalWlrootsState.screens = readWlrootsScreens(globalWlrootsState.client)
}

// wlrootsScreenEventFD returns a readable fd that becomes ready when an output
// is added or removed. ok is false when the client is unavailable or exposes no
// pipe. The client owns the fd, so callers poll it read-only and never close it.
func wlrootsScreenEventFD() (int, bool) {
	err := ensureWlrootsState()
	if err != nil {
		return -1, false
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	client := globalWlrootsState.client
	if client == nil {
		return -1, false
	}

	fd := C.neru_wlr_screen_event_fd(client)
	if fd < 0 {
		return -1, false
	}

	return int(fd), true
}

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

	// A missing zwlr_virtual_pointer_v1 (KWin) is not fatal. Screens and the
	// overlay still work, and pointer input routes through libei.
	hasVirtualPointer := C.neru_wlr_has_virtual_pointer(client) != 0

	// Wayland cannot query the global pointer, so the position is tracked
	// client-side from move_absolute, starting at screen center.
	C.neru_wlr_init_cursor(client)

	// Start dispatch after init_cursor to avoid reader_count conflicts with
	// its roundtrips.
	C.neru_wlr_start_dispatch(client)

	screens := readWlrootsScreens(client)

	globalWlrootsState.client = client
	globalWlrootsState.screens = screens
	globalWlrootsState.hasVirtualPointer = hasVirtualPointer

	globalWlrootsState.ready = true

	return nil
}

// wlrootsHasVirtualPointer reports whether the connected compositor advertises
// zwlr_virtual_pointer_v1, which picks the native path over libei.
func wlrootsHasVirtualPointer() (bool, error) {
	err := ensureWlrootsState()
	if err != nil {
		return false, err
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	return globalWlrootsState.hasVirtualPointer, nil
}

// wlrootsFocusedAppIDBufferSize matches NERU_APP_ID_LEN in wlroots_client.h.
const wlrootsFocusedAppIDBufferSize = 256

// wlrootsFocusedAppID returns the app_id of the focused toplevel from
// wlr-foreign-toplevel-management. The bool is false when there is no manager
// (GNOME/Mutter) or nothing is focused yet.
func wlrootsFocusedAppID() (string, bool) {
	err := ensureWlrootsState()
	if err != nil {
		return "", false
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	client := globalWlrootsState.client
	if client == nil {
		return "", false
	}

	buf := make([]C.char, wlrootsFocusedAppIDBufferSize)
	bufLen := C.int(wlrootsFocusedAppIDBufferSize)

	if C.neru_wlr_focused_app_id(client, &buf[0], bufLen) == 0 {
		return "", false
	}

	return C.GoString(&buf[0]), true
}

// wlrootsFocusedTitleBufferSize matches NERU_TITLE_LEN in wlroots_client.h.
const wlrootsFocusedTitleBufferSize = 512

// wlrootsFocusedAppIdentity returns the app_id and title of the focused
// toplevel, read under one lock so they describe the same window. The bool is
// false when no manager is present or nothing is focused.
func wlrootsFocusedAppIdentity() (string, string, bool) {
	err := ensureWlrootsState()
	if err != nil {
		return "", "", false
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	client := globalWlrootsState.client
	if client == nil {
		return "", "", false
	}

	appBuf := make([]C.char, wlrootsFocusedAppIDBufferSize)
	titleBuf := make([]C.char, wlrootsFocusedTitleBufferSize)
	appLen := C.int(wlrootsFocusedAppIDBufferSize)
	titleLen := C.int(wlrootsFocusedTitleBufferSize)

	found := C.neru_wlr_focused_app_identity(
		client,
		&appBuf[0],
		appLen,
		&titleBuf[0],
		titleLen,
	)
	if found == 0 {
		return "", "", false
	}

	return C.GoString(&appBuf[0]), C.GoString(&titleBuf[0]), true
}

// wlrootsFocusEventFD returns a readable fd that becomes ready when the focused
// app_id changes. ok is false when the client is unavailable or exposes no
// pipe. The client owns the fd, so callers poll it read-only and never close it.
func wlrootsFocusEventFD() (int, bool) {
	err := ensureWlrootsState()
	if err != nil {
		return -1, false
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	client := globalWlrootsState.client
	if client == nil {
		return -1, false
	}

	fd := C.neru_wlr_focus_event_fd(client)
	if fd < 0 {
		return -1, false
	}

	return int(fd), true
}

// wlrootsSetCursor mirrors an externally injected pointer position (a libei
// move on KDE) into the client-side cursor cache.
func wlrootsSetCursor(point image.Point) error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	defer globalWlrootsState.mu.Unlock()

	C.neru_wlr_set_cursor(globalWlrootsState.client, C.int(point.X), C.int(point.Y))

	return nil
}

func wlrootsRefreshCursorPosition() error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	defer globalWlrootsState.mu.Unlock()

	if C.neru_wlr_refresh_cursor(globalWlrootsState.client) == 0 {
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to refresh wlroots cursor position",
		)
	}

	return nil
}

func wlrootsScreenBounds() (image.Rectangle, error) {
	err := ensureWlrootsState()
	if err != nil {
		return image.Rectangle{}, err
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	cursor, _ := wlrootsCursorPositionLocked()
	for _, screen := range globalWlrootsState.screens {
		if cursor.In(screen.Bounds) {
			return screen.Bounds, nil
		}
	}

	return globalWlrootsState.screens[0].Bounds, nil
}

// wlrootsScreenOutputs returns every connected output, named and placed in
// the shared coordinate space, in the order the compositor announced them.
func wlrootsScreenOutputs() ([]screenCastOutput, error) {
	err := ensureWlrootsState()
	if err != nil {
		return nil, err
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	outputs := make([]screenCastOutput, 0, len(globalWlrootsState.screens))
	for _, screen := range globalWlrootsState.screens {
		outputs = append(outputs, screenCastOutput{name: screen.Name, bounds: screen.Bounds})
	}

	return outputs, nil
}

func wlrootsScreens() ([]ports.Screen, error) {
	err := ensureWlrootsState()
	if err != nil {
		return nil, err
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	screens := make([]ports.Screen, 0, len(globalWlrootsState.screens))
	for _, screen := range globalWlrootsState.screens {
		screens = append(screens, ports.Screen{Name: screen.Name, Bounds: screen.Bounds})
	}

	return screens, nil
}

func wlrootsCursorPosition() (image.Point, error) {
	err := ensureWlrootsState()
	if err != nil {
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

	// Never poll Wayland events here. That triggers the pointer motion
	// handler, which corrupts the position cache.
	var posX, posY C.int
	initialized := C.neru_wlr_get_cursor(client, &posX, &posY)

	// If cursor was never initialized, fall back to first screen center
	if initialized == 0 {
		if len(globalWlrootsState.screens) > 0 {
			scr := globalWlrootsState.screens[0]

			return image.Point{
				X: scr.Bounds.Min.X + scr.Bounds.Dx()/2,
				Y: scr.Bounds.Min.Y + scr.Bounds.Dy()/2,
			}, nil
		}

		return image.Point{}, nil
	}

	return image.Point{X: int(posX), Y: int(posY)}, nil
}

func wlrootsMoveCursorToPoint(point image.Point) error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	defer globalWlrootsState.mu.Unlock()

	client := globalWlrootsState.client

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

func wlrootsMoveCursorBy(delta image.Point) error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	defer globalWlrootsState.mu.Unlock()

	client := globalWlrootsState.client

	if C.neru_wlr_move_relative(client, C.int(delta.X), C.int(delta.Y)) == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to move wlroots virtual pointer by (%d, %d)",
			delta.X,
			delta.Y,
		)
	}

	return nil
}

// wlrootsClick performs a mouse click at the given position using the virtual pointer.
func wlrootsClick(point image.Point, button int) error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	defer globalWlrootsState.mu.Unlock()

	client := globalWlrootsState.client

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
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	defer globalWlrootsState.mu.Unlock()

	client := globalWlrootsState.client

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
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	defer globalWlrootsState.mu.Unlock()

	client := globalWlrootsState.client

	if C.neru_wlr_button(client, C.int(button), 0) == 0 {
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to release wlroots button",
		)
	}

	return nil
}

// wlrootsScroll sends a scroll event on the virtual pointer. axis is 0 for
// vertical and 1 for horizontal, and discrete is the step count.
func wlrootsScroll(axis, delta, discrete int) error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	res := C.neru_wlr_scroll(client, C.int(axis), C.int(delta), C.int(discrete))
	if res == 0 {
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to perform wlroots scroll event",
		)
	}

	return nil
}

// wlrootsScrollContinuous sends one fractional scroll step with no discrete
// count, which smooth scroll is built on. A zero delta_discrete makes the
// compositor pass the fraction on as a wl_pointer.axis instead of holding it
// until a whole notch adds up. delta is in wlrootsScroll's units.
func wlrootsScrollContinuous(axis int, delta float64) error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	res := C.neru_wlr_scroll_continuous(client, C.int(axis), C.double(delta))
	if res == 0 {
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to perform wlroots continuous scroll event",
		)
	}

	return nil
}

func wlrootsScrollBatch(axis int, deltas, discretes []int) error {
	if len(deltas) == 0 {
		return nil
	}

	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	cDeltas := make([]C.int, len(deltas))
	cDiscretes := make([]C.int, len(discretes))
	for i := range deltas {
		cDeltas[i] = C.int(deltas[i])
		cDiscretes[i] = C.int(discretes[i])
	}

	res := C.neru_wlr_scroll_batch(
		client,
		C.int(axis),
		&cDeltas[0],
		&cDiscretes[0],
		C.int(len(deltas)),
	)
	if res == 0 {
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to perform wlroots batch scroll",
		)
	}

	return nil
}

func wlrootsModifierEvent(modifier string, isDown bool) error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	cModifier := C.CString(modifier)
	defer C.free(unsafe.Pointer(cModifier))

	cDown := C.int(0)
	if isDown {
		cDown = C.int(1)
	}

	if C.neru_wlr_modifier_event(client, cModifier, cDown) == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to post wlroots modifier event %q",
			modifier,
		)
	}

	return nil
}

// wlrootsSync waits until the compositor has processed every request this
// connection issued, so a caller knows an injected modifier is applied. False
// means no answer in time or no connection, never that the modifier is known
// not to have landed.
func wlrootsSync(timeout time.Duration) bool {
	err := ensureWlrootsState()
	if err != nil {
		return false
	}

	globalWlrootsState.mu.Lock()
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	milliseconds := timeout.Milliseconds()
	if milliseconds <= 0 {
		return false
	}

	return C.neru_wlr_sync(client, C.int(milliseconds)) != 0
}

// wlrootsHasVirtualKeyboard reports whether the connected compositor advertises
// a usable virtual keyboard (zwp_virtual_keyboard_v1).
func wlrootsHasVirtualKeyboard() (bool, error) {
	err := ensureWlrootsState()
	if err != nil {
		return false, err
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	return C.neru_wlr_has_virtual_keyboard(globalWlrootsState.client) != 0, nil
}

// wlrootsKey presses (pressed=true) or releases (pressed=false) a single key
// identified by its evdev keycode on the virtual keyboard.
func wlrootsKey(keycode uint32, pressed bool) error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	pressedInt := 0
	if pressed {
		pressedInt = 1
	}

	if C.neru_wlr_key(client, C.uint32_t(keycode), C.int(pressedInt)) == 0 {
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to post virtual keyboard key event",
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

// wlrootsFocusedWindowGeometry returns the activated toplevel's global bounds.
// Only cosmic-comp's zcosmic_toplevel_info_v1 carries geometry, so elsewhere
// this is CodeNotSupported. found=false with a nil error means nothing is
// activated or the window is not placed yet.
func wlrootsFocusedWindowGeometry() (image.Rectangle, bool, error) {
	err := ensureWlrootsState()
	if err != nil {
		return image.Rectangle{}, false, err
	}

	globalWlrootsState.mu.RLock()
	defer globalWlrootsState.mu.RUnlock()

	client := globalWlrootsState.client
	if client == nil || C.neru_wlr_has_toplevel_geometry(client) == 0 {
		return image.Rectangle{}, false, derrors.New(
			derrors.CodeNotSupported,
			"this compositor reports no toplevel geometry (zcosmic_toplevel_info_v1 v2 is absent)",
		)
	}

	var x, y, w, h C.int32_t
	if C.neru_wlr_focused_toplevel_geometry(client, &x, &y, &w, &h) == 0 {
		return image.Rectangle{}, false, nil
	}

	return image.Rect(int(x), int(y), int(x)+int(w), int(y)+int(h)), true, nil
}
