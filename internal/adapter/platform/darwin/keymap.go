//go:build darwin

package darwin

/*
#include "keymap.h"
#include <stdlib.h>

extern void keymapLayoutChangeBridge(void);
*/
import "C"

import (
	"strings"
	"unsafe"
)

// KeymapLayoutChangeHandler is called after keyboard layout maps are rebuilt.
type KeymapLayoutChangeHandler func()

var keymapLayoutChangeSlot cgoSlot[KeymapLayoutChangeHandler]

// SetKeymapLayoutChangeHandler registers a Go-level callback invoked after
// keyboard layout maps are rebuilt at runtime (e.g., when the user switches
// between US and Dvorak). Pass nil to unregister.
func SetKeymapLayoutChangeHandler(handler KeymapLayoutChangeHandler) {
	keymapLayoutChangeSlot.Set(handler)
	if handler != nil {
		C.NeruSetKeymapLayoutChangeCallback2(
			C.KeymapLayoutChangeCallback(C.keymapLayoutChangeBridge),
		)
	} else {
		C.NeruSetKeymapLayoutChangeCallback2(nil)
	}
}

//export keymapLayoutChangeBridge
func keymapLayoutChangeBridge() {
	keymapLayoutChangeSlot.withValidAsync(func(handler KeymapLayoutChangeHandler) {
		handler()
	})
}

// KeyCodeToCharacter returns the key string the event tap would produce for a
// raw virtual key code with the given CGEventFlags: a character ("a", "5", "."),
// or a named key for keys that fold to one (numpad Enter -> "Return"). Returns
// "" when the key code does not resolve. It exists so tests can pin the
// keycode-translation contract from Go.
func KeyCodeToCharacter(keyCode uint16, flags uint64) string {
	cStr := C.NeruCopyKeyCodeToCharacter(C.CGKeyCode(keyCode), C.CGEventFlags(flags))
	if cStr == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cStr))

	return C.GoString(cStr)
}

// KeyCodeToName returns the layout-independent name for a raw virtual key
// code ("Return", "Escape", "A"), or "" when the key code has none. It exists
// so tests can pin the keycode-translation contract from Go.
func KeyCodeToName(keyCode uint16) string {
	cStr := C.NeruCopyKeyCodeToName(C.CGKeyCode(keyCode))
	if cStr == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cStr))

	return C.GoString(cStr)
}

// SetReferenceKeyboardLayout configures the key translation reference layout.
// Pass an empty inputSourceID to use automatic fallback resolution.
// Returns false only when a non-empty layout ID was provided but could not be resolved.
func SetReferenceKeyboardLayout(inputSourceID string) bool {
	layoutID := strings.TrimSpace(inputSourceID)
	var cLayoutID *C.char
	if layoutID != "" {
		cLayoutID = C.CString(layoutID)
		defer C.free(unsafe.Pointer(cLayoutID))
	}

	result := C.NeruSetReferenceKeyboardLayout(cLayoutID) != 0

	return result
}
