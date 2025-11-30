package scroll

/*
#cgo CFLAGS: -x objective-c
#include "../../../core/infra/bridge/overlay.h"
#include <stdlib.h>

// Callback function that Go can reference.
extern void resizeScrollCompletionCallback(void* context);
*/
import "C"

import (
	"unsafe"

	"github.com/y3owk1n/neru/internal/app/components/overlayutil"
	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"go.uber.org/zap"
)

//export resizeScrollCompletionCallback
func resizeScrollCompletionCallback(context unsafe.Pointer) {
	// Convert context to callback ID
	id := uint64(uintptr(context))

	// Delegate to global callback manager
	overlayutil.CompleteGlobalCallback(id)
}

// Overlay manages the rendering of scroll mode overlays using native platform APIs.
type Overlay struct {
	window          C.OverlayWindow
	config          config.ScrollConfig
	logger          *zap.Logger
	callbackManager *overlayutil.CallbackManager
	borderBuilder   *overlayutil.WindowBorderBuilder
}

// NewOverlay initializes a new scroll overlay instance with its own window.
func NewOverlay(config config.ScrollConfig, logger *zap.Logger) (*Overlay, error) {
	window := C.createOverlayWindow()
	if window == nil {
		return nil, derrors.New(derrors.CodeOverlayFailed, "failed to create overlay window")
	}

	return &Overlay{
		window:          window,
		config:          config,
		logger:          logger,
		callbackManager: overlayutil.NewCallbackManager(logger),
		borderBuilder:   &overlayutil.WindowBorderBuilder{},
	}, nil
}

// NewOverlayWithWindow initializes a scroll overlay instance using a shared window.
func NewOverlayWithWindow(
	config config.ScrollConfig,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) (*Overlay, error) {
	return &Overlay{
		window:          (C.OverlayWindow)(windowPtr),
		config:          config,
		logger:          logger,
		callbackManager: overlayutil.NewCallbackManager(logger),
		borderBuilder:   &overlayutil.WindowBorderBuilder{},
	}, nil
}

// GetWindow returns the underlying C overlay window.
func (o *Overlay) GetWindow() C.OverlayWindow {
	return o.window
}

// GetConfig returns the scroll configuration.
func (o *Overlay) GetConfig() config.ScrollConfig {
	return o.config
}

// GetLogger returns the logger.
func (o *Overlay) GetLogger() *zap.Logger {
	return o.logger
}

// Show displays the scroll overlay window.
func (o *Overlay) Show() {
	C.NeruShowOverlayWindow(o.window)
}

// Hide conceals the scroll overlay window.
func (o *Overlay) Hide() {
	C.NeruHideOverlayWindow(o.window)
}

// Clear removes all scroll highlights from the overlay.
func (o *Overlay) Clear() {
	C.NeruClearOverlay(o.window)
}

// ResizeToActiveScreen adjusts the overlay window size to match the screen containing the mouse cursor.
func (o *Overlay) ResizeToActiveScreen() {
	C.NeruResizeOverlayToActiveScreen(o.window)
}

// ResizeToActiveScreenSync adjusts the overlay window size synchronously with callback notification.
func (o *Overlay) ResizeToActiveScreenSync() {
	o.callbackManager.StartResizeOperation(func(callbackID uint64) {
		// Pass integer ID as opaque pointer context for C callback.
		// Safe: ID is a primitive value that C treats as opaque and Go round-trips via uintptr.
		// Note: go vet complains about unsafe.Pointer misuse, but this is intentional and safe.
		C.NeruResizeOverlayToActiveScreenWithCallback(
			o.window,
			(C.ResizeCompletionCallback)(
				unsafe.Pointer(C.resizeScrollCompletionCallback), //nolint:unconvert
			),
			unsafe.Pointer(uintptr(callbackID)), //nolint:govet
		)
	})
}

// DrawScrollHighlight renders a highlight border around the specified screen area.
func (o *Overlay) DrawScrollHighlight(xCoordinate, yCoordinate, width, height int) {
	// Use scroll config for highlight color and width
	color := o.config.HighlightColor
	borderWidth := o.config.HighlightWidth

	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor)) //nolint:nlreturn

	// Use border builder to create rectangles
	rectangles := o.borderBuilder.BuildBorderRectangles(
		xCoordinate, yCoordinate, width, height, borderWidth,
	)

	// Convert to C rectangles
	lines := make([]C.CGRect, len(rectangles))
	for i, rect := range rectangles {
		lines[i] = C.CGRect{
			origin: C.CGPoint{
				x: C.double(rect.X),
				y: C.double(rect.Y),
			},
			size: C.CGSize{
				width:  C.double(rect.Width),
				height: C.double(rect.Height),
			},
		}
	}

	C.NeruDrawWindowBorder(
		o.window,
		&lines[0],
		C.int(len(lines)),
		cColor,
		C.int(borderWidth),
		C.double(1.0),
	)
}

// UpdateConfig updates the overlay configuration.
func (o *Overlay) UpdateConfig(config config.ScrollConfig) {
	o.config = config
}

// Destroy releases the overlay window resources.
func (o *Overlay) Destroy() {
	if o.window != nil {
		C.NeruDestroyOverlayWindow(o.window)
		o.window = nil
	}
}
