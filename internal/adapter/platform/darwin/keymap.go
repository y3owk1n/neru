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

// SetReferenceKeyboardLayout configures the key translation reference layout.
// Pass an empty inputSourceID to use automatic fallback resolution.
// Returns false only when a non-empty layout ID was provided but could not be resolved.
func SetReferenceKeyboardLayout(inputSourceID string) bool {
	layoutID := strings.TrimSpace(inputSourceID)
	var cLayoutID *C.char
	if layoutID != "" {
		cLayoutID = C.CString(layoutID)
		defer C.free(unsafe.Pointer(cLayoutID)) //nolint:nlreturn
	}

	result := C.NeruSetReferenceKeyboardLayout(cLayoutID) != 0

	return result
}
