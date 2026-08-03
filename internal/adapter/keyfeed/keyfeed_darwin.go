//go:build darwin

package keyfeed

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#include "../platform/darwin/keyfeed.h"
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"

	_ "github.com/y3owk1n/neru/internal/adapter/platform/darwin"
	"github.com/y3owk1n/neru/internal/derrors"
)

// postKey injects an already-normalized key into the focused application via
// CGEventPost. Synthetic events are marked so Neru's own event tap ignores them
// when the daemon is running.
func postKey(normalized string) error {
	cKey := C.CString(normalized)
	defer C.free(unsafe.Pointer(cKey)) //nolint:nlreturn

	ret := C.NeruPostKeyFeed(cKey)
	switch ret {
	case 1:
		return nil
	case 0:
		return derrors.Newf(derrors.CodeInvalidInput, "unsupported key %q", normalized)
	default:
		return derrors.New(
			derrors.CodeAccessibilityFailed,
			"failed to post key event: check accessibility permissions",
		)
	}
}
