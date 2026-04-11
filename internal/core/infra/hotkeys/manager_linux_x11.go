//go:build linux && cgo

package hotkeys

/*
#cgo linux LDFLAGS: -lX11
#include <X11/Xlib.h>
#include <X11/keysym.h>
#include <stdlib.h>

static Window neru_hotkeys_root_window(Display *display) {
	return RootWindow(display, DefaultScreen(display));
}

static int neru_xevent_type(XEvent *ev) {
	return ev->type;
}

static unsigned int neru_xkey_keycode(XEvent *ev) {
	return ev->xkey.keycode;
}

static unsigned int neru_xkey_state(XEvent *ev) {
	return ev->xkey.state;
}
*/
import "C"

import (
	"strings"
	"sync"
	"unsafe"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

type x11HotkeyBinding struct {
	keycode   C.int
	modifiers C.uint
}

type x11HotkeyState struct {
	display  *C.Display
	root     C.Window
	bindings map[HotkeyID]x11HotkeyBinding
	ids      map[string]HotkeyID
	done     chan struct{}
	once     sync.Once
}

var x11States sync.Map

func (m *Manager) registerX11Hotkey(id HotkeyID, keyString string) error {
	state, err := m.ensureX11State()
	if err != nil {
		return err
	}

	keycode, modifiers, parseErr := parseX11Hotkey(state.display, keyString)
	if parseErr != nil {
		return parseErr
	}

	for _, mask := range []C.uint{0, C.Mod2Mask, C.LockMask, C.Mod2Mask | C.LockMask} {
		C.XGrabKey(
			state.display,
			C.int(keycode),
			modifiers|mask,
			state.root,
			C.True,
			C.GrabModeAsync,
			C.GrabModeAsync,
		)
	}
	C.XSelectInput(state.display, state.root, C.KeyPressMask)
	C.XFlush(state.display)

	state.bindings[id] = x11HotkeyBinding{keycode: C.int(keycode), modifiers: modifiers}
	state.ids[x11BindingKey(keycode, modifiers)] = id

	return nil
}

func (m *Manager) unregisterX11Hotkey(id HotkeyID) {
	stateAny, ok := x11States.Load(m)
	if !ok {
		return
	}
	state := stateAny.(*x11HotkeyState)

	binding, exists := state.bindings[id]
	if !exists {
		return
	}

	for _, mask := range []C.uint{0, C.Mod2Mask, C.LockMask, C.Mod2Mask | C.LockMask} {
		C.XUngrabKey(state.display, binding.keycode, binding.modifiers|mask, state.root)
	}
	C.XFlush(state.display)

	delete(state.ids, x11BindingKey(C.uint(binding.keycode), binding.modifiers))
	delete(state.bindings, id)
}

func (m *Manager) unregisterAllX11Hotkeys() {
	stateAny, ok := x11States.Load(m)
	if !ok {
		return
	}
	state := stateAny.(*x11HotkeyState)

	for id := range state.bindings {
		m.unregisterX11Hotkey(id)
	}

	state.once.Do(func() {
		close(state.done)
		C.XCloseDisplay(state.display)
		x11States.Delete(m)
	})
}

func (m *Manager) ensureX11State() (*x11HotkeyState, error) {
	if stateAny, ok := x11States.Load(m); ok {
		return stateAny.(*x11HotkeyState), nil
	}

	display := C.XOpenDisplay(nil)
	if display == nil {
		return nil, derrors.New(
			derrors.CodeHotkeyRegisterFailed,
			"failed to open X11 display for global hotkeys",
		)
	}

	state := &x11HotkeyState{
		display:  display,
		root:     C.neru_hotkeys_root_window(display),
		bindings: make(map[HotkeyID]x11HotkeyBinding),
		ids:      make(map[string]HotkeyID),
		done:     make(chan struct{}),
	}
	x11States.Store(m, state)
	go m.runX11HotkeyLoop(state)

	return state, nil
}

func (m *Manager) runX11HotkeyLoop(state *x11HotkeyState) {
	for {
		select {
		case <-state.done:
			return
		default:
		}

		var event C.XEvent
		C.XNextEvent(state.display, &event)
		if C.neru_xevent_type(&event) != C.KeyPress {
			continue
		}

		keycode := C.neru_xkey_keycode(&event)
		modifiers := C.neru_xkey_state(&event) &^ (C.Mod2Mask | C.LockMask)

		if id, ok := state.ids[x11BindingKey(keycode, modifiers)]; ok {
			if callback := m.callbackFor(id); callback != nil {
				go callback()
			}
		}
	}
}

func parseX11Hotkey(display *C.Display, keyString string) (C.uint, C.uint, error) {
	parts := strings.Split(keyString, "+")
	if len(parts) == 0 {
		return 0, 0, derrors.Newf(derrors.CodeInvalidInput, "invalid hotkey: %q", keyString)
	}

	var modifiers C.uint
	keyPart := strings.TrimSpace(parts[len(parts)-1])
	for _, part := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "cmd", "command", "super", "meta":
			modifiers |= C.Mod4Mask
		case "ctrl", "control", "primary":
			modifiers |= C.ControlMask
		case "shift":
			modifiers |= C.ShiftMask
		case "alt", "option":
			modifiers |= C.Mod1Mask
		case "":
		default:
			return 0, 0, derrors.Newf(
				derrors.CodeInvalidInput,
				"unsupported X11 hotkey modifier %q in %q",
				part,
				keyString,
			)
		}
	}

	keysym := x11KeysymFor(keyPart)
	if keysym == 0 {
		return 0, 0, derrors.Newf(
			derrors.CodeInvalidInput,
			"unsupported X11 hotkey key %q in %q",
			keyPart,
			keyString,
		)
	}

	keycode := C.XKeysymToKeycode(display, keysym)
	if keycode == 0 {
		return 0, 0, derrors.Newf(
			derrors.CodeInvalidInput,
			"failed to resolve X11 keycode for %q",
			keyString,
		)
	}

	return C.uint(keycode), modifiers, nil
}

func x11KeysymFor(key string) C.KeySym {
	key = strings.TrimSpace(key)
	if len(key) == 1 {
		letter := strings.ToLower(key)
		cKey := C.CString(letter)
		defer C.free(unsafe.Pointer(cKey)) //nolint:nlreturn
		return C.XStringToKeysym(cKey)
	}

	switch strings.ToLower(key) {
	case "space":
		return C.XK_space
	case "return", "enter":
		return C.XK_Return
	case "tab":
		return C.XK_Tab
	case "escape", "esc":
		return C.XK_Escape
	case "up":
		return C.XK_Up
	case "down":
		return C.XK_Down
	case "left":
		return C.XK_Left
	case "right":
		return C.XK_Right
	default:
		cKey := C.CString(key)
		defer C.free(unsafe.Pointer(cKey)) //nolint:nlreturn
		return C.XStringToKeysym(cKey)
	}
}

func x11BindingKey(keycode C.uint, modifiers C.uint) string {
	return strings.Join([]string{itoa(int(keycode)), itoa(int(modifiers))}, ":")
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}

	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}

	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}

	return sign + string(buf[i:])
}
