//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: x11
#include <stdlib.h>
#include "../../platform/linux/x11_hotkeys.h"
*/
import "C"

import (
	"strings"
	"sync"
	"time"
	"unsafe"

	_ "github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
	"github.com/y3owk1n/neru/internal/ports"
)

const x11HotkeyPollInterval = 10 * time.Millisecond

type x11HotkeyBinding struct {
	keycode   C.int
	modifiers C.uint
}

type x11HotkeyState struct {
	display  *C.Display
	root     C.Window
	bindings map[ports.HotkeyID]x11HotkeyBinding
	ids      map[string]ports.HotkeyID
	stopCh   chan struct{} // signals runX11HotkeyLoop to exit
	doneCh   chan struct{} // closed when runX11HotkeyLoop has exited
	once     sync.Once
}

var x11States sync.Map

func (m *Manager) registerX11Hotkey(hotkeyID ports.HotkeyID, keyString string) error {
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

	state.bindings[hotkeyID] = x11HotkeyBinding{keycode: C.int(keycode), modifiers: modifiers}
	state.ids[x11BindingKey(keycode, modifiers)] = hotkeyID

	return nil
}

func (m *Manager) unregisterX11Hotkey(hotkeyID ports.HotkeyID) {
	stateAny, ok := x11States.Load(m)
	if !ok {
		return
	}
	state, ok := stateAny.(*x11HotkeyState)
	if !ok {
		return
	}

	binding, exists := state.bindings[hotkeyID]
	if !exists {
		return
	}

	for _, mask := range []C.uint{0, C.Mod2Mask, C.LockMask, C.Mod2Mask | C.LockMask} {
		C.XUngrabKey(
			state.display,
			binding.keycode,
			binding.modifiers|mask,
			state.root,
		)
	}
	C.XFlush(state.display)

	delete(state.ids, x11BindingKey(C.uint(binding.keycode), binding.modifiers))
	delete(state.bindings, hotkeyID)
}

func (m *Manager) unregisterAllX11Hotkeys() {
	stateAny, ok := x11States.Load(m)
	if !ok {
		return
	}
	state, ok := stateAny.(*x11HotkeyState)
	if !ok {
		return
	}

	for id := range state.bindings {
		m.unregisterX11Hotkey(id)
	}

	state.once.Do(func() {
		// Signal the event loop to stop and wait for it to exit
		// before closing the display, preventing a use-after-free.
		close(state.stopCh)
		<-state.doneCh

		C.XCloseDisplay(state.display)
		x11States.Delete(m)
	})
}

func (m *Manager) ensureX11State() (*x11HotkeyState, error) {
	if stateAny, ok := x11States.Load(m); ok {
		state, ok := stateAny.(*x11HotkeyState)
		if !ok {
			return nil, derrors.New(derrors.CodeHotkeyRegisterFailed, "invalid X11 state type")
		}

		return state, nil
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
		bindings: make(map[ports.HotkeyID]x11HotkeyBinding),
		ids:      make(map[string]ports.HotkeyID),
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	x11States.Store(m, state)
	go m.runX11HotkeyLoop(state)

	return state, nil
}

func (m *Manager) runX11HotkeyLoop(state *x11HotkeyState) {
	defer close(state.doneCh)

	for {
		select {
		case <-state.stopCh:
			return
		default:
		}

		// Use XPending to check for queued events instead of calling the
		// blocking XNextEvent directly. This allows the stop channel to be
		// checked between iterations, preventing a goroutine leak and a
		// use-after-free when unregisterAllX11Hotkeys closes the display.
		if C.neru_hotkeys_pending(state.display) == 0 {
			time.Sleep(x11HotkeyPollInterval)

			continue
		}

		var event C.XEvent
		C.XNextEvent(state.display, &event)
		if C.neru_xevent_type(&event) != C.KeyPress {
			continue
		}

		keycode := C.neru_xkey_keycode(&event)
		modifiers := C.neru_xkey_state(&event) &^ (C.Mod2Mask | C.LockMask)

		// Hold m.mu while reading state.ids — Register/Unregister write
		// to this map under the same lock, so an unguarded read here is a
		// concurrent map read/write (runtime crash under the race detector).
		// We also fetch the callback in the same critical section to avoid
		// a second lock acquisition via callbackFor.
		m.mu.RLock()
		id, ok := state.ids[x11BindingKey(keycode, modifiers)]

		var callback ports.HotkeyCallback
		if ok {
			callback = m.callbacks[id]
		}

		m.mu.RUnlock()

		if callback != nil {
			go callback()
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

// x11KeysymFor resolves the base key of a hotkey to the X11 keysym a grab
// needs. The keys below are mapped here rather than left to XStringToKeysym,
// which knows X11's own spellings ("Page_Up", "Prior") and not Neru's: a name
// from the vocabulary resolves only where this switch says it does, and "Home"
// and "End" landing on the right keysym through XStringToKeysym is a
// coincidence of spelling, not a contract.
//
// The rest fall through to XStringToKeysym: punctuation, and the named keys
// X11 spells the same way Neru does — Delete, Insert and F1-F24. "Backspace"
// is the one named key neither path resolves, since X11 spells it "BackSpace";
// that gap predates this switch and is untouched here.
func x11KeysymFor(key string) C.KeySym {
	key = strings.TrimSpace(key)
	if len(key) == 1 {
		letter := strings.ToLower(key)
		cKey := C.CString(letter)

		defer C.free(unsafe.Pointer(cKey))

		return C.XStringToKeysym(cKey)
	}

	name := x11CanonicalKeyName(key)

	switch name {
	case keyvocab.KeySpace:
		return C.XK_space
	case keyvocab.KeyReturn, keyvocab.KeyEnter:
		return C.XK_Return
	case keyvocab.KeyTab:
		return C.XK_Tab
	case keyvocab.KeyEscape:
		return C.XK_Escape
	case keyvocab.KeyUp:
		return C.XK_Up
	case keyvocab.KeyDown:
		return C.XK_Down
	case keyvocab.KeyLeft:
		return C.XK_Left
	case keyvocab.KeyRight:
		return C.XK_Right
	case keyvocab.KeyHome:
		return C.XK_Home
	case keyvocab.KeyEnd:
		return C.XK_End
	case keyvocab.KeyPageUp:
		return C.XK_Page_Up
	case keyvocab.KeyPageDown:
		return C.XK_Page_Down
	default:
		cKey := C.CString(name)
		defer C.free(unsafe.Pointer(cKey))

		return C.XStringToKeysym(cKey)
	}
}

// x11CanonicalKeyName gives a written key name its vocabulary spelling
// ("pageup" -> "PageUp"), so the switch above compares against the named-key
// declaration instead of a second set of lowercase literals. A name the
// vocabulary does not know is returned unchanged for XStringToKeysym to try.
//
// This is keyvocab.NormalizeKey minus the alias fold, which is how the hotkey
// strings reaching this adapter were already canonicalized — config's
// CanonicalHotkeyForPlatform display-cases the base key without folding
// aliases — and it is what a grab needs: a grab names a physical key, and the
// vocabulary's aliases cross keys that X11 keeps apart. Folding "Backspace" to
// "Delete" the way the taps do would resolve XK_Delete and grab the
// forward-delete key for a binding written "Backspace". So a named key keeps
// its own spelling here — "Enter" does not become "Return", though both are
// mapped above — and only "esc", which the vocabulary deliberately keeps out of
// the named-key set, resolves through its alias.
func x11CanonicalKeyName(key string) string {
	if display, isNamed := keyvocab.NamedKeyDisplay(key); isNamed {
		return display
	}

	if means, isAlias := keyvocab.ResolveAlias(key); isAlias {
		return means
	}

	return key
}

func x11BindingKey(keycode C.uint, modifiers C.uint) string {
	return strings.Join([]string{itoa(int(keycode)), itoa(int(modifiers))}, ":")
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}

	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}

	var buf [20]byte
	index := len(buf)
	for value > 0 {
		index--
		buf[index] = byte('0' + value%10)
		value /= 10
	}

	return sign + string(buf[index:])
}
