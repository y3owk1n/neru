//go:build darwin

package darwin

/*
#include "callback_context.h"
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
