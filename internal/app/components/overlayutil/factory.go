package overlayutil

/*
#cgo CFLAGS: -x objective-c
#include "../../../core/infra/bridge/overlay.h"
*/
import "C"

import (
	"unsafe"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"go.uber.org/zap"
)

// BaseOverlay holds the common components for an overlay.
type BaseOverlay struct {
	Window          unsafe.Pointer
	CallbackManager *CallbackManager
	StyleCache      *StyleCache
}

// NewBaseOverlay creates a new overlay window and initializes common components.
func NewBaseOverlay(logger *zap.Logger) (*BaseOverlay, error) {
	window := C.createOverlayWindow()
	if window == nil {
		return nil, derrors.New(derrors.CodeOverlayFailed, "failed to create overlay window")
	}

	return &BaseOverlay{
		Window:          unsafe.Pointer(window),
		CallbackManager: NewCallbackManager(logger),
		StyleCache:      NewStyleCache(),
	}, nil
}

// NewBaseOverlayWithWindow initializes common components with an existing window.
func NewBaseOverlayWithWindow(logger *zap.Logger, window unsafe.Pointer) *BaseOverlay {
	return &BaseOverlay{
		Window:          window,
		CallbackManager: NewCallbackManager(logger),
		StyleCache:      NewStyleCache(),
	}
}
