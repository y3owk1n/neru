//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c

#include "accessibility.h"
*/
import "C"

import "unsafe"

// HasClickAction checks if the accessibility element has a click action.
func HasClickAction(element unsafe.Pointer) bool {
	result := C.hasClickAction(element) != 0 //nolint:nlreturn

	return result
}

// SetApplicationAttribute sets an attribute on an application by its PID.
func SetApplicationAttribute(pid int, attribute string, value bool) bool {
	cAttr := C.CString(attribute)
	defer C.free(unsafe.Pointer(cAttr)) //nolint:nlreturn

	val := 0
	if value {
		val = 1
	}

	result := C.setApplicationAttribute(C.int(pid), cAttr, C.int(val)) != 0

	return result
}
