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
	"strings"
	"sync"
	"unsafe"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	// Blank-import to link the wayland-scanner generated protocol objects.
	_ "github.com/y3owk1n/neru/internal/core/infra/platform/linux/wlr_protocol"
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

	if C.neru_wlr_has_virtual_pointer(client) == 0 { //nolint:nlreturn
		C.neru_wlr_disconnect(client)

		return derrors.New(
			derrors.CodeActionFailed,
			"Wayland compositor does not support zwlr_virtual_pointer_v1 protocol; "+
				"this protocol is required and is provided by wlroots-based compositors (Sway, Hyprland, niri, River)",
		)
	}

	// Initialize cursor position to screen center. Wayland has no
	// protocol to query global pointer position, so we track it
	// client-side via move_absolute only (matching warpd's pattern).
	C.neru_wlr_init_cursor(client)

	// Populate screen list from the client.
	count := int(C.neru_wlr_screen_count(client)) //nolint:nlreturn
	screens := make([]wlrootsScreen, 0, count)

	for index := range count {
		var posX, posY, width, height C.int

		nameBuf := make([]C.char, wlrootsScreenNameBufferSize)
		if C.neru_wlr_screen_info( //nolint:nlreturn
			client,
			C.int(index),
			&posX,
			&posY,
			&width,
			&height,
			&nameBuf[0],
			wlrootsScreenNameBufferSize, //nolint:nlreturn
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

	// Fallback: if no screens were discovered via xdg_output, use a single
	// default screen so the rest of the system has something to work with.
	if len(screens) == 0 {
		screens = append(screens, wlrootsScreen{
			Name:   "wayland-0",
			Bounds: image.Rect(0, 0, wlrootsDefaultWidth, wlrootsDefaultHeight),
		})
	}

	globalWlrootsState.client = client
	globalWlrootsState.screens = screens
	globalWlrootsState.ready = true

	return nil
}

func wlrootsScreenBounds() (image.Rectangle, error) {
	err := ensureWlrootsState()
	if err != nil {
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

	err := ensureWlrootsState()
	if err != nil {
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
	err := ensureWlrootsState()
	if err != nil {
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

	// Cursor position is tracked purely client-side via move_absolute.
	// No need to poll Wayland events — doing so previously triggered
	// the pointer motion handler which corrupted the position cache.
	var posX, posY C.int
	initialized := C.neru_wlr_get_cursor(client, &posX, &posY) //nolint:nlreturn

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
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	if C.neru_wlr_move_absolute(client, C.int(point.X), C.int(point.Y)) == 0 { //nolint:nlreturn
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
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	// Move to target.
	if C.neru_wlr_move_absolute(client, C.int(point.X), C.int(point.Y)) == 0 { //nolint:nlreturn
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to move wlroots virtual pointer to (%d, %d)",
			point.X,
			point.Y,
		)
	}

	if C.neru_wlr_click(client, C.int(button)) == 0 { //nolint:nlreturn
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
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	// Move to target.
	if C.neru_wlr_move_absolute(client, C.int(point.X), C.int(point.Y)) == 0 { //nolint:nlreturn
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

	if C.neru_wlr_button(client, C.int(button), C.int(pressedInt)) == 0 { //nolint:nlreturn
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
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	if C.neru_wlr_button(client, C.int(button), 0) == 0 { //nolint:nlreturn
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to release wlroots button",
		)
	}

	return nil
}

// wlrootsScroll sends a scroll event on the virtual pointer.
// axis: 0 = vertical, 1 = horizontal.
// delta: pixel delta for the axis event.
// discrete: discrete step count (e.g., +/-1 per logical scroll click).
func wlrootsScroll(axis, delta, discrete int) error {
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	client := globalWlrootsState.client
	defer globalWlrootsState.mu.Unlock()

	res := C.neru_wlr_scroll(client, C.int(axis), C.int(delta), C.int(discrete)) //nolint:nlreturn
	if res == 0 {
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to perform wlroots scroll event",
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

	if C.neru_wlr_has_virtual_keyboard(client) == 0 { //nolint:nlreturn
		return derrors.New(
			derrors.CodeActionFailed,
			"Wayland compositor does not support zwp_virtual_keyboard_manager_v1 protocol; "+
				"this protocol is required for sticky modifier key injection on Wayland",
		)
	}

	cModifier := C.CString(modifier)
	defer C.free(unsafe.Pointer(cModifier)) //nolint:nlreturn

	cDown := C.int(0)
	if isDown {
		cDown = C.int(1)
	}

	if C.neru_wlr_modifier_event(client, cModifier, cDown) == 0 { //nolint:nlreturn
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to post wlroots modifier event %q",
			modifier,
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
