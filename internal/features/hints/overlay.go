package hints

/*
#cgo CFLAGS: -x objective-c
#include "../../infra/bridge/overlay.h"
#include <stdlib.h>

// Callback function that Go can reference.
extern void resizeHintCompletionCallback(void* context);
*/
import "C"

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/errors"
	"go.uber.org/zap"
)

var (
	hintCallbackID   uint64
	hintCallbackMap  = make(map[uint64]chan struct{}, 8) // Pre-size for typical usage
	hintCallbackLock sync.Mutex
	hintDataPool     sync.Pool
	cLabelSlicePool  sync.Pool
	hintPoolOnce     sync.Once

	// Pre-allocated common errors.
	errCreateOverlayWindow = derrors.New(
		derrors.CodeOverlayFailed,
		"failed to create overlay window",
	)
)

//export resizeHintCompletionCallback
func resizeHintCompletionCallback(context unsafe.Pointer) {
	// Convert context to callback ID
	id := uint64(uintptr(context))

	hintCallbackLock.Lock()
	if done, ok := hintCallbackMap[id]; ok {
		close(done)
		delete(hintCallbackMap, id)
	}
	hintCallbackLock.Unlock()
}

// Overlay manages the rendering of hint overlays using native platform APIs.
type Overlay struct {
	window C.OverlayWindow
	config config.HintsConfig
	logger *zap.Logger
}

// StyleMode represents the visual styling configuration for hint overlays.
type StyleMode struct {
	fontSize         int
	fontFamily       string
	borderRadius     int
	padding          int
	borderWidth      int
	opacity          float64
	backgroundColor  string
	textColor        string
	matchedTextColor string
	borderColor      string
}

// GetFontSize returns the font size.
func (s StyleMode) GetFontSize() int {
	return s.fontSize
}

// GetFontFamily returns the font family.
func (s StyleMode) GetFontFamily() string {
	return s.fontFamily
}

// GetBorderRadius returns the border radius.
func (s StyleMode) GetBorderRadius() int {
	return s.borderRadius
}

// GetPadding returns the padding.
func (s StyleMode) GetPadding() int {
	return s.padding
}

// GetBorderWidth returns the border width.
func (s StyleMode) GetBorderWidth() int {
	return s.borderWidth
}

// GetOpacity returns the opacity.
func (s StyleMode) GetOpacity() float64 {
	return s.opacity
}

// GetBackgroundColor returns the background color.
func (s StyleMode) GetBackgroundColor() string {
	return s.backgroundColor
}

// GetTextColor returns the text color.
func (s StyleMode) GetTextColor() string {
	return s.textColor
}

// GetMatchedTextColor returns the matched text color.
func (s StyleMode) GetMatchedTextColor() string {
	return s.matchedTextColor
}

// GetBorderColor returns the border color.
func (s StyleMode) GetBorderColor() string {
	return s.borderColor
}

// initPools initializes the object pools once.
func initPools() {
	hintPoolOnce.Do(func() {
		hintDataPool = sync.Pool{New: func() any {
			s := make([]C.HintData, 0)

			return &s
		}}
		cLabelSlicePool = sync.Pool{New: func() any {
			s := make([]*C.char, 0)

			return &s
		}}
	})
}

// NewOverlay creates a new hint overlay instance with its own window.
func NewOverlay(config config.HintsConfig, logger *zap.Logger) (*Overlay, error) {
	window := C.createOverlayWindow()
	if window == nil {
		return nil, errCreateOverlayWindow
	}
	initPools()

	return &Overlay{
		window: window,
		config: config,
		logger: logger,
	}, nil
}

// NewOverlayWithWindow creates a hint overlay instance using a shared window.
func NewOverlayWithWindow(
	config config.HintsConfig,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) (*Overlay, error) {
	initPools()

	return &Overlay{
		window: (C.OverlayWindow)(windowPtr),
		config: config,
		logger: logger,
	}, nil
}

// GetWindow returns the underlying C overlay window.
func (o *Overlay) GetWindow() C.OverlayWindow {
	return o.window
}

// GetConfig returns the hints config.
func (o *Overlay) GetConfig() config.HintsConfig { return o.config }

// GetLogger returns the logger.
func (o *Overlay) GetLogger() *zap.Logger { return o.logger }

// Show shows the overlay.
func (o *Overlay) Show() {
	C.NeruShowOverlayWindow(o.window)
}

// Hide hides the overlay.
func (o *Overlay) Hide() {
	C.NeruHideOverlayWindow(o.window)
}

// Clear clears all hints from the overlay.
func (o *Overlay) Clear() {
	C.NeruClearOverlay(o.window)
}

// ResizeToActiveScreen resizes the overlay window to the screen containing the mouse cursor.
func (o *Overlay) ResizeToActiveScreen() {
	C.NeruResizeOverlayToActiveScreen(o.window)
}

// ResizeToActiveScreenSync resizes the overlay window synchronously with callback notification.
func (o *Overlay) ResizeToActiveScreenSync() {
	done := make(chan struct{})

	// Generate unique ID for this callback
	callbackID := atomic.AddUint64(&hintCallbackID, 1)

	// Store channel in map
	hintCallbackLock.Lock()
	hintCallbackMap[callbackID] = done
	hintCallbackLock.Unlock()

	if o.logger != nil {
		o.logger.Debug("Hint overlay resize started", zap.Uint64("callback_id", callbackID))
	}

	// Pass ID as context (safe - no Go pointers)
	// Note: uintptr conversion must happen in same expression to satisfy go vet
	C.NeruResizeOverlayToActiveScreenWithCallback(
		o.window,
		(C.ResizeCompletionCallback)(
			unsafe.Pointer(C.resizeHintCompletionCallback), //nolint:unconvert
		),
		*(*unsafe.Pointer)(unsafe.Pointer(&callbackID)),
	)

	// Don't wait for callback - continue immediately for better UX
	// The resize operation is typically fast and visually complete before callback
	// Start a goroutine to handle cleanup when callback eventually arrives
	go func() {
		if o.logger != nil {
			o.logger.Debug(
				"Hint overlay resize background cleanup started",
				zap.Uint64("callback_id", callbackID),
			)
		}

		// Use timer instead of time.After to prevent memory leaks
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()

		select {
		case <-done:
			timer.Stop() // Stop timer immediately on success
			// Callback received, normal cleanup already handled in callback
			if o.logger != nil {
				o.logger.Debug(
					"Hint overlay resize callback received",
					zap.Uint64("callback_id", callbackID),
				)
			}
		case <-timer.C:
			// Long timeout for cleanup only - callback likely failed
			hintCallbackLock.Lock()
			delete(hintCallbackMap, callbackID)
			hintCallbackLock.Unlock()

			if o.logger != nil {
				o.logger.Debug("Hint overlay resize cleanup timeout - removed callback from map",
					zap.Uint64("callback_id", callbackID))
			}
		}
	}()
}

// DrawHintsWithStyle draws hints on the overlay with custom style.
func (o *Overlay) DrawHintsWithStyle(hints []*Hint, style StyleMode) error {
	return o.drawHintsInternal(hints, style, true)
}

// DrawTargetDot draws a small circular dot at the target position.
func (o *Overlay) DrawTargetDot(
	pointX, pointY int,
	radius float64,
	color, borderColor string,
	borderWidth float64,
) error {
	center := C.CGPoint{
		x: C.double(pointX),
		y: C.double(pointY),
	}

	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor)) //nolint:nlreturn

	var cBorderColor *C.char
	if borderColor != "" {
		cBorderColor = C.CString(borderColor)
		defer C.free(unsafe.Pointer(cBorderColor)) //nolint:nlreturn
	}

	C.NeruDrawTargetDot(
		o.window,
		center,
		C.double(radius),
		cColor,
		cBorderColor,
		C.double(borderWidth),
	)

	return nil
}

// DrawScrollHighlight draws a highlight around a scroll area.
func (o *Overlay) DrawScrollHighlight(
	xCoordinate, yCoordinate, width, height int,
	color string,
	borderWidth int,
) {
	renderBounds := C.CGRect{
		origin: C.CGPoint{
			x: C.double(xCoordinate),
			y: C.double(yCoordinate),
		},
		size: C.CGSize{
			width:  C.double(width),
			height: C.double(height),
		},
	}

	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor)) //nolint:nlreturn

	C.NeruDrawScrollHighlight(o.window, renderBounds, cColor, C.int(borderWidth))
}

// BuildStyle returns StyleMode based on action name using the provided config.
func BuildStyle(cfg config.HintsConfig) StyleMode {
	style := StyleMode{
		fontSize:         cfg.FontSize,
		fontFamily:       cfg.FontFamily,
		borderRadius:     cfg.BorderRadius,
		padding:          cfg.Padding,
		borderWidth:      cfg.BorderWidth,
		opacity:          cfg.Opacity,
		backgroundColor:  cfg.BackgroundColor,
		textColor:        cfg.TextColor,
		matchedTextColor: cfg.MatchedTextColor,
		borderColor:      cfg.BorderColor,
	}

	return style
}

// UpdateConfig updates the overlay configuration.
func (o *Overlay) UpdateConfig(config config.HintsConfig) {
	o.config = config
}

// Destroy destroys the overlay.
func (o *Overlay) Destroy() {
	if o.window != nil {
		C.NeruDestroyOverlayWindow(o.window)
		o.window = nil
	}
}

// drawHintsInternal is the internal implementation for drawing hints.
func (o *Overlay) drawHintsInternal(hints []*Hint, style StyleMode, showArrow bool) error {
	if len(hints) == 0 {
		o.Clear()

		return nil
	}

	start := time.Now()
	tmpHints := hintDataPool.Get()
	cHintsPtr, _ := tmpHints.(*[]C.HintData)
	if cap(*cHintsPtr) < len(hints) {
		s := make([]C.HintData, len(hints))
		cHintsPtr = &s
	} else {
		*cHintsPtr = (*cHintsPtr)[:len(hints)]
	}
	cHints := *cHintsPtr
	tmpCLables := cLabelSlicePool.Get()
	cLabelsPtr, _ := tmpCLables.(*[]*C.char)
	if cap(*cLabelsPtr) < len(hints) {
		s := make([]*C.char, len(hints))
		cLabelsPtr = &s
	} else {
		*cLabelsPtr = (*cLabelsPtr)[:len(hints)]
	}
	cLabels := *cLabelsPtr

	matchedCount := 0
	for i, hint := range hints {
		cLabels[i] = C.CString(hint.GetLabel())
		cHints[i] = C.HintData{
			label: cLabels[i],
			position: C.CGPoint{
				x: C.double(hint.GetPosition().X),
				y: C.double(hint.GetPosition().Y),
			},
			size: C.CGSize{
				width:  C.double(hint.GetSize().X),
				height: C.double(hint.GetSize().Y),
			},
			matchedPrefixLength: C.int(len(hint.GetMatchedPrefix())),
		}

		if len(hint.GetMatchedPrefix()) > 0 {
			matchedCount++
		}
	}

	o.logger.Debug("Hint match statistics",
		zap.Int("total_hints", len(hints)),
		zap.Int("matched_hints", matchedCount))

	// Create style
	cFontFamily := C.CString(style.GetFontFamily())
	cBgColor := C.CString(style.GetBackgroundColor())
	cTextColor := C.CString(style.GetTextColor())
	cMatchedTextColor := C.CString(style.GetMatchedTextColor())
	cBorderColor := C.CString(style.GetBorderColor())

	arrowFlag := 0
	if showArrow {
		arrowFlag = 1
	}

	finalStyle := C.HintStyle{
		fontSize:         C.int(style.GetFontSize()),
		fontFamily:       cFontFamily,
		backgroundColor:  cBgColor,
		textColor:        cTextColor,
		matchedTextColor: cMatchedTextColor,
		borderColor:      cBorderColor,
		borderRadius:     C.int(style.GetBorderRadius()),
		borderWidth:      C.int(style.GetBorderWidth()),
		padding:          C.int(style.GetPadding()),
		opacity:          C.double(style.GetOpacity()),
		showArrow:        C.int(arrowFlag),
	}

	// Draw hints
	C.NeruDrawHints(o.window, &cHints[0], C.int(len(cHints)), finalStyle)

	// Free all C strings
	for _, cLabel := range cLabels {
		C.free(unsafe.Pointer(cLabel))
	}
	*cHintsPtr = (*cHintsPtr)[:0]
	*cLabelsPtr = (*cLabelsPtr)[:0]
	hintDataPool.Put(cHintsPtr)
	cLabelSlicePool.Put(cLabelsPtr)
	C.free(unsafe.Pointer(cFontFamily))
	C.free(unsafe.Pointer(cBgColor))
	C.free(unsafe.Pointer(cTextColor))
	C.free(unsafe.Pointer(cMatchedTextColor))
	C.free(unsafe.Pointer(cBorderColor))

	o.logger.Debug("Hints drawn successfully",
		zap.Duration("duration", time.Since(start)))

	return nil
}
