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

	"github.com/y3owk1n/neru/internal/derrors"
)

var (
	errUinputKeyboardSend    = errors.New("failed to send uinput key event")
	errUinputKeyboardRelease = errors.New("failed to release uinput key (key may be latched)")
)

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
// ensureUinputKeyboard creates the synthetic keyboard on first use and reuses
// it for the life of the process.
//
// When /dev/uinput is writable, `action feed` posts keystrokes through this
// device instead of the compositor's virtual-keyboard or libei protocols. It
// enters the input stack below libinput, so the compositor routes the keys to
// the focused surface exactly like a physical press, uniformly across X11,
// wlroots and KWin, and sidesteps the fragile RemoteDesktop consent path on KDE.
//
// The device carries Neru's uinput vendor id and the neru- name prefix, which
// is how the evdev keyboard proxy knows not to grab it (neru_evdev_is_neru_device,
// isNeruInjectionDevice); its lifetime EVIOCGRAB would otherwise re-read the
// injected keys and loop them back into the event tap.
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
			_ = releaseUinputKeys(pressed)

			return errUinputKeyboardSend
		}

		pressed = append(pressed, code)
	}

	// A failed key-down means nothing was delivered: unwind the modifiers and
	// report failure.
	if C.neru_uinput_key(uinputKeyboardFd, C.int(keycode), 1) == 0 {
		_ = releaseUinputKeys(pressed)

		return errUinputKeyboardSend
	}

	// The chord is delivered on the key-down; now release the main key and the
	// modifiers. Always attempt every release so one failure can't strand the
	// rest down, and surface any failure — a discarded release would leave a
	// key latched on the reused device and silently corrupt later input.
	releasedOK := C.neru_uinput_key(uinputKeyboardFd, C.int(keycode), 0) != 0
	if !releaseUinputKeys(pressed) {
		releasedOK = false
	}

	if !releasedOK {
		return errUinputKeyboardRelease
	}

	return nil
}

// releaseUinputKeys releases the given keycodes in reverse press order,
// attempting every release even when one fails, and reports whether all
// succeeded. A failed release leaves a key latched on the reused device, so
// callers surface it rather than assuming a clean chord.
func releaseUinputKeys(codes []int) bool {
	allReleased := true

	for _, code := range slices.Backward(codes) {
		if C.neru_uinput_key(uinputKeyboardFd, C.int(code), 0) == 0 {
			allReleased = false
		}
	}

	return allReleased
}
