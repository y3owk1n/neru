package bridge

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ApplicationServices -framework Cocoa -framework Carbon -framework CoreGraphics
#include <stdlib.h>
#include <stdint.h>
#include <string.h>
// callbackContext is a C struct matching overlayutil.CallbackContext.
// Allocated via C.malloc so the pointer is outside Go's GC and safe for
// C code to hold across async dispatch boundaries.
typedef struct {
	uint64_t callbackID;
	uint64_t generation;
} callbackContext;
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

// MallocCallbackContext allocates a callbackContext on the C heap and returns
// an unsafe.Pointer to it. The memory is outside Go's GC, so it is safe for
// C code to retain across async dispatch boundaries.
// The caller must eventually call FreeCallbackContext to avoid leaks.
func MallocCallbackContext(callbackID, generation uint64) unsafe.Pointer {
	ctx := (*C.callbackContext)(C.malloc(C.size_t(unsafe.Sizeof(C.callbackContext{}))))
	ctx.callbackID = C.uint64_t(callbackID)
	ctx.generation = C.uint64_t(generation)

	return unsafe.Pointer(ctx)
}

// FreeCallbackContext frees a callbackContext previously allocated by MallocCallbackContext.
// Safe to call with nil.
func FreeCallbackContext(ptr unsafe.Pointer) {
	if ptr != nil {
		C.free(ptr)
	}
}
