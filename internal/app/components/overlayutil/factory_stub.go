//go:build !darwin

package overlayutil

import (
	"unsafe"

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
	return &BaseOverlay{
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
