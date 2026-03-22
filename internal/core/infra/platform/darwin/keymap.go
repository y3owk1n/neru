//go:build darwin

package darwin

/*
#include "keymap.h"
#include <stdlib.h>
*/
import "C"

import (
	"strings"
	"unsafe"
)

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

	result := C.setReferenceKeyboardLayout(cLayoutID) != 0

	return result
}
