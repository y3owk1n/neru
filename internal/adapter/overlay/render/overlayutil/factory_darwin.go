//go:build darwin

package overlayutil

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#include "../../../platform/darwin/overlay.h"
*/
import "C"

import (
	"unsafe"

	"go.uber.org/zap"

	// The overlay.h included above only declares its symbols; importing the
	// bridge is what links the Objective-C that defines them (ADR 0009). It
	// belongs here rather than in util.go, which carries no build tag and so
	// may not name a platform package at all.
	_ "github.com/y3owk1n/neru/internal/adapter/platform/darwin"
	"github.com/y3owk1n/neru/internal/derrors"
)

// BaseOverlay holds the common components for an overlay.
type BaseOverlay struct {
	Window          unsafe.Pointer
	CallbackManager *CallbackManager
	StyleCache      *StyleCache
}

// NewBaseOverlay creates a new overlay window and initializes common components.
func NewBaseOverlay(logger *zap.Logger) (*BaseOverlay, error) {
	window := C.NeruCreateOverlayWindow()
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
