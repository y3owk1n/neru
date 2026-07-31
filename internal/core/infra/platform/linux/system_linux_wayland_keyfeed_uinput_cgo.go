//go:build linux && cgo

package linux

/*
#include "evdev.h"
*/
import "C"

import (
	"errors"
	"slices"
	"sync"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// uinput keyboard injection. When /dev/uinput is writable, `action feed` posts
// keystrokes through a synthetic keyboard device instead of the compositor's
// virtual-keyboard / libei protocols. The device enters the input stack below
// libinput, so the compositor routes the keys to the focused surface exactly
// like a physical keypress — uniformly across X11, wlroots, and KWin — and
// sidesteps the fragile RemoteDesktop portal consent path on KDE.
//
// The device is created lazily on first use and reused for the process
// lifetime. It is tagged BUS_VIRTUAL so Neru's own evdev capture skips it (see
// isUinputVirtualDevice); otherwise an active EVIOCGRAB would grab our injected
// keys and loop them back into the event tap.

var errUinputKeyboardSend = errors.New("failed to send uinput key event")

var (
	uinputKeyboardOnce sync.Once
	uinputKeyboardFd   C.int
	errUinputKeyboard  error
)

// uinputKeyboardMu serializes chord emission. The events forming one chord
// (modifier-down… key down/up… modifier-up) are separate write() calls to the
// shared device fd; without this lock, two concurrent feeds could interleave
// their writes and deliver a garbled chord.
var uinputKeyboardMu sync.Mutex

// ensureUinputKeyboard creates the synthetic keyboard device on first call and
// caches the result. A creation failure (typically /dev/uinput not writable) is
// remembered so callers fall through to the compositor backends.
func ensureUinputKeyboard() error {
	uinputKeyboardOnce.Do(func() {
		var deviceFd C.int
		if C.neru_uinput_create_keyboard(&deviceFd) == 0 {
			errUinputKeyboard = derrors.New(
				derrors.CodeNotSupported,
				"uinput keyboard unavailable: /dev/uinput is not writable "+
					"(add a udev rule or the input group to grant access)",
			)

			return
		}

		uinputKeyboardFd = deviceFd
	})

	return errUinputKeyboard
}

// uinputKeyboardAvailable reports whether keystrokes can be fed through the
// uinput virtual keyboard, creating the device on first call.
func uinputKeyboardAvailable() bool {
	return ensureUinputKeyboard() == nil
}

// feedKeyUinput injects a key chord through the uinput virtual keyboard: press
// modifiers in order, press and release the main key, then release modifiers in
// reverse order. Each modifier is emitted on our own device; because libinput
// tracks key state per device, releasing our modifier never clears one the user
// is still physically holding.
func feedKeyUinput(modifiers []string, keycode uint32) error {
	err := ensureUinputKeyboard()
	if err != nil {
		return err
	}

	codes, err := uinputModifierCodes(modifiers)
	if err != nil {
		return err
	}

	// Hold the lock across the whole emit sequence so the chord's events reach
	// the device fd contiguously, never interleaved with a concurrent feed.
	uinputKeyboardMu.Lock()
	defer uinputKeyboardMu.Unlock()

	pressed := make([]int, 0, len(codes))

	for _, code := range codes {
		if C.neru_uinput_key(uinputKeyboardFd, C.int(code), 1) == 0 {
			releaseUinputKeys(pressed)

			return errUinputKeyboardSend
		}

		pressed = append(pressed, code)
	}

	// The app acts on the key-down, so only the down gates success; a failed
	// key-up leaves only cleanup pending.
	downOK := C.neru_uinput_key(uinputKeyboardFd, C.int(keycode), 1) != 0
	if downOK {
		_ = C.neru_uinput_key(uinputKeyboardFd, C.int(keycode), 0)
	}

	releaseUinputKeys(pressed)

	if !downOK {
		return errUinputKeyboardSend
	}

	return nil
}

// releaseUinputKeys releases the given keycodes in reverse press order. Errors
// are ignored: a release failure only risks a latched virtual key, and there is
// no recovery beyond retrying the same failing write.
func releaseUinputKeys(codes []int) {
	for _, code := range slices.Backward(codes) {
		_ = C.neru_uinput_key(uinputKeyboardFd, C.int(code), 0)
	}
}
