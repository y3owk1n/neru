package bridge

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ApplicationServices -framework Cocoa -framework Carbon -framework CoreGraphics
#include <stdlib.h>
*/
import "C"

import "unsafe"

// FreeCString frees a C string allocated by C.CString.
// Callers should nil their own pointer after freeing.
func FreeCString(ptr unsafe.Pointer) {
	if ptr != nil {
		C.free(ptr)
	}
}
