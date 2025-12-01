package hints

/*
#cgo CFLAGS: -x objective-c
#include "../../../core/infra/bridge/overlay.h"
#include <stdlib.h>

// Callback function that Go can reference.
extern void resizeHintCompletionCallback(void* context);
*/
import "C"

import (
	"fmt"
	"image"
	"sync"
	"time"
	"unsafe"

	"github.com/y3owk1n/neru/internal/app/components/overlayutil"
	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"go.uber.org/zap"
)

//export resizeHintCompletionCallback
func resizeHintCompletionCallback(context unsafe.Pointer) {
	// Read callback ID from the pointer (points to a slice element in callbackIDStore)
	id := *(*uint64)(context)

	overlayutil.CompleteGlobalCallback(id)
}

var (
	hintDataPool    sync.Pool
	cLabelSlicePool sync.Pool
	hintPoolOnce    sync.Once

	// Pre-allocated common errors.
	errCreateOverlayWindow = derrors.New(
		derrors.CodeOverlayFailed,
		"failed to create overlay window",
	)
)

// Overlay manages the rendering of hint overlays using native platform APIs.
type Overlay struct {
	window C.OverlayWindow
	config config.HintsConfig
	logger *zap.Logger

	callbackManager *overlayutil.CallbackManager

	// Cached C strings for style properties to reduce allocations
	cachedStyleMu          sync.RWMutex
	cachedFontFamily       *C.char
	cachedBgColor          *C.char
	cachedTextColor        *C.char
	cachedMatchedTextColor *C.char
	cachedBorderColor      *C.char
	cachedHighlightColor   *C.char

	// State tracking for incremental updates
	hintStateMu   sync.RWMutex
	previousHints []*Hint
	previousInput string
	previousStyle StyleMode
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

// FontSize returns the font size.
func (s StyleMode) FontSize() int {
	return s.fontSize
}

// FontFamily returns the font family.
func (s StyleMode) FontFamily() string {
	return s.fontFamily
}

// BorderRadius returns the border radius.
func (s StyleMode) BorderRadius() int {
	return s.borderRadius
}

// Padding returns the padding.
func (s StyleMode) Padding() int {
	return s.padding
}

// BorderWidth returns the border width.
func (s StyleMode) BorderWidth() int {
	return s.borderWidth
}

// Opacity returns the opacity.
func (s StyleMode) Opacity() float64 {
	return s.opacity
}

// BackgroundColor returns the background color.
func (s StyleMode) BackgroundColor() string {
	return s.backgroundColor
}

// TextColor returns the text color.
func (s StyleMode) TextColor() string {
	return s.textColor
}

// MatchedTextColor returns the matched text color.
func (s StyleMode) MatchedTextColor() string {
	return s.matchedTextColor
}

// BorderColor returns the border color.
func (s StyleMode) BorderColor() string {
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
		window:          window,
		config:          config,
		logger:          logger,
		callbackManager: overlayutil.NewCallbackManager(logger),
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
		window:          (C.OverlayWindow)(windowPtr),
		config:          config,
		logger:          logger,
		callbackManager: overlayutil.NewCallbackManager(logger),
	}, nil
}

// Window returns the underlying C overlay window.
func (o *Overlay) Window() C.OverlayWindow {
	return o.window
}

// Config returns the hints config.
func (o *Overlay) Config() config.HintsConfig { return o.config }

// Logger returns the logger.
func (o *Overlay) Logger() *zap.Logger { return o.logger }

// Show shows the overlay.
func (o *Overlay) Show() {
	C.NeruShowOverlayWindow(o.window)
}

// Hide hides the overlay.
func (o *Overlay) Hide() {
	C.NeruHideOverlayWindow(o.window)
}

// Clear clears all hints from the overlay and resets state.
func (o *Overlay) Clear() {
	C.NeruClearOverlay(o.window)
	// Reset previous state so next draw will be a full redraw
	o.hintStateMu.Lock()
	o.previousHints = nil
	o.previousInput = ""
	o.previousStyle = StyleMode{}
	o.hintStateMu.Unlock()
}

// ResizeToActiveScreen resizes the overlay window with callback notification.
func (o *Overlay) ResizeToActiveScreen() {
	o.callbackManager.StartResizeOperation(func(callbackID uint64) {
		// Pass integer ID as opaque pointer context for C callback.
		// Uses CallbackIDToPointer to convert in a way that go vet accepts.
		contextPtr := overlayutil.CallbackIDToPointer(callbackID)

		C.NeruResizeOverlayToActiveScreenWithCallback(
			o.window,
			(C.ResizeCompletionCallback)(C.resizeHintCompletionCallback),
			contextPtr,
		)
	})
}

// DrawHintsWithStyle draws hints on the overlay with custom style.
func (o *Overlay) DrawHintsWithStyle(hints []*Hint, style StyleMode) error {
	return o.drawHintsInternal(hints, style, true)
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
	// Invalidate style cache when config changes
	o.freeStyleCache()
}

// Destroy destroys the overlay.
func (o *Overlay) Destroy() {
	// Clean up callback manager first to stop background goroutines
	if o.callbackManager != nil {
		o.callbackManager.Cleanup()
	}

	if o.window != nil {
		o.freeStyleCache()
		C.NeruDestroyOverlayWindow(o.window)
		o.window = nil
	}
}

// freeStyleCache frees all cached C strings.
func (o *Overlay) freeStyleCache() {
	o.cachedStyleMu.Lock()
	defer o.cachedStyleMu.Unlock()

	if o.cachedFontFamily != nil {
		C.free(unsafe.Pointer(o.cachedFontFamily))
		o.cachedFontFamily = nil
	}
	if o.cachedBgColor != nil {
		C.free(unsafe.Pointer(o.cachedBgColor))
		o.cachedBgColor = nil
	}
	if o.cachedTextColor != nil {
		C.free(unsafe.Pointer(o.cachedTextColor))
		o.cachedTextColor = nil
	}
	if o.cachedMatchedTextColor != nil {
		C.free(unsafe.Pointer(o.cachedMatchedTextColor))
		o.cachedMatchedTextColor = nil
	}
	if o.cachedBorderColor != nil {
		C.free(unsafe.Pointer(o.cachedBorderColor))
		o.cachedBorderColor = nil
	}
	if o.cachedHighlightColor != nil {
		C.free(unsafe.Pointer(o.cachedHighlightColor))
		o.cachedHighlightColor = nil
	}
}

// updateStyleCacheLocked updates cached C strings for the current style.
// Must be called with cachedStyleMu write lock held.
func (o *Overlay) updateStyleCacheLocked(style StyleMode) {
	// Free old cached strings
	if o.cachedFontFamily != nil {
		C.free(unsafe.Pointer(o.cachedFontFamily))
	}
	if o.cachedBgColor != nil {
		C.free(unsafe.Pointer(o.cachedBgColor))
	}
	if o.cachedTextColor != nil {
		C.free(unsafe.Pointer(o.cachedTextColor))
	}
	if o.cachedMatchedTextColor != nil {
		C.free(unsafe.Pointer(o.cachedMatchedTextColor))
	}
	if o.cachedBorderColor != nil {
		C.free(unsafe.Pointer(o.cachedBorderColor))
	}

	// Create new cached strings
	o.cachedFontFamily = C.CString(style.FontFamily())
	o.cachedBgColor = C.CString(style.BackgroundColor())
	o.cachedTextColor = C.CString(style.TextColor())
	o.cachedMatchedTextColor = C.CString(style.MatchedTextColor())
	o.cachedBorderColor = C.CString(style.BorderColor())
}

// getCachedStyleStrings returns cached C strings for style, updating cache if needed.
func (o *Overlay) getCachedStyleStrings(
	style StyleMode,
) (*C.char, *C.char, *C.char, *C.char, *C.char) {
	o.cachedStyleMu.RLock()
	// Check if cache needs rebuild or is invalid
	if o.cachedFontFamily == nil {
		o.cachedStyleMu.RUnlock()
		o.cachedStyleMu.Lock()
		// Double-check after acquiring write lock
		if o.cachedFontFamily == nil {
			o.updateStyleCacheLocked(style)
		}
		fontFamily := o.cachedFontFamily
		bgColor := o.cachedBgColor
		textColor := o.cachedTextColor
		matchedTextColor := o.cachedMatchedTextColor
		borderColor := o.cachedBorderColor
		o.cachedStyleMu.Unlock()

		return fontFamily, bgColor, textColor, matchedTextColor, borderColor
	}

	fontFamily := o.cachedFontFamily
	bgColor := o.cachedBgColor
	textColor := o.cachedTextColor
	matchedTextColor := o.cachedMatchedTextColor
	borderColor := o.cachedBorderColor
	o.cachedStyleMu.RUnlock()

	return fontFamily, bgColor, textColor, matchedTextColor, borderColor
}

// drawHintsInternal is the internal implementation for drawing hints.
func (o *Overlay) drawHintsInternal(hints []*Hint, style StyleMode, showArrow bool) error {
	if len(hints) == 0 {
		o.Clear()

		return nil
	}

	start := time.Now()

	// Extract current input from hints (use first hint's matched prefix as input)
	currentInput := ""
	if len(hints) > 0 && len(hints[0].MatchedPrefix()) > 0 {
		currentInput = hints[0].MatchedPrefix()
	}

	// Check if we can do incremental updates
	o.hintStateMu.RLock()
	canIncrementalUpdate := len(o.previousHints) > 0
	o.hintStateMu.RUnlock()

	if canIncrementalUpdate {
		// Try incremental update
		if o.drawHintsIncremental(hints, currentInput, style, showArrow) {
			// Update cached state on successful incremental update
			o.hintStateMu.Lock()
			o.previousHints = make([]*Hint, len(hints))
			copy(o.previousHints, hints)
			o.previousInput = currentInput
			o.previousStyle = style
			o.hintStateMu.Unlock()

			o.logger.Debug("Hints incremental update successful")

			return nil
		}
		o.logger.Debug("Hints incremental update failed, falling back to full redraw")
	}
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
		cLabels[i] = C.CString(hint.Label())
		cHints[i] = C.HintData{
			label: cLabels[i],
			position: C.CGPoint{
				x: C.double(hint.Position().X),
				y: C.double(hint.Position().Y),
			},
			size: C.CGSize{
				width:  C.double(hint.Size().X),
				height: C.double(hint.Size().Y),
			},
			matchedPrefixLength: C.int(len(hint.MatchedPrefix())),
		}

		if len(hint.MatchedPrefix()) > 0 {
			matchedCount++
		}
	}

	o.logger.Debug("Hint match statistics",
		zap.Int("total_hints", len(hints)),
		zap.Int("matched_hints", matchedCount))

	// Use cached style strings to avoid repeated allocations
	cFontFamily, cBgColor, cTextColor, cMatchedTextColor, cBorderColor := o.getCachedStyleStrings(
		style,
	)

	arrowFlag := 0
	if showArrow {
		arrowFlag = 1
	}

	finalStyle := C.HintStyle{
		fontSize:         C.int(style.FontSize()),
		fontFamily:       cFontFamily,
		backgroundColor:  cBgColor,
		textColor:        cTextColor,
		matchedTextColor: cMatchedTextColor,
		borderColor:      cBorderColor,
		borderRadius:     C.int(style.BorderRadius()),
		borderWidth:      C.int(style.BorderWidth()),
		padding:          C.int(style.Padding()),
		opacity:          C.double(style.Opacity()),
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
	// Note: We don't free cached style strings - they're reused across draws

	// Update cached state
	o.hintStateMu.Lock()
	o.previousHints = make([]*Hint, len(hints))
	copy(o.previousHints, hints)
	o.previousInput = currentInput
	o.previousStyle = style
	o.hintStateMu.Unlock()

	o.logger.Debug("Hints drawn successfully",
		zap.Duration("duration", time.Since(start)))

	return nil
}

// drawHintsIncremental performs incremental updates by only updating changed hints.
func (o *Overlay) drawHintsIncremental(
	hints []*Hint,
	currentInput string,
	style StyleMode,
	showArrow bool,
) bool {
	o.hintStateMu.RLock()
	previousHints := o.previousHints
	previousInput := o.previousInput
	previousStyle := o.previousStyle
	o.hintStateMu.RUnlock()

	if len(previousHints) == 0 {
		return false // No previous state to compare against
	}

	// Check if only the input changed (common case for typing)
	if o.hintsAreStructurallyEqual(hints, previousHints) && style == previousStyle {
		// Only input changed - we can do incremental match updates
		if currentInput != previousInput {
			o.updateMatchesIncremental(currentInput)

			return true
		}
		// No changes at all - but we need to ensure overlay is actually visible
		// If the overlay was cleared between activations, we need to redraw even if nothing changed.
		if currentInput == "" && previousInput == "" {
			// This might be a fresh activation after clear - force redraw to be safe
			return false
		}
		// Otherwise, assume overlay is still showing and no redraw needed
		return true
	}

	// Handle structural changes (hints added/removed) using incremental C API
	return o.drawHintsIncrementalStructural(
		hints,
		previousHints,
		currentInput,
		style,
		previousInput,
		previousStyle,
		showArrow,
	)
}

// hintsAreStructurallyEqual checks if two hint lists have the same structure (same hints at same positions).
func (o *Overlay) hintsAreStructurallyEqual(hintsA, hintsB []*Hint) bool {
	if len(hintsA) != len(hintsB) {
		return false
	}

	// Build position map for efficient lookup
	hintsBMap := make(map[string]*Hint) // Use position as key (x,y)
	for _, hint := range hintsB {
		key := o.hintPositionKey(hint)
		hintsBMap[key] = hint
	}

	// Check if all hints in hintsA exist in hintsB at the same position with same label
	for _, hintA := range hintsA {
		key := o.hintPositionKey(hintA)
		hintB, exists := hintsBMap[key]
		if !exists {
			return false
		}
		if hintA.Label() != hintB.Label() {
			return false
		}
	}

	return true
}

// hintPositionKey generates a unique key for a hint based on its position.
func (o *Overlay) hintPositionKey(hint *Hint) string {
	pos := hint.Position()

	return fmt.Sprintf("%d,%d", pos.X, pos.Y)
}

// updateMatchesIncremental updates match states incrementally when input changes.
func (o *Overlay) updateMatchesIncremental(newInput string) {
	cPrefix := C.CString(newInput)
	defer C.free(unsafe.Pointer(cPrefix)) //nolint:nlreturn

	C.NeruUpdateHintMatchPrefix(o.window, cPrefix)

	o.logger.Debug("Incremental match update",
		zap.String("new_input", newInput))
}

// drawHintsIncrementalStructural handles structural changes using the incremental C API.
func (o *Overlay) drawHintsIncrementalStructural(
	currentHints []*Hint,
	previousHints []*Hint,
	currentInput string,
	currentStyle StyleMode,
	previousInput string,
	previousStyle StyleMode,
	showArrow bool,
) bool {
	// Build maps for efficient lookup
	previousHintMap := make(map[string]*Hint)
	for _, hint := range previousHints {
		key := o.hintPositionKey(hint)
		previousHintMap[key] = hint
	}

	currentHintMap := make(map[string]*Hint)
	for _, hint := range currentHints {
		key := o.hintPositionKey(hint)
		currentHintMap[key] = hint
	}

	// Find hints to add/update (in current but not in previous, or changed)
	var hintsToAdd []*Hint
	for _, hint := range currentHints {
		key := o.hintPositionKey(hint)
		prevHint, exists := previousHintMap[key]
		if !exists { //nolint:gocritic
			// New hint
			hintsToAdd = append(hintsToAdd, hint)
		} else if hint.Label() != prevHint.Label() {
			// Hint label changed
			hintsToAdd = append(hintsToAdd, hint)
		} else if currentInput != previousInput || currentStyle != previousStyle {
			// Hint exists but match state or style changed
			hintsToAdd = append(hintsToAdd, hint)
		}
	}

	// Find hints to remove (in previous but not in current)
	var positionsToRemove []image.Point
	for _, hint := range previousHints {
		key := o.hintPositionKey(hint)
		if _, exists := currentHintMap[key]; !exists {
			positionsToRemove = append(positionsToRemove, hint.Position())
		}
	}

	// If we need to add all hints (overlay is likely empty), do a full redraw instead
	if len(hintsToAdd) == len(currentHints) && len(positionsToRemove) == len(previousHints) {
		// All hints need to be added and all previous hints removed - this is effectively a full redraw
		// Fall back to full redraw for better performance and correctness
		return false
	}

	// If no changes, nothing to do
	if len(hintsToAdd) == 0 && len(positionsToRemove) == 0 {
		return true
	}

	// Convert hints to C structures
	hintsToAddC := o.convertHintsToC(hintsToAdd, currentInput)
	defer func() {
		// Free C strings for labels
		for _, cHint := range hintsToAddC {
			if cHint.label != nil {
				C.free(unsafe.Pointer(cHint.label))
			}
		}
	}()

	// Convert positions to C structures
	var positionsToRemoveC []C.CGPoint
	if len(positionsToRemove) > 0 {
		positionsToRemoveC = make([]C.CGPoint, len(positionsToRemove))
		for i, pos := range positionsToRemove {
			positionsToRemoveC[i] = C.CGPoint{
				x: C.double(pos.X),
				y: C.double(pos.Y),
			}
		}
	}

	// Get style strings
	cFontFamily, cBgColor, cTextColor, cMatchedTextColor, cBorderColor := o.getCachedStyleStrings(
		currentStyle,
	)

	arrowFlag := 0
	if showArrow {
		arrowFlag = 1
	}

	finalStyle := C.HintStyle{
		fontSize:         C.int(currentStyle.FontSize()),
		fontFamily:       cFontFamily,
		backgroundColor:  cBgColor,
		textColor:        cTextColor,
		matchedTextColor: cMatchedTextColor,
		borderColor:      cBorderColor,
		borderRadius:     C.int(currentStyle.BorderRadius()),
		borderWidth:      C.int(currentStyle.BorderWidth()),
		padding:          C.int(currentStyle.Padding()),
		opacity:          C.double(currentStyle.Opacity()),
		showArrow:        C.int(arrowFlag),
	}

	// Call incremental C API
	var hintsToAddPtr *C.HintData
	var positionsToRemovePtr *C.CGPoint
	if len(hintsToAddC) > 0 {
		hintsToAddPtr = &hintsToAddC[0]
	}
	if len(positionsToRemoveC) > 0 {
		positionsToRemovePtr = &positionsToRemoveC[0]
	}

	C.NeruDrawIncrementHints(
		o.window,
		hintsToAddPtr,
		C.int(len(hintsToAddC)),
		positionsToRemovePtr,
		C.int(len(positionsToRemoveC)),
		finalStyle,
	)

	o.logger.Debug("Incremental structural update",
		zap.Int("hints_added", len(hintsToAdd)),
		zap.Int("hints_removed", len(positionsToRemove)))

	return true
}

// convertHintsToC converts hint objects to C HintData structures.
func (o *Overlay) convertHintsToC(hintsGo []*Hint, currentInput string) []C.HintData {
	if len(hintsGo) == 0 {
		return nil
	}

	cHints := make([]C.HintData, len(hintsGo))
	cLabels := make([]*C.char, len(hintsGo))

	for hintIndex, hint := range hintsGo {
		cLabels[hintIndex] = C.CString(hint.Label())

		matchedPrefixLength := 0
		if currentInput != "" {
			matchedPrefixLength = len(hint.MatchedPrefix())
		}

		cHints[hintIndex] = C.HintData{
			label: cLabels[hintIndex],
			position: C.CGPoint{
				x: C.double(hint.Position().X),
				y: C.double(hint.Position().Y),
			},
			size: C.CGSize{
				width:  C.double(hint.Size().X),
				height: C.double(hint.Size().Y),
			},
			matchedPrefixLength: C.int(matchedPrefixLength),
		}
	}

	return cHints
}
