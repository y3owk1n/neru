//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: x11 xtst
#include <stdlib.h>
#include "../../platform/linux/x11_eventtap.h"
*/
import "C"

import (
	"math/bits"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

const (
	x11PollingInterval = 10 * time.Millisecond

	// x11GrabWarnAfter is how long a mode start waits for the keyboard before
	// warning. The hotkey that started the mode is still held, and until it
	// comes up the server answers AlreadyGrabbed, because Neru's hotkey
	// connection or the window manager holds a passive grab on it.
	x11GrabWarnAfter = time.Second
	x11KeyBufferSize = 64
	x11BitsPerByte   = 8

	// A key event's state carries the XKB group in bits 13 and 14
	// (XkbGroupForCoreState).
	x11GroupShift = 13

	// x11MaxLayouts is how many layouts a core XKB keymap can hold.
	x11MaxLayouts = 4
	x11GroupMask  = 0x3 << x11GroupShift
)

// x11AutorepeatWarned warns once, because runX11 opens a connection per
// activation and the server's answer does not change.
var x11AutorepeatWarned sync.Once

// x11WarnedLayout is the requested layout the tap last warned the keymap
// lacks, so each mode start does not say it again.
var x11WarnedLayout atomic.Pointer[string]

// x11QueryModifierState queries the server's keyboard state and counts the
// held modifier keys, so modifiers held at grab time count down to zero on
// release instead of going negative and arming detection early.
func x11QueryModifierState(display *C.Display) linuxModifierState {
	var state linuxModifierState

	var keymap [32]C.char
	C.XQueryKeymap(display, &keymap[0])

	// Map X11 modifier keysyms to our canonical modifier names.
	type modifierKeysym struct {
		keysym   C.KeySym
		modifier string
	}
	modifierKeysyms := []modifierKeysym{
		{C.XK_Shift_L, evdevModifierShift},
		{C.XK_Shift_R, evdevModifierShift},
		{C.XK_Control_L, evdevModifierCtrl},
		{C.XK_Control_R, evdevModifierCtrl},
		{C.XK_Alt_L, evdevModifierAlt},
		{C.XK_Alt_R, evdevModifierAlt},
		{C.XK_Super_L, evdevModifierCmd},
		{C.XK_Super_R, evdevModifierCmd},
		{C.XK_Meta_L, evdevModifierCmd},
		{C.XK_Meta_R, evdevModifierCmd},
	}

	for _, modKey := range modifierKeysyms {
		keycode := C.XKeysymToKeycode(display, modKey.keysym)
		if keycode == 0 {
			continue
		}
		idx := int(keycode) / x11BitsPerByte
		bit := int(keycode) % x11BitsPerByte
		if idx < 32 && (keymap[idx]>>uint(bit))&1 != 0 {
			state.update(modKey.modifier, true)
		}
	}

	return state
}

// grabX11Keyboard takes the keyboard, retrying until the key that started the
// mode is released or the mode ends. It has no deadline, since the user may
// hold the hotkey as long as they like. Keys go to the focused window until
// the grab succeeds, so it logs once past x11GrabWarnAfter.
func (et *EventTap) grabX11Keyboard(display *C.Display) bool {
	warnAt := time.Now().Add(x11GrabWarnAfter)
	warned := false

	for {
		status := C.neru_eventtap_grab_keyboard(display)
		if status == C.GrabSuccess {
			return true
		}

		if !warned && time.Now().After(warnAt) && et.logger != nil {
			et.logger.Warn(
				"Still waiting for the X11 keyboard; mode keys reach the focused window until it is free",
				zap.Int("grab_status", int(status)),
				zap.Duration("waited", x11GrabWarnAfter),
			)

			warned = true
		}

		select {
		case <-et.stopCh:
			return false
		case <-time.After(x11PollingInterval):
		}
	}
}

func (et *EventTap) runX11() {
	defer close(et.doneCh)

	// Never grab through XWayland. Compositors break the grab by sending a
	// synthetic Escape or Cancel, which quits modes. run() already routes
	// Wayland elsewhere, so this is a backstop for a direct caller.
	if platform.DetectLinuxBackend().IsWayland() {
		return
	}

	if os.Getenv("DISPLAY") == "" {
		return
	}

	display := C.neru_eventtap_open()
	if display == nil {
		return
	}
	defer C.neru_eventtap_close(display)

	if C.neru_eventtap_set_detectable_autorepeat(display) == 0 {
		x11AutorepeatWarned.Do(func() {
			et.logger.Warn(
				"X server does not support detectable autorepeat; a held key repeats " +
					"at the server's autorepeat rate instead of held_repeat's",
			)
		})
	}

	if !et.grabX11Keyboard(display) {
		return
	}
	defer C.neru_eventtap_ungrab_keyboard(display)

	et.x11Grabbed.Store(true)
	defer et.x11Grabbed.Store(false)

	// Count modifiers already held at grab time. Otherwise their releases
	// drive counts negative and allZero() arms detection while some are still
	// held.
	modState := x11QueryModifierState(display)

	// Each activation opens its own connection, so a layout change takes
	// effect at the next mode start.
	asciiGroups := uint(C.neru_eventtap_ascii_groups(display))
	forcedGroup := et.x11ForcedGroup(display)

	// Keycodes down since the grab. With detectable autorepeat a held key
	// repeats KeyPress, and counting a modifier's repeats would keep modState
	// from returning to zero, so sticky detection would never re-arm.
	down := make(map[C.uint]struct{})

	for {
		select {
		case <-et.stopCh:
			return
		default:
		}

		if C.neru_eventtap_pending(display) == 0 {
			select {
			case <-et.stopCh:
				return
			case request := <-et.yieldCh:
				if !et.yieldX11Keyboard(display, request) {
					return
				}

				// Keys may have gone up or down while the window had them.
				modState = x11QueryModifierState(display)
				x11ForgetReleasedKeys(display, down)
			case <-time.After(x11PollingInterval):
			}

			continue
		}

		var event C.XEvent
		eventType := C.neru_eventtap_next(display, &event)
		if eventType != C.KeyPress && eventType != C.KeyRelease {
			continue
		}

		xkey := (*C.XKeyEvent)(unsafe.Pointer(&event))

		_, wasDown := down[xkey.keycode]
		repeat := eventType == C.KeyPress && wasDown

		if eventType == C.KeyPress {
			down[xkey.keycode] = struct{}{}
		} else {
			delete(down, xkey.keycode)
		}

		buffer := make([]C.char, x11KeyBufferSize)
		var keysym C.KeySym
		length := C.XLookupString(
			xkey,
			&buffer[0],
			C.int(len(buffer)),
			&keysym,
			nil,
		)

		if modifier := x11ModifierName(keysym); modifier != "" {
			if repeat {
				continue
			}

			isDown := eventType == C.KeyPress
			modState.update(modifier, isDown)

			if et.consumeSyntheticModifierEvent(modifier, isDown) {
				continue
			}

			if et.stickyToggleEnabled() && et.stickyDetectionArmed() {
				et.dispatchKey(keyvocab.ModifierToggleEvent(modifier, isDown))
			}

			// Re-arm on a clean slate, so activation-chord releases are not
			// read as sticky toggles.
			if !isDown && !et.stickyDetectionArmed() && modState.allZero() {
				et.stickyArmDetection()
			}

			continue
		}

		if group, ok := x11ReferenceGroup(
			asciiGroups,
			forcedGroup,
			uint(xkey.state&x11GroupMask)>>x11GroupShift,
		); ok {
			length, keysym = x11LookupInGroup(xkey, group, buffer)
		}

		key := x11ChordFromLookup(&modState, length, buffer, keysym)
		if key == "" {
			continue
		}

		if eventType == C.KeyRelease {
			// KeyUpEvent keeps only the base key, so held-key bookkeeping sees
			// the plain name.
			if keyUp := keyvocab.KeyUpEvent(key); keyUp != "" {
				et.dispatchKey(keyUp)
			}

			continue
		}

		et.dispatchKey(key)
	}
}

// yieldX11Keyboard lets go of the keyboard for one YieldKeyboard request and
// takes it back once the caller's keys are out. It reports false when the mode
// ended meanwhile.
//
// x11Grabbed stays set throughout. A key that arrives before the grab is back
// then waits for the tap, rather than going out during the re-grab, which
// would catch it.
func (et *EventTap) yieldX11Keyboard(display *C.Display, request x11Yield) bool {
	C.neru_eventtap_ungrab_keyboard(display)
	close(request.released)

	select {
	case <-request.sent:
	case <-et.stopCh:
		return false
	}

	return et.grabX11Keyboard(display)
}

// x11ForgetReleasedKeys drops from down every key the server no longer reads
// as held, since its release went to the focused window.
func x11ForgetReleasedKeys(display *C.Display, down map[C.uint]struct{}) {
	var keymap [32]C.char
	C.XQueryKeymap(display, &keymap[0])

	for keycode := range down {
		if (keymap[keycode/x11BitsPerByte]>>(keycode%x11BitsPerByte))&1 == 0 {
			delete(down, keycode)
		}
	}
}

// x11ReferenceGroup picks the layout a key is named in, given the mask of
// ASCII-capable layouts, the layout general.kb_layout_to_use forces (-1 for
// none) and the active one. A forced layout wins. Otherwise the active layout
// wins when ASCII-capable, as on macOS, so a Dvorak user gets Dvorak, else the
// first ASCII-capable one does, so a Russian speaker keeps bindings on their
// keys. It reports false when the key is already named in the right layout.
func x11ReferenceGroup(asciiGroups uint, forced int, active uint) (uint, bool) {
	if forced >= 0 {
		return uint(forced), uint(forced) != active
	}

	if asciiGroups == 0 || asciiGroups&(1<<active) != 0 {
		return 0, false
	}

	return uint(bits.TrailingZeros(asciiGroups)), true
}

// x11LayoutNames lists the XKB names of the server keymap's layouts, in order.
func x11LayoutNames(display *C.Display) []string {
	cNames := make([]*C.char, x11MaxLayouts)

	count := int(
		C.neru_eventtap_layout_names(display, unsafe.SliceData(cNames), C.int(len(cNames))),
	)
	names := make([]string, count)

	for i := range count {
		names[i] = C.GoString(cNames[i])
		C.free(unsafe.Pointer(cNames[i]))
	}

	return names
}

// x11ForcedGroup returns the index of the layout general.kb_layout_to_use
// names, or -1 when it names none or the keymap lacks it. A missing name is
// logged once per configured value.
func (et *EventTap) x11ForcedGroup(display *C.Display) int {
	requested := requestedKeyboardLayout.Load()
	if requested == nil || *requested == "" {
		return -1
	}

	names := x11LayoutNames(display)
	if index := layoutIndex(names, *requested); index >= 0 {
		return index
	}

	if x11WarnedLayout.Swap(requested) != requested && et.logger != nil {
		et.logger.Warn(
			"Configured keyboard layout is not in the X server's keymap; using automatic selection",
			zap.String("layout_id", *requested),
			zap.Int("layouts", len(names)),
		)
	}

	return -1
}

// x11KeyboardLayouts lists the server keymap's layouts and names the one keys
// resolve in, for neru doctor. It opens a connection of its own.
func x11KeyboardLayouts() KeyboardLayouts {
	display := C.neru_eventtap_open()
	if display == nil {
		return KeyboardLayouts{}
	}
	defer C.neru_eventtap_close(display)

	layouts := KeyboardLayouts{Names: x11LayoutNames(display)}

	forced := -1
	if requested := RequestedKeyboardLayout(); requested != "" {
		forced = layoutIndex(layouts.Names, requested)
		layouts.Unmatched = forced < 0
	}

	active := int(C.neru_eventtap_active_group(display))
	asciiGroups := uint(C.neru_eventtap_ascii_groups(display))

	if active < 0 || (forced < 0 && asciiGroups == 0) {
		return layouts
	}

	group, renamed := x11ReferenceGroup(asciiGroups, forced, uint(active))
	if !renamed {
		group = uint(active)
	}

	if int(group) < len(layouts.Names) {
		layouts.Reference = layouts.Names[group]
	}

	return layouts
}

// x11LookupInGroup resolves a key event as if group were the active layout, so
// a binding stays on its physical key under another language. The live lookup
// still decides whether the key is a modifier, because an XKB remap such as
// ctrl:swapcaps belongs to the layout that carries it.
func x11LookupInGroup(xkey *C.XKeyEvent, group uint, buffer []C.char) (C.int, C.KeySym) {
	xkey.state = xkey.state&^x11GroupMask | C.uint(group)<<x11GroupShift

	var keysym C.KeySym
	length := C.XLookupString(xkey, &buffer[0], C.int(len(buffer)), &keysym, nil)

	return length, keysym
}

// x11ChordFromLookup names a key press the way the evdev backend does: the
// held modifiers in canonical order, then the key. Without it no Ctrl, Alt or
// Super chord in `[<mode>.hotkeys]` could match on X11.
//
// The base key comes from the keysym, not XLookupString's string, because with
// Ctrl held the string is a control character (Ctrl+C gives \x03) while the
// keysym is still XK_c. The keysym is state-resolved, so Shift has chosen the
// level, as xkb_state_key_get_one_sym does for evdev, and both backends agree
// on what Shift+; is called.
//
// A keysym outside Latin-1 falls back to the server's character, unprefixed,
// so a non-Latin layout still dispatches its keys.
func x11ChordFromLookup(
	modState *linuxModifierState,
	length C.int,
	buffer []C.char,
	keysym C.KeySym,
) string {
	if base := x11BaseKeyName(uint32(keysym)); base != "" {
		return x11ChordName(modState, base)
	}

	return keyvocab.NormalizeKey(x11KeyFromLookup(length, buffer, keysym))
}

// x11ChordName joins the held modifiers to their key. It is the whole chord
// spelling, kept apart from cgo so it can be tested.
func x11ChordName(modState *linuxModifierState, base string) string {
	if base == "" {
		return ""
	}

	return keyvocab.NormalizeKey(modState.prefix() + base)
}

// x11BaseKeyName names the key a keysym stands for, ignoring held modifiers:
// the named keys first, then any Latin-1 keysym, which is its own code point
// by the X11 protocol. Everything else answers "".
func x11BaseKeyName(keysym uint32) string {
	// The named keys win. XK_space is 0x20 and has to be "Space" rather than " ".
	if name := x11KeysymName(C.KeySym(keysym)); name != "" {
		return name
	}

	const (
		asciiPrintableFirst = 0x20
		asciiPrintableLast  = 0x7e
		latin1First         = 0xa0
		latin1Last          = 0xff
	)

	if (keysym >= asciiPrintableFirst && keysym <= asciiPrintableLast) ||
		(keysym >= latin1First && keysym <= latin1Last) {
		return string(rune(keysym))
	}

	return ""
}

func x11KeyFromLookup(length C.int, buffer []C.char, keysym C.KeySym) string {
	var key string
	if length > 0 {
		key = C.GoStringN(&buffer[0], length)
	} else {
		key = x11KeysymName(keysym)
	}

	return keyvocab.NormalizeKey(key)
}

// x11KeypadBeginName is the keypad center key with NumLock off. It has no
// navigation function, and "5" is the name the Wayland keymap gives it.
const x11KeypadBeginName = "5"

// x11KeysymName maps the keysyms XLookupString gives no character for onto
// the names Neru binds. A key with no entry is dropped.
//
// With NumLock off the keypad reports its own keysyms, which fold onto the
// main-keyboard names neru_normalize_xkb_name uses
// (internal/adapter/platform/linux/wayland_keymap.c), so one physical key
// reaches one binding on every Linux backend. With NumLock on, KP_0 to KP_9
// and the operators resolve to characters and deliberately have no case here.
func x11KeysymName(keysym C.KeySym) string {
	switch keysym {
	case C.XK_Return:
		return evdevKeyNameReturn
	case C.XK_space:
		return "Space"
	// Shift turns Tab into ISO_Left_Tab, and Shift+Tab bindings must reach it.
	case C.XK_Tab, C.XK_ISO_Left_Tab:
		return evdevKeyNameTab
	case C.XK_Escape:
		return evdevKeyNameEscape
	case C.XK_BackSpace:
		return "Backspace"
	case C.XK_Left, C.XK_KP_Left:
		return evdevKeyNameLeft
	case C.XK_Right, C.XK_KP_Right:
		return "Right"
	case C.XK_Up, C.XK_KP_Up:
		return "Up"
	case C.XK_Down, C.XK_KP_Down:
		return "Down"
	// Navigation keys produce no character from XLookupString, so this is the
	// only way they reach the handler. Names match the evdev backend.
	case C.XK_Home, C.XK_KP_Home:
		return evdevKeyNameHome
	case C.XK_End, C.XK_KP_End:
		return evdevKeyNameEnd
	// Prior and Next are the page keys, the name xkbcommon lists first and
	// the Wayland fold table uses.
	case C.XK_Prior, C.XK_KP_Prior:
		return evdevKeyNamePageUp
	case C.XK_Next, C.XK_KP_Next:
		return evdevKeyNamePageDown
	case C.XK_Insert, C.XK_KP_Insert:
		return evdevKeyNameInsert
	// Keypad-only. XLookupString answers the main Delete key with a
	// character, and the keypad center key has no main-keyboard twin.
	case C.XK_KP_Delete:
		return evdevKeyNameDelete
	case C.XK_KP_Begin:
		return x11KeypadBeginName
	default:
		return x11FunctionKeysymName(keysym)
	}
}

// x11FunctionKeysymName maps the F1-F24 keysyms to their Neru names, the only
// way function keys reach the handler.
func x11FunctionKeysymName(keysym C.KeySym) string {
	switch keysym {
	case C.XK_F1:
		return evdevKeyNameF1
	case C.XK_F2:
		return evdevKeyNameF2
	case C.XK_F3:
		return evdevKeyNameF3
	case C.XK_F4:
		return evdevKeyNameF4
	case C.XK_F5:
		return evdevKeyNameF5
	case C.XK_F6:
		return evdevKeyNameF6
	case C.XK_F7:
		return evdevKeyNameF7
	case C.XK_F8:
		return evdevKeyNameF8
	case C.XK_F9:
		return evdevKeyNameF9
	case C.XK_F10:
		return evdevKeyNameF10
	case C.XK_F11:
		return evdevKeyNameF11
	case C.XK_F12:
		return evdevKeyNameF12
	case C.XK_F13:
		return evdevKeyNameF13
	case C.XK_F14:
		return evdevKeyNameF14
	case C.XK_F15:
		return evdevKeyNameF15
	case C.XK_F16:
		return evdevKeyNameF16
	case C.XK_F17:
		return evdevKeyNameF17
	case C.XK_F18:
		return evdevKeyNameF18
	case C.XK_F19:
		return evdevKeyNameF19
	case C.XK_F20:
		return evdevKeyNameF20
	case C.XK_F21:
		return evdevKeyNameF21
	case C.XK_F22:
		return evdevKeyNameF22
	case C.XK_F23:
		return evdevKeyNameF23
	case C.XK_F24:
		return evdevKeyNameF24
	default:
		return ""
	}
}

func x11ModifierName(keysym C.KeySym) string {
	switch keysym {
	case C.XK_Shift_L, C.XK_Shift_R:
		return evdevModifierShift
	case C.XK_Control_L, C.XK_Control_R:
		return evdevModifierCtrl
	case C.XK_Alt_L, C.XK_Alt_R:
		return evdevModifierAlt
	case C.XK_Super_L, C.XK_Super_R, C.XK_Meta_L, C.XK_Meta_R:
		return evdevModifierCmd
	default:
		return ""
	}
}

// postLinuxModifierEvent injects a modifier through the running backend: the
// Wayland virtual keyboard, or XTest on X11. It asks the same detector
// PostModifierEvent does, so the two cannot disagree.
func postLinuxModifierEvent(modifier string, isDown bool) bool {
	if platform.DetectLinuxBackend().IsWayland() {
		return linux.WaylandModifierEvent(modifier, isDown) == nil
	}

	if os.Getenv("DISPLAY") == "" {
		return false
	}

	cModifier := C.CString(modifier)
	defer C.free(unsafe.Pointer(cModifier))

	cDown := C.int(0)
	if isDown {
		cDown = C.int(1)
	}

	return C.neru_eventtap_post_modifier(cModifier, cDown) != 0
}

// layoutIndex returns the position of name among names, ignoring case, or -1.
func layoutIndex(names []string, name string) int {
	return slices.IndexFunc(names, func(candidate string) bool {
		return strings.EqualFold(candidate, name)
	})
}
