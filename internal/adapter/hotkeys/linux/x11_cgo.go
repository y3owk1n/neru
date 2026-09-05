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

	eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
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
	// displayMu serializes every Xlib call made on display.
	//
	// Xlib is thread-safe only when XInitThreads runs before the first
	// XOpenDisplay, and nothing in this process calls it. Manager.mu guards the
	// Go maps and says nothing about the connection, so without this lock a
	// registration on a caller's goroutine and the poll loop on its own would
	// be inside the same connection buffers at once: XPending flushes the
	// output buffer and reads from the socket, which is exactly what the
	// XGrabKey/XSelectInput/XFlush sequence beside it is doing.
	//
	// It is never held across a call that waits for a key. The loop polls with
	// neru_hotkeys_pending and only calls XNextEvent when an event is already
	// queued, so the longest a registration can wait behind it is one
	// non-blocking read — holding it across a bare XNextEvent would wedge
	// registration until the user pressed something.
	displayMu sync.Mutex

	display  *C.Display
	root     C.Window
	bindings map[ports.HotkeyID]x11HotkeyBinding
	ids      map[string]ports.HotkeyID
	// held is owned by the poll loop, the only goroutine that reads key events,
	// and dispatch runs the callbacks it fires, in order, off it.
	held     x11HeldHotkeys
	dispatch *eventtaplinux.HotkeyDispatcher
	stopCh   chan struct{} // signals runX11HotkeyLoop to exit
	doneCh   chan struct{} // closed when runX11HotkeyLoop has exited
	once     sync.Once
}

// x11IgnoredModifierMasks are the lock modifiers a grab has to be repeated
// under, because the server reports them in a key event's state and a grab
// naming a different state does not match: Lock is CapsLock, and Mod2 is where
// NumLock conventionally sits.
//
// The set is stated once so a grab and the ungrab that undoes it cannot drift:
// an ungrab that misses a mask leaves a hotkey grabbed after Neru forgot it.
func x11IgnoredModifierMasks() [4]C.uint {
	return [4]C.uint{0, C.Mod2Mask, C.LockMask, C.Mod2Mask | C.LockMask}
}

// grab resolves keyString against the connection's keymap and installs the
// grabs for it, as one critical section.
//
// The resolve belongs inside the lock rather than beside it: XKeysymToKeycode
// is an Xlib call on this same connection, so it is one of the calls being
// serialized, not a pure computation that happens to precede them.
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
// none was waiting.
//
// The pending check and the read are one critical section because they are one
// sequence: XPending answering "an event is queued" is only a fact about this
// connection while nothing else is draining it, and an XNextEvent whose event
// another thread has taken blocks until the next one arrives — with displayMu
// held, which is the one way this lock could wedge registration.
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

// closeDisplay tears down the connection. Callers must have stopped the poll
// loop first — the lock orders this against a registration, not against a read
// of a display that has been freed.
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
		// Signal the event loop to stop and wait for it to exit
		// before closing the display, preventing a use-after-free.
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

	// XOpenDisplay and the root-window read are the two Xlib calls here that do
	// not go through displayMu, and the only ones that need not: they run
	// before the state exists to lock, before it is published to x11States and
	// before the poll loop that would share the connection is started.
	display := C.XOpenDisplay(nil)
	if display == nil {
		return nil, derrors.New(
			derrors.CodeHotkeyRegisterFailed,
			"failed to open X11 display for global hotkeys",
		)
	}

	// A held key is otherwise reported as release/press pairs at the server's
	// autorepeat rate, and every one of those releases would end the hold.
	// With detectable autorepeat the hold is further KeyPress events and one
	// KeyRelease when the key comes up, which is the edge the binder repeats
	// on. Per connection, so the in-mode tap asks for the same on its own
	// (platform/linux/x11_eventtap.c).
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
		dispatch: eventtaplinux.NewHotkeyDispatcher(m.logger),
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

		// nextEvent polls with XPending instead of calling the blocking
		// XNextEvent directly. This allows the stop channel to be checked
		// between iterations, preventing a goroutine leak and a
		// use-after-free when unregisterAllX11Hotkeys closes the display —
		// and it is what lets every Xlib call on this connection share one
		// lock without a registration ever waiting on a keystroke.
		event, hasEvent := state.nextEvent()
		if !hasEvent {
			time.Sleep(x11HotkeyPollInterval)

			continue
		}

		keycode := C.neru_xkey_keycode(&event)

		switch C.neru_xevent_type(&event) {
		case C.KeyPress:
			modifiers := C.neru_xkey_state(&event) &^ (C.Mod2Mask | C.LockMask)

			// Hold m.mu while reading state.ids — Register/Unregister write
			// to this map under the same lock, so an unguarded read here is a
			// concurrent map read/write (runtime crash under the race
			// detector). The callback is fetched in the same critical section
			// to avoid a second lock acquisition.
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
// "Backspace" is the same story with a sharper edge: X11 spells it "BackSpace",
// so it resolved through neither path, and the repair that suggests itself —
// folding the name through the vocabulary's aliases — grabs the wrong physical
// key, for the reason x11CanonicalKeyName below sets out. Delete and Insert are
// mapped alongside it rather than left to their matching spellings, so a reader
// sees the three editing keys and their distinct keysyms in one place.
//
// The rest fall through to XStringToKeysym: punctuation, and F1-F24, where
// X11's own name for the key is the name Neru writes. Those are pinned by test
// across the range instead of restating 24 spellings here.
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
	case keyvocab.KeyBackspace:
		return C.XK_BackSpace
	case keyvocab.KeyDelete:
		return C.XK_Delete
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
