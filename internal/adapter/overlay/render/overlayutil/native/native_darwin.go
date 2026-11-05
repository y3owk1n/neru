//go:build darwin

package native

import (
	"unsafe"

	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
)

// Malloc allocates size bytes on the C heap, where the Go GC does not move or
// free them, so C code can retain them across async dispatch boundaries. The caller must call
// Free exactly once when done.
func Malloc(size uintptr) unsafe.Pointer {
	return darwin.Malloc(size)
}

// Free releases memory allocated by Malloc. Safe to call with nil.
func Free(ptr unsafe.Pointer) {
	darwin.Free(ptr)
}

// FreeCString releases a C string previously allocated by C.CString.
// Safe to call with nil.
func FreeCString(ptr unsafe.Pointer) {
	darwin.FreeCString(ptr)
}
