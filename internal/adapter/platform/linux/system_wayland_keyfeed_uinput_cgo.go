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

// uinputKeyboardMu serializes chord emission. A chord is several write() calls
// to the shared fd, and concurrent feeds must not interleave them.
var uinputKeyboardMu sync.Mutex

// ensureUinputKeyboard creates the synthetic keyboard on first use and reuses
// it for the life of the process. A creation failure is remembered so callers
// fall through to the compositor backends. The device sits below libinput, so
// keys route like a physical press on every backend.
//
// Its vendor id and neru- name prefix stop the evdev keyboard proxy grabbing it
// (neru_evdev_is_neru_device, isNeruInjectionDevice), which would loop injected
// keys back into the event tap.
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

// feedKeyUinput injects a key chord through the uinput virtual keyboard. libinput
// tracks key state per device, so releasing our modifier never clears one the
// user physically holds.
func feedKeyUinput(modifiers []string, keycode uint32) error {
	err := ensureUinputKeyboard()
	if err != nil {
		return err
	}

	codes, err := uinputModifierCodes(modifiers)
	if err != nil {
		return err
	}

	// Held across the whole chord so its writes stay contiguous.
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

	// Nothing was delivered, so unwind the modifiers.
	if C.neru_uinput_key(uinputKeyboardFd, C.int(keycode), 1) == 0 {
		_ = releaseUinputKeys(pressed)

		return errUinputKeyboardSend
	}

	// Attempt every release and surface any failure, since a lost one latches
	// a key on the reused device.
	releasedOK := C.neru_uinput_key(uinputKeyboardFd, C.int(keycode), 0) != 0
	if !releaseUinputKeys(pressed) {
		releasedOK = false
	}

	if !releasedOK {
		return errUinputKeyboardRelease
	}

	return nil
}

// releaseUinputKeys releases keycodes in reverse press order, attempting every
// release, and reports whether all succeeded.
func releaseUinputKeys(codes []int) bool {
	allReleased := true

	for _, code := range slices.Backward(codes) {
		if C.neru_uinput_key(uinputKeyboardFd, C.int(code), 0) == 0 {
			allReleased = false
		}
	}

	return allReleased
}
