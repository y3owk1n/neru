//go:build darwin

package native

import (
	"unsafe"

	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
)

// MallocCallbackContext allocates a CallbackContext struct on the C heap.
// The returned pointer is safe for C code to retain across async dispatch
// boundaries because it lives outside the Go GC's reach.
// Caller must call FreeCallbackContext exactly once when done.
func MallocCallbackContext(callbackID, generation uint64) unsafe.Pointer {
	return darwin.MallocCallbackContext(callbackID, generation)
}

// FreeCallbackContext releases a C-heap CallbackContext previously allocated
// by MallocCallbackContext. Safe to call with nil.
func FreeCallbackContext(ptr unsafe.Pointer) {
	darwin.FreeCallbackContext(ptr)
}

// FreeCString releases a C string previously allocated by C.CString.
// Safe to call with nil.
func FreeCString(ptr unsafe.Pointer) {
	darwin.FreeCString(ptr)
}
