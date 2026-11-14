//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: x11
#include <stdlib.h>
#include "../../platform/linux/x11_hotkeys.h"
*/
import "C"

import (
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
	_ "github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
	"github.com/y3owk1n/neru/internal/ports"
)

const x11HotkeyPollInterval = 10 * time.Millisecond

// x11CoreModifierMask keeps the eight core modifier bits of a key event's
// state: Shift, Lock, Control and Mod1 through Mod5.
const x11CoreModifierMask = 0xFF

type x11HotkeyBinding struct {
	keycode   C.int
	modifiers C.uint
}

type x11HotkeyState struct {
	// displayMu serializes every Xlib call made on display. Xlib is
	// thread-safe only after XInitThreads, which nothing here calls, and
	// Manager.mu guards only the Go maps. Without this lock a registration
	// and the poll loop's XPending would share the connection buffers.
	//
	// It is never held across a call that waits for a key. The loop calls
	// XNextEvent only when neru_hotkeys_pending says an event is queued, so a
	// registration waits at most one non-blocking read.
	displayMu sync.Mutex

	display  *C.Display
	root     C.Window
	bindings map[ports.HotkeyID]x11HotkeyBinding
	ids      map[string]ports.HotkeyID
	// held is owned by the poll loop, the only goroutine reading key events.
	// dispatch runs the callbacks it fires, in order, off that goroutine.
	held     x11HeldHotkeys
	dispatch *eventtaplinux.HotkeyDispatcher
	stopCh   chan struct{} // signals runX11HotkeyLoop to exit
	doneCh   chan struct{} // closed when runX11HotkeyLoop has exited
	once     sync.Once
}

// x11IgnoredModifierMasks are the lock modifiers a grab must be repeated
// under, because a grab only matches the exact state the server reports. Lock
// is CapsLock and Mod2 is usually NumLock. Stated once so grab and ungrab
// cannot drift, since a missed mask leaves a forgotten hotkey grabbed.
func x11IgnoredModifierMasks() [4]C.uint {
	return [4]C.uint{0, C.Mod2Mask, C.LockMask, C.Mod2Mask | C.LockMask}
}

// grab resolves keyString against the connection's keymap and installs the
// grabs for it, as one critical section. XKeysymToKeycode is an Xlib call on
// this connection, so it belongs inside the lock.
func (s *x11HotkeyState) grab(keyString string) (x11HotkeyBinding, error) {
	s.displayMu.Lock()
	defer s.displayMu.Unlock()

	keycode, modifiers, err := parseX11Hotkey(s.display, keyString)
	if err != nil {
		return x11HotkeyBinding{}, err
	}

	for _, mask := range x11IgnoredModifierMasks() {
		C.XGrabKey(
			s.display,
			C.int(keycode),
			modifiers|mask,
			s.root,
			C.True,
			C.GrabModeAsync,
			C.GrabModeAsync,
		)
	}

	C.XSelectInput(s.display, s.root, C.KeyPressMask|C.KeyReleaseMask)
	C.XFlush(s.display)

	return x11HotkeyBinding{keycode: C.int(keycode), modifiers: modifiers}, nil
}

// ungrab releases the grabs grab installed for one binding.
func (s *x11HotkeyState) ungrab(binding x11HotkeyBinding) {
	s.displayMu.Lock()
	defer s.displayMu.Unlock()

	for _, mask := range x11IgnoredModifierMasks() {
		C.XUngrabKey(
			s.display,
			binding.keycode,
			binding.modifiers|mask,
			s.root,
		)
	}

	C.XFlush(s.display)
}

// nextEvent takes one queued event off the connection, reporting false when
// none was waiting. The pending check and the read are one critical section,
// because an XNextEvent whose event another thread took blocks for the next
// one, with displayMu held, wedging registration.
func (s *x11HotkeyState) nextEvent() (C.XEvent, bool) {
	s.displayMu.Lock()
	defer s.displayMu.Unlock()

	var event C.XEvent

	if C.neru_hotkeys_pending(s.display) == 0 {
		return event, false
	}

	C.XNextEvent(s.display, &event)

	return event, true
}

// closeDisplay tears down the connection. Callers must stop the poll loop
// first. The lock orders this against a registration only.
func (s *x11HotkeyState) closeDisplay() {
	s.displayMu.Lock()
	defer s.displayMu.Unlock()

	C.XCloseDisplay(s.display)
}

var x11States sync.Map

func (m *Manager) registerX11Hotkey(hotkeyID ports.HotkeyID, keyString string) error {
	state, err := m.ensureX11State()
	if err != nil {
		return err
	}

	binding, grabErr := state.grab(keyString)
	if grabErr != nil {
		return grabErr
	}

	state.bindings[hotkeyID] = binding
	state.ids[x11BindingKey(C.uint(binding.keycode), binding.modifiers)] = hotkeyID

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

	state.ungrab(binding)

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
		// Stop the event loop and wait for it before closing the display, to
		// avoid a use-after-free.
		close(state.stopCh)
		<-state.doneCh

		state.dispatch.Stop()
		state.closeDisplay()
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

	// XOpenDisplay and the root-window read skip displayMu safely, because
	// they run before the state is published or the poll loop starts.
	display := C.XOpenDisplay(nil)
	if display == nil {
		return nil, derrors.New(
			derrors.CodeHotkeyRegisterFailed,
			"failed to open X11 display for global hotkeys",
		)
	}

	// Without detectable autorepeat a held key arrives as release/press pairs
	// and every release ends the hold. With it, the hold is repeated KeyPress
	// and one KeyRelease, the edge the binder repeats on. It is per
	// connection, so the in-mode tap sets it too (platform/linux/x11_eventtap.c).
	if C.neru_hotkeys_set_detectable_autorepeat(display) == 0 {
		m.logger.Warn(
			"X server does not support detectable autorepeat; a held global hotkey " +
				"repeats at the server's autorepeat rate instead of held_repeat's",
		)
	}

	state := &x11HotkeyState{
		display:  display,
		root:     C.neru_hotkeys_root_window(display),
		bindings: make(map[ports.HotkeyID]x11HotkeyBinding),
		ids:      make(map[string]ports.HotkeyID),
		held:     make(x11HeldHotkeys),
		dispatch: eventtaplinux.NewHotkeyDispatcher(),
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

		// nextEvent polls rather than blocking in XNextEvent, so the stop
		// channel is checked between iterations (no leak or use-after-free
		// when unregisterAllX11Hotkeys closes the display) and registration
		// never waits on a keystroke.
		event, hasEvent := state.nextEvent()
		if !hasEvent {
			time.Sleep(x11HotkeyPollInterval)

			continue
		}

		keycode := C.neru_xkey_keycode(&event)

		switch C.neru_xevent_type(&event) {
		case C.KeyPress:
			// Only the core modifiers name a binding. The state also carries
			// the XKB layout in bits 13 and 14, and a grab fires on any layout.
			modifiers := C.neru_xkey_state(
				&event,
			) & x11CoreModifierMask &^ (C.Mod2Mask | C.LockMask)

			// Register and Unregister write state.ids under m.mu, so read it
			// (and the callback) under the same lock.
			m.mu.RLock()
			hotkeyID, bound := state.ids[x11BindingKey(keycode, modifiers)]

			var callbacks hotkeyCallbacks
			if bound {
				callbacks = m.callbacks[hotkeyID]
			}

			m.mu.RUnlock()

			if !bound || !state.held.press(uint32(keycode), hotkeyID) {
				continue
			}

			if callbacks.press != nil {
				state.dispatch.Dispatch(callbacks.press)
			}
		case C.KeyRelease:
			hotkeyID, wasDown := state.held.release(uint32(keycode))
			if !wasDown {
				continue
			}

			m.mu.RLock()
			release := m.callbacks[hotkeyID].release
			m.mu.RUnlock()

			if release != nil {
				state.dispatch.Dispatch(release)
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

	// A forced layout decides the grab too, or a user forcing Dvorak would get
	// hotkeys on QWERTY keys while mode keys follow Dvorak.
	if layout := eventtaplinux.RequestedKeyboardLayout(); layout != "" {
		cLayout := C.CString(layout)
		defer C.free(unsafe.Pointer(cLayout))

		if forced := C.neru_hotkeys_keycode_in_layout(display, keysym, cLayout); forced != 0 {
			keycode = forced
		}
	}

	if keycode == 0 {
		return 0, 0, derrors.Newf(
			derrors.CodeInvalidInput,
			"failed to resolve X11 keycode for %q",
			keyString,
		)
	}

	return C.uint(keycode), modifiers, nil
}

// x11KeysymFor resolves the base key of a hotkey to the keysym a grab needs.
// The keys below are mapped here because XStringToKeysym knows X11 spellings
// ("Page_Up", "Prior"), not Neru's. "Home" and "End" matching by spelling is
// a coincidence, not a contract.
//
// "Backspace" (X11 "BackSpace") and "Delete" both map to the backspace key on
// purpose. In [hotkeys] "Delete" means backspace on every platform (kVK_Delete
// on macOS, VK_BACK on Windows), while X11's "Delete" is forward-delete.
// Forward-delete has no hotkey name, and one should be added for all three
// platforms, not as a Linux-only meaning. Insert sits with them so the
// editing keys are in one place.
//
// A single printable character is its own keysym, because the Latin-1 keysyms
// share their character codes. XStringToKeysym would want the X11 name
// ("semicolon", "colon"), which only matches the character for letters and
// digits. F1-F24 fall through to XStringToKeysym, where X11's name is Neru's.
// Tests pin the symbols and the function keys.
func x11KeysymFor(key string) C.KeySym {
	key = strings.TrimSpace(key)
	if len(key) == 1 {
		if char := strings.ToLower(key)[0]; char > ' ' && char <= '~' {
			return C.KeySym(char)
		}

		return 0
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
	case keyvocab.KeyBackspace, keyvocab.KeyDelete:
		return C.XK_BackSpace
	case keyvocab.KeyInsert:
		return C.XK_Insert
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
// declaration. An unknown name is returned unchanged for XStringToKeysym.
//
// This is keyvocab.NormalizeKey without the alias fold, matching how
// keyvocab.CanonicalHotkeyForPlatform canonicalizes hotkeys. A grab names a
// physical key, and folding "Backspace" to "Delete" as the taps do once made
// it grab forward-delete. So "Enter" stays "Enter", and only "esc", which the
// vocabulary keeps out of the named-key set, resolves through its alias.
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
	return strings.Join([]string{strconv.Itoa(int(keycode)), strconv.Itoa(int(modifiers))}, ":")
}
