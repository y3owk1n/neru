//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c

#include "keymap.h"
#include <stdlib.h>
*/
import "C"

import "unsafe"

// SetReferenceKeyboardLayout sets the reference keyboard layout for key code parsing.
func SetReferenceKeyboardLayout(layoutID string) bool {
	cLayoutID := C.CString(layoutID)
	defer C.free(unsafe.Pointer(cLayoutID)) //nolint:nlreturn

	result := C.setReferenceKeyboardLayout(cLayoutID) != 0

	return result
}
