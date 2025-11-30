package action

/*
#cgo CFLAGS: -x objective-c
#include "../../../core/infra/bridge/overlay.h"
#include <stdlib.h>

// Callback function that Go can reference.
extern void resizeActionCompletionCallback(void* context);
*/
import "C"

import (
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/app/components/overlayutil"
	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"go.uber.org/zap"
)

//export resizeActionCompletionCallback
func resizeActionCompletionCallback(context unsafe.Pointer) {
	// Convert context to callback ID
	callbackID := uint64(uintptr(context))

	// Delegate to global callback manager
	overlayutil.CompleteGlobalCallback(callbackID)
}

const (
	// DefaultCallbackMapSize is the default size for callback maps.
	DefaultCallbackMapSize = 8

	// DefaultTimerDuration is the default timer duration.
	DefaultTimerDuration = 2 * time.Second

	// DefaultGridLinesCount is the default number of grid lines.
	DefaultGridLinesCount = 4
)

// Overlay manages the rendering of action mode overlays using native platform APIs.
type Overlay struct {
	window          C.OverlayWindow
	config          config.ActionConfig
	logger          *zap.Logger
	callbackManager *overlayutil.CallbackManager
	borderBuilder   *overlayutil.WindowBorderBuilder
}

// NewOverlay initializes a new action overlay instance with its own window.
func NewOverlay(cfg config.ActionConfig, logger *zap.Logger) (*Overlay, error) {
	window := C.createOverlayWindow()
	if window == nil {
		return nil, derrors.New(derrors.CodeOverlayFailed, "failed to create overlay window")
	}

	return &Overlay{
		window:          window,
		config:          cfg,
		logger:          logger,
		callbackManager: overlayutil.NewCallbackManager(logger),
		borderBuilder:   &overlayutil.WindowBorderBuilder{},
	}, nil
}

// NewOverlayWithWindow initializes an action overlay instance using a shared window.
func NewOverlayWithWindow(
	config config.ActionConfig,
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

// Window returns the underlying C overlay window.
func (o *Overlay) Window() C.OverlayWindow { return o.window }

// Config returns the action configuration.
func (o *Overlay) Config() config.ActionConfig { return o.config }

// Logger returns the logger.
func (o *Overlay) Logger() *zap.Logger { return o.logger }

// Show displays the action overlay window.
func (o *Overlay) Show() {
	C.NeruShowOverlayWindow(o.window)
}

// Hide conceals the action overlay window.
func (o *Overlay) Hide() {
	C.NeruHideOverlayWindow(o.window)
}

// Clear removes all action highlights from the overlay.
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
		// Pass ID as context (safe - no Go pointers)
		// Convert callbackID to unsafe.Pointer for C interop
		// This is safe because we're passing a primitive value that C treats as opaque
		contextPtr := *(*unsafe.Pointer)(unsafe.Pointer(&callbackID))

		C.NeruResizeOverlayToActiveScreenWithCallback(
			o.window,
			(C.ResizeCompletionCallback)(C.resizeActionCompletionCallback),
			contextPtr,
		)
	})
}

// DrawActionHighlight renders a highlight border around the specified screen area.
func (o *Overlay) DrawActionHighlight(xCoordinate, yCoordinate, width, height int) {
	// Use action config for highlight color and width
	color := o.config.HighlightColor
	highlightWidth := o.config.HighlightWidth

	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor)) //nolint:nlreturn

	// Use border builder to create rectangles
	rectangles := o.borderBuilder.BuildBorderRectangles(
		xCoordinate, yCoordinate, width, height, highlightWidth,
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
		C.int(highlightWidth),
		C.double(1.0),
	)
}

// UpdateConfig updates the overlay configuration.
func (o *Overlay) UpdateConfig(cfg config.ActionConfig) {
	o.config = cfg
}

// Destroy releases the overlay window resources.
func (o *Overlay) Destroy() {
	if o.window != nil {
		C.NeruDestroyOverlayWindow(o.window)
		o.window = nil
	}
}
