//go:build darwin

package darwin

/*
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

// Malloc allocates size bytes on the C heap. The memory is outside Go's GC, so
// C code can retain it across async dispatch boundaries. The caller must
// release it with Free.
func Malloc(size uintptr) unsafe.Pointer {
	return C.malloc(C.size_t(size))
}

// Free releases memory allocated by Malloc. Safe to call with nil.
func Free(ptr unsafe.Pointer) {
	if ptr != nil {
		C.free(ptr)
	}
}
