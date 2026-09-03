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
	"time"
	"unsafe"

	// Blank-import to link the wayland-scanner generated protocol objects.
	_ "github.com/y3owk1n/neru/internal/adapter/platform/linux/wlr_protocol"
	"github.com/y3owk1n/neru/internal/derrors"
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

	// hasVirtualPointer reports whether the compositor advertises
	// zwlr_virtual_pointer_v1. When false (notably KWin/KDE), pointer moves,
	// clicks, and scrolls are injected through libei / the RemoteDesktop portal
	// instead (see system_wayland_kde_cgo.go).
	hasVirtualPointer bool
}

var globalWlrootsState = &wlrootsState{}

// readWlrootsScreens reads the current output list from the C client into Go
// values. It mirrors the C-side neru_wlr_screen_count / neru_wlr_screen_info
// reads used elsewhere (no C-side lock; the fields are only mutated on the
// dispatch thread and read here best-effort). When no outputs are known it
// returns a single default screen so the rest of the system has something to
// work with.
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

	// Fallback: if no screens were discovered via xdg_output, use a single
	// default screen so the rest of the system has something to work with.
	if len(screens) == 0 {
		screens = append(screens, wlrootsScreen{
			Name:   "wayland-0",
			Bounds: image.Rect(0, 0, wlrootsDefaultWidth, wlrootsDefaultHeight),
		})
	}

	return screens
}

// wlrootsRefreshScreens re-reads the output list from the C client into the Go
// cache after a display-configuration change (hotplug). ScreenBounds and friends
// read globalWlrootsState.screens, so this must run before the app re-queries
// them on a screen-change event.
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

// wlrootsScreenEventFD returns a readable file descriptor that becomes ready
// whenever the display configuration changes (an output is added or removed), so
// the app watcher can wake and re-enumerate instead of polling. ok is false when
// the wlroots client is unavailable or exposes no pipe. The fd is owned by the
// client and closed on disconnect; callers must poll it read-only and must not
// close it.
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

	// zwlr_virtual_pointer_v1 is the native injection path (Sway, Hyprland,
	// niri, River). KWin/KDE intentionally does not implement it, so its
	// absence is no longer fatal: screen bounds and the overlay still come up
	// via xdg_output + zwlr_layer_shell_v1, and pointer moves/clicks are routed
	// through libei / the RemoteDesktop portal (connected lazily on first use).
	hasVirtualPointer := C.neru_wlr_has_virtual_pointer(client) != 0

	// Initialize cursor position to screen center. Wayland has no
	// protocol to query global pointer position, so we track it
	// client-side via move_absolute only (matching warpd's pattern).
	C.neru_wlr_init_cursor(client)

	// Start dispatch thread after init_cursor to avoid reader_count
	// conflicts with roundtrip calls during cursor discovery.
	C.neru_wlr_start_dispatch(client)

	// Populate screen list from the client.
	screens := readWlrootsScreens(client)

	globalWlrootsState.client = client
	globalWlrootsState.screens = screens
	globalWlrootsState.hasVirtualPointer = hasVirtualPointer

	globalWlrootsState.ready = true

	return nil
}

// wlrootsHasVirtualPointer reports whether the connected compositor advertises
// zwlr_virtual_pointer_v1. The Wayland input dispatcher uses this to choose
// between the native virtual-pointer path (wlroots) and libei (KWin/KDE).
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

// wlrootsFocusedAppID returns the app_id of the currently activated (focused)
// toplevel as reported by wlr-foreign-toplevel-management. The bool is false
// when the compositor exposes no such manager (GNOME/Mutter) or when nothing
// is focused yet. This works for both wlroots compositors and KWin/KDE, since
// both bind the same client stack via ensureWlrootsState.
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

// wlrootsFocusedAppIdentity returns the app_id and title of the currently
// activated (focused) toplevel, read together under a single lock so they
// always describe the same window. The bool is false when no manager is present
// or nothing is focused. The title disambiguates multiple windows of the
// focused application, which share an app_id.
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

// wlrootsFocusEventFD returns a readable file descriptor that becomes ready
// whenever the focused app_id changes, so the app watcher can wake on focus
// changes instead of polling. ok is false when the wlroots client is
// unavailable (GNOME/Mutter, no compositor) or exposes no pipe. The fd is owned
// by the client and closed on disconnect; callers must not close it and must
// poll it read-only.
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

// wlrootsSetCursor mirrors an externally-injected pointer position (e.g. a
// libei move on KDE) into the wlroots client's client-side cursor cache so
// CursorPosition and screen resolution stay accurate.
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
	err := ensureWlrootsState()
	if err != nil {
		return err
	}

	globalWlrootsState.mu.Lock()
	defer globalWlrootsState.mu.Unlock()

	client := globalWlrootsState.client

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

	res := C.neru_wlr_scroll(client, C.int(axis), C.int(delta), C.int(discrete))
	if res == 0 {
		return derrors.New(
			derrors.CodeActionFailed,
			"failed to perform wlroots scroll event",
		)
	}

	return nil
}

// wlrootsScrollContinuous sends one sub-notch-capable scroll step on the virtual
// pointer: an axis event carrying a fractional value and no discrete step count.
//
// This is the primitive smooth scroll is built on. wlroots leaves
// delta_discrete at zero for a plain axis request, and a zero delta_discrete is
// what makes the compositor pass the fraction on to the focused client as a
// wl_pointer.axis rather than holding it in a value120 accumulator until a whole
// notch adds up.
//
// axis: 0 = vertical, 1 = horizontal. delta is in the same units wlrootsScroll
// takes, so one wheel notch is the same number either way.
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
// connection has issued, and reports whether it answered within timeout.
//
// A caller that has just injected a virtual-keyboard modifier uses it to learn
// the modifier is applied rather than sleeping and hoping. False means the
// compositor did not answer in time, or there is no wlroots connection to ask —
// never that the modifier is known not to have landed.
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
