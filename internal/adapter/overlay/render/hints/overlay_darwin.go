//go:build darwin

package hints

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#include "../../../platform/darwin/overlay.h"
#include <stdlib.h>

// Callback function that Go can reference.
extern void resizeHintCompletionCallback(void* context);
*/
import "C"

import (
	"image"
	"sync"
	"time"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/overlayutil"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

//export resizeHintCompletionCallback
func resizeHintCompletionCallback(context unsafe.Pointer) {
	// Read callback context from the C-heap-allocated CallbackContext
	ctx := *(*overlayutil.CallbackContext)(context)

	// Free the C-allocated context now that we've copied the values
	overlayutil.FreeCallbackContext(context)

	overlayutil.CompleteGlobalCallback(ctx.CallbackID, ctx.Generation)
}

var (
	hintDataPool sync.Pool
	hintPoolOnce sync.Once
)

func boolToInt(v bool) int {
	if v {
		return 1
	}

	return 0
}

// hintPlacementValue translates a configured placement into the constant the
// Objective-C overlay switches on. The strings are config's declaration of the
// vocabulary; the constants come from the header that
// internal/architecture/hint_placement_vocabulary_test.go pins the Objective-C
// enum to.
//
// Every value config.HintPlacements() declares has a branch, and anything else
// is refused rather than translated. The switch used to answer an unrecognized
// placement with HINT_PLACEMENT_BOTTOM — right for `bottom` by coincidence, and
// silent for a placement added to the vocabulary without a branch here: it
// would validate, reach the overlay, and draw every label at the bottom with
// nothing failing anywhere. The refusal answers no constant at all, because
// there is no honest placement to hand the drawing code; its caller does not
// draw once it is set.
//
// The empty string is not one of those: it is a style that reached the overlay
// before a configuration settled it, and it draws at the documented default —
// the same answer the Linux overlay gives it (`resolveHintBadgeOffset`), so a
// draw that beats the first Apply puts hints in the same place on both.
func hintPlacementValue(placement string) (int, error) {
	if placement == "" {
		placement = config.HintPlacementDefault
	}

	switch placement {
	case config.HintPlacementTop:
		return int(C.HINT_PLACEMENT_TOP), nil
	case config.HintPlacementCenter:
		return int(C.HINT_PLACEMENT_CENTER), nil
	case config.HintPlacementBottom:
		return int(C.HINT_PLACEMENT_BOTTOM), nil
	default:
		return 0, derrors.New(
			derrors.CodeNotSupported,
			"hint placement is not drawn by the macos overlay",
		)
	}
}

// Overlay manages the rendering of hint overlays using native platform APIs.
type Overlay struct {
	window C.OverlayWindow
	config config.HintsConfig
	logger *zap.Logger

	// configMu protects config from concurrent read/write.
	configMu sync.RWMutex

	callbackManager *overlayutil.CallbackManager

	// Cached C strings for style properties to reduce allocations
	styleCache *overlayutil.StyleCache

	// Label C strings are retained in cachedLabels until Clear() or Destroy().
	// The C overlay holds label pointers between draws for incremental updates,
	// so eviction while active would produce empty labels.
	labelCacheMu sync.Mutex
	cachedLabels map[string]*C.char

	// drawMu serializes draw operations against cache invalidation.
	// Draw paths hold RLock; freeAllCaches holds Lock.
	drawMu sync.RWMutex

	// State tracking for incremental updates
	// NOTE: Assumes Hint instances are immutable between draws to avoid aliasing issues
	hintStateMu   sync.RWMutex
	previousHints []*Hint
	previousInput string
	previousStyle StyleMode
}

func initPools() {
	hintPoolOnce.Do(func() {
		hintDataPool = sync.Pool{New: func() any {
			s := make([]C.HintData, 0)

			return &s
		}}
	})
}

// NewOverlay creates a new hint overlay instance with its own window.
func NewOverlay(config config.HintsConfig, logger *zap.Logger) (*Overlay, error) {
	base, err := overlayutil.NewBaseOverlay(logger)
	if err != nil {
		return nil, err
	}
	base.CallbackManager.SetComponent("hints")
	initPools()

	return &Overlay{
		window:          (C.OverlayWindow)(base.Window),
		config:          config,
		logger:          logger,
		callbackManager: base.CallbackManager,
		styleCache:      base.StyleCache,
		cachedLabels:    make(map[string]*C.char),
	}, nil
}

// NewOverlayWithWindow creates a hint overlay instance using a shared window.
func NewOverlayWithWindow(
	config config.HintsConfig,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) (*Overlay, error) {
	initPools()
	base := overlayutil.NewBaseOverlayWithWindow(logger, windowPtr)
	base.CallbackManager.SetComponent("hints")

	return &Overlay{
		window:          (C.OverlayWindow)(base.Window),
		config:          config,
		logger:          logger,
		callbackManager: base.CallbackManager,
		styleCache:      base.StyleCache,
		cachedLabels:    make(map[string]*C.char),
	}, nil
}

// Window returns the underlying C overlay window.
func (o *Overlay) Window() C.OverlayWindow {
	return o.window
}

// Config returns the hints config.
func (o *Overlay) Config() config.HintsConfig {
	o.configMu.RLock()
	defer o.configMu.RUnlock()

	return o.config
}

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
// Falls back to a non-callback resize if the callback ID pool is exhausted.
func (o *Overlay) ResizeToActiveScreen() {
	started := o.callbackManager.StartResizeOperation(func(callbackID uint64, generation uint64) {
		// Pass callback ID and generation as opaque pointer context for C callback.
		// Uses CallbackIDToPointer to convert in a way that go vet accepts.
		contextPtr := overlayutil.CallbackIDToPointer(callbackID, generation)

		C.NeruResizeOverlayToActiveScreenWithCallback(
			o.window,
			(C.ResizeCompletionCallback)(C.resizeHintCompletionCallback),
			contextPtr,
		)
	})
	if !started {
		// Pool exhausted — fall back to non-callback resize so the overlay
		// is still moved to the correct screen.
		C.NeruResizeOverlayToActiveScreen(o.window)
	}
}

// DrawHintsWithStyle draws hints on the overlay with custom style.
//
// A placement this overlay cannot draw is reported rather than approximated:
// the labels would otherwise appear in a placement the user did not configure,
// with nothing anywhere saying so. The caller degrades quietly on
// CodeNotSupported, so the warning that explains the missing labels is logged
// here.
func (o *Overlay) DrawHintsWithStyle(hints []*Hint, style StyleMode) error {
	return o.drawHintsInternal(hints, style, true)
}

// DrawSearchInput draws the active hints search input.
func (o *Overlay) DrawSearchInput(
	query string,
	resultCount int,
	frame SearchInputFrame,
	style SearchInputStyle,
) error {
	cQuery := C.CString(query)

	defer C.free(unsafe.Pointer(cQuery))

	cFontFamily := C.CString(style.FontFamily())
	cBackgroundColor := C.CString(style.BackgroundColor())
	cTextColor := C.CString(style.TextColor())
	cBorderColor := C.CString(style.BorderColor())

	defer C.free(unsafe.Pointer(cFontFamily))

	defer C.free(unsafe.Pointer(cBackgroundColor))

	defer C.free(unsafe.Pointer(cTextColor))

	defer C.free(unsafe.Pointer(cBorderColor))

	input := C.SearchInputData{
		query:       cQuery,
		resultCount: C.int(resultCount),
		position: C.CGPoint{
			x: C.double(frame.Position().X),
			y: C.double(frame.Position().Y),
		},
		width: C.double(frame.Width()),
	}
	cStyle := C.SearchInputStyle{
		fontSize:        C.int(style.FontSize()),
		fontFamily:      cFontFamily,
		backgroundColor: cBackgroundColor,
		textColor:       cTextColor,
		borderColor:     cBorderColor,
		borderRadius:    C.int(style.BorderRadius()),
		borderWidth:     C.int(style.BorderWidth()),
		paddingX:        C.int(style.PaddingX()),
		paddingY:        C.int(style.PaddingY()),
	}

	C.NeruDrawHintSearchInput(o.window, input, cStyle)

	return nil
}

// HideSearchInput hides the hints search input.
func (o *Overlay) HideSearchInput() {
	C.NeruHideHintSearchInput(o.window)
}

// SetConfig sets the overlay configuration.
func (o *Overlay) SetConfig(config config.HintsConfig) {
	o.configMu.Lock()
	o.config = config
	o.configMu.Unlock()

	// Invalidate caches when config changes
	o.freeAllCaches()
}

// Cleanup frees Go-side resources (callbackManager, styleCache, labelCache)
// without destroying the native window. Use this for overlays that share a
// window managed by the overlay Manager.
func (o *Overlay) Cleanup() {
	if o.callbackManager != nil {
		o.callbackManager.Cleanup()
	}
	o.freeAllCaches()
}

// Destroy destroys the overlay.
func (o *Overlay) Destroy() {
	o.Cleanup()

	if o.window != nil {
		C.NeruDestroyOverlayWindow(o.window)
		o.window = nil
	}
}

// freeAllCaches frees both the style cache and the label cache under drawMu
// so that no in-flight draw can reference freed C pointers.
func (o *Overlay) freeAllCaches() {
	o.drawMu.Lock()
	defer o.drawMu.Unlock()

	o.styleCache.Free()
	o.freeLabelCacheLocked()
}

// freeLabelCacheLocked frees all cached label C strings.
// Caller must hold drawMu.Lock.
func (o *Overlay) freeLabelCacheLocked() {
	o.labelCacheMu.Lock()
	defer o.labelCacheMu.Unlock()
	for _, cStr := range o.cachedLabels {
		if cStr != nil {
			C.free(unsafe.Pointer(cStr))
		}
	}
	o.cachedLabels = make(map[string]*C.char)
}

// getOrCacheLabel returns a cached C string for the label, creating it if needed.
// Labels are retained until Clear() or Destroy() to avoid dangling pointers in the
// C overlay, which stores label pointers between draws for incremental updates.
func (o *Overlay) getOrCacheLabel(label string) *C.char {
	o.labelCacheMu.Lock()
	defer o.labelCacheMu.Unlock()

	if cStr, ok := o.cachedLabels[label]; ok {
		return cStr
	}

	cStr := C.CString(label)
	o.cachedLabels[label] = cStr

	return cStr
}

// drawHintsInternal is the internal implementation for drawing hints.
//
// The placement is resolved once, before anything is drawn: it is the same for
// every label in the set, and both draw paths below take the resolved constant
// rather than the configured string, so a placement this overlay has no branch
// for cannot reach the Objective-C side.
//
// An empty set is the one draw the refusal does not cover. It means "take the
// labels off the screen", and a placement decides where a label goes, not
// whether one can be removed — refusing here would strand whatever is on
// screen there.
func (o *Overlay) drawHintsInternal(hints []*Hint, style StyleMode, showArrow bool) error {
	if len(hints) == 0 {
		o.Clear()

		return nil
	}

	placement, placementErr := hintPlacementValue(style.Placement())
	if placementErr != nil {
		// The error reaches the mode handler, which degrades quietly on
		// CodeNotSupported — so say it here too, or a placement this overlay
		// cannot draw costs the user every hint with only a debug line to
		// explain it. The placement is a fixed configuration keyword.
		o.logger.Warn(
			"Hint placement not drawn by the macos overlay",
			zap.String("placement", style.Placement()),
		)

		return placementErr
	}

	start := time.Now()

	// Extract current input from hints (find first hint with matched prefix)
	currentInput := ""
	for _, hint := range hints {
		if len(hint.MatchedPrefix()) > 0 {
			currentInput = hint.MatchedPrefix()

			break
		}
	}

	// Check if we can do incremental updates
	o.hintStateMu.RLock()
	canIncrementalUpdate := len(o.previousHints) > 0
	o.hintStateMu.RUnlock()

	if canIncrementalUpdate {
		// Try incremental update
		if o.drawHintsIncremental(hints, currentInput, style, placement, showArrow) {
			// Update cached state on successful incremental update
			o.hintStateMu.Lock()
			o.previousHints = make([]*Hint, len(hints))
			copy(o.previousHints, hints)
			o.previousInput = currentInput
			o.previousStyle = style
			o.hintStateMu.Unlock()

			if ce := o.logger.Check(
				zap.DebugLevel,
				"Hints incremental update successful",
			); ce != nil {
				ce.Write()
			}

			return nil
		}
		if ce := o.logger.Check(
			zap.DebugLevel,
			"Hints incremental update failed, falling back to full redraw",
		); ce != nil {
			ce.Write()
		}
	}

	// Hold drawMu.RLock for the entire span from label lookup through the C
	// draw call so that freeLabelCache (which takes drawMu.Lock) cannot free
	// the C strings while they are still referenced in the HintData slice.
	o.drawMu.RLock()

	tmpHints := hintDataPool.Get()
	cHintsPtr, _ := tmpHints.(*[]C.HintData)
	if cap(*cHintsPtr) < len(hints) {
		s := make([]C.HintData, len(hints))
		cHintsPtr = &s
	} else {
		*cHintsPtr = (*cHintsPtr)[:len(hints)]
	}
	cHints := *cHintsPtr

	matchedCount := 0
	for i, hint := range hints {
		cLabel := o.getOrCacheLabel(hint.Label())
		cHints[i] = C.HintData{
			label: cLabel,
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

	if ce := o.logger.Check(zap.DebugLevel, "Hint match statistics"); ce != nil {
		ce.Write(
			zap.Int("total_hints", len(hints)),
			zap.Int("matched_hints", matchedCount))
	}

	// Use cached style strings to avoid repeated allocations
	cachedStyle := o.styleCache.Get(func(_cachedStyle *overlayutil.CachedStyle) {
		_cachedStyle.FontFamily = unsafe.Pointer(C.CString(style.FontFamily()))
		_cachedStyle.BgColor = unsafe.Pointer(C.CString(style.BackgroundColor()))
		_cachedStyle.TextColor = unsafe.Pointer(C.CString(style.TextColor()))
		_cachedStyle.MatchedTextColor = unsafe.Pointer(C.CString(style.MatchedTextColor()))
		_cachedStyle.BorderColor = unsafe.Pointer(C.CString(style.BorderColor()))
		_cachedStyle.BoundaryBgColor = unsafe.Pointer(C.CString(style.BoundaryBackgroundColor()))
		_cachedStyle.BoundaryBorderColor = unsafe.Pointer(C.CString(style.BoundaryBorderColor()))
	})

	arrowFlag := 0
	if showArrow {
		arrowFlag = 1
	}

	finalStyle := C.HintStyle{
		fontSize:                 C.int(style.FontSize()),
		fontFamily:               (*C.char)(cachedStyle.FontFamily),
		backgroundColor:          (*C.char)(cachedStyle.BgColor),
		textColor:                (*C.char)(cachedStyle.TextColor),
		matchedTextColor:         (*C.char)(cachedStyle.MatchedTextColor),
		borderColor:              (*C.char)(cachedStyle.BorderColor),
		borderRadius:             C.int(style.BorderRadius()),
		borderWidth:              C.int(style.BorderWidth()),
		paddingX:                 C.int(style.PaddingX()),
		paddingY:                 C.int(style.PaddingY()),
		showArrow:                C.int(arrowFlag),
		placement:                C.int(placement),
		boundaryHighlightEnabled: C.int(boolToInt(style.BoundaryHighlightEnabled())),
		boundaryBorderWidth:      C.int(style.BoundaryBorderWidth()),
		boundaryBorderRadius:     C.int(style.BoundaryBorderRadius()),
		boundaryBackgroundColor:  (*C.char)(cachedStyle.BoundaryBgColor),
		boundaryBorderColor:      (*C.char)(cachedStyle.BoundaryBorderColor),
	}

	C.NeruDrawHints(o.window, &cHints[0], C.int(len(cHints)), finalStyle)

	o.drawMu.RUnlock()

	// Zero out cached-label pointers in the backing array before returning to pool.
	// After RUnlock, freeAllCaches could free the C strings these point to;
	// clearing them prevents any future pool consumer from seeing dangling pointers.
	for i := range *cHintsPtr {
		(*cHintsPtr)[i].label = nil
	}

	*cHintsPtr = (*cHintsPtr)[:0]
	hintDataPool.Put(cHintsPtr)
	// Note: We don't free cached label or style strings - they're reused across draws

	// Update cached state
	o.hintStateMu.Lock()
	o.previousHints = make([]*Hint, len(hints))
	copy(o.previousHints, hints)
	o.previousInput = currentInput
	o.previousStyle = style
	o.hintStateMu.Unlock()

	if ce := o.logger.Check(zap.DebugLevel, "Hints drawn successfully"); ce != nil {
		ce.Write(zap.Duration("duration", time.Since(start)))
	}

	return nil
}

// drawHintsIncremental performs incremental updates by only updating changed
// hints. It takes the placement already resolved from style, because the caller
// refuses one it cannot draw before either path begins.
func (o *Overlay) drawHintsIncremental(
	hints []*Hint,
	currentInput string,
	style StyleMode,
	placement int,
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
		placement,
		showArrow,
	)
}

// hintsAreStructurallyEqual checks if two hint lists have the same structure (same hints at same positions).
// It first tries a fast index-by-index comparison (O(n), zero allocations) which succeeds when
// the slices are in the same order — the common case since previousHints is a copy of the prior
// hints slice. Only if the fast path detects a mismatch does it fall back to a map-based lookup.
func (o *Overlay) hintsAreStructurallyEqual(hintsA, hintsB []*Hint) bool {
	if len(hintsA) != len(hintsB) {
		return false
	}

	// Fast path: index-by-index comparison (no allocation).
	// This succeeds when both slices are in the same order, which is the
	// typical case because previousHints is populated with copy().
	fastEqual := true
	for i, hintA := range hintsA {
		hintB := hintsB[i]
		if hintA.Position() != hintB.Position() ||
			hintA.Label() != hintB.Label() ||
			hintA.Size() != hintB.Size() {
			fastEqual = false

			break
		}
	}

	if fastEqual {
		return true
	}

	// Slow path: order differs, fall back to map-based lookup.
	hintsBMap := make(map[image.Point]*Hint, len(hintsB))
	for _, hint := range hintsB {
		hintsBMap[hint.Position()] = hint
	}

	// Check if all hints in hintsA exist in hintsB at the same position with same label and size
	for _, hintA := range hintsA {
		hintB, exists := hintsBMap[hintA.Position()]
		if !exists {
			return false
		}
		if hintA.Label() != hintB.Label() || hintA.Size() != hintB.Size() {
			return false
		}
	}

	return true
}

// updateMatchesIncremental updates match states incrementally when input changes.
func (o *Overlay) updateMatchesIncremental(newInput string) {
	cPrefix := C.CString(newInput)
	defer C.free(unsafe.Pointer(cPrefix))

	C.NeruUpdateHintMatchPrefix(o.window, cPrefix)

	if ce := o.logger.Check(zap.DebugLevel, "Incremental match update"); ce != nil {
		ce.Write(zap.String("new_input", newInput))
	}
}

// drawHintsIncrementalStructural handles structural changes using the
// incremental C API. Like the full redraw, it takes the resolved placement
// rather than the configured string.
func (o *Overlay) drawHintsIncrementalStructural(
	currentHints []*Hint,
	previousHints []*Hint,
	currentInput string,
	currentStyle StyleMode,
	previousInput string,
	previousStyle StyleMode,
	placement int,
	showArrow bool,
) bool {
	// Build maps for efficient lookup
	previousHintMap := make(map[image.Point]*Hint, len(previousHints))
	for _, hint := range previousHints {
		previousHintMap[hint.Position()] = hint
	}
	currentHintMap := make(map[image.Point]*Hint, len(currentHints))
	for _, hint := range currentHints {
		currentHintMap[hint.Position()] = hint
	}

	// Find hints to add/update (in current but not in previous, or changed)
	var hintsToAdd []*Hint
	for _, hint := range currentHints {
		prevHint, exists := previousHintMap[hint.Position()]
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
		if _, exists := currentHintMap[hint.Position()]; !exists {
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

	// Hold drawMu.RLock for the entire span from label lookup through the C
	// draw call so that freeLabelCache cannot free labels mid-draw.
	o.drawMu.RLock()

	// Convert hints to C structures (labels are cached, not freed per call)
	hintsToAddC := o.convertHintsToC(hintsToAdd, currentInput)

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
	cachedStyle := o.styleCache.Get(func(_cachedStyle *overlayutil.CachedStyle) {
		_cachedStyle.FontFamily = unsafe.Pointer(C.CString(currentStyle.FontFamily()))
		_cachedStyle.BgColor = unsafe.Pointer(C.CString(currentStyle.BackgroundColor()))
		_cachedStyle.TextColor = unsafe.Pointer(C.CString(currentStyle.TextColor()))
		_cachedStyle.MatchedTextColor = unsafe.Pointer(C.CString(currentStyle.MatchedTextColor()))
		_cachedStyle.BorderColor = unsafe.Pointer(C.CString(currentStyle.BorderColor()))
		_cachedStyle.BoundaryBgColor = unsafe.Pointer(
			C.CString(currentStyle.BoundaryBackgroundColor()),
		)
		_cachedStyle.BoundaryBorderColor = unsafe.Pointer(
			C.CString(currentStyle.BoundaryBorderColor()),
		)
	})

	arrowFlag := 0
	if showArrow {
		arrowFlag = 1
	}

	finalStyle := C.HintStyle{
		fontSize:                 C.int(currentStyle.FontSize()),
		fontFamily:               (*C.char)(cachedStyle.FontFamily),
		backgroundColor:          (*C.char)(cachedStyle.BgColor),
		textColor:                (*C.char)(cachedStyle.TextColor),
		matchedTextColor:         (*C.char)(cachedStyle.MatchedTextColor),
		borderColor:              (*C.char)(cachedStyle.BorderColor),
		borderRadius:             C.int(currentStyle.BorderRadius()),
		borderWidth:              C.int(currentStyle.BorderWidth()),
		paddingX:                 C.int(currentStyle.PaddingX()),
		paddingY:                 C.int(currentStyle.PaddingY()),
		showArrow:                C.int(arrowFlag),
		placement:                C.int(placement),
		boundaryHighlightEnabled: C.int(boolToInt(currentStyle.BoundaryHighlightEnabled())),
		boundaryBorderWidth:      C.int(currentStyle.BoundaryBorderWidth()),
		boundaryBorderRadius:     C.int(currentStyle.BoundaryBorderRadius()),
		boundaryBackgroundColor:  (*C.char)(cachedStyle.BoundaryBgColor),
		boundaryBorderColor:      (*C.char)(cachedStyle.BoundaryBorderColor),
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

	o.drawMu.RUnlock()

	if ce := o.logger.Check(zap.DebugLevel, "Incremental structural update"); ce != nil {
		ce.Write(
			zap.Int("hints_added", len(hintsToAdd)),
			zap.Int("hints_removed", len(positionsToRemove)))
	}

	return true
}

// convertHintsToC converts hint objects to C HintData structures.
// Caller must hold drawMu.RLock to prevent label cache invalidation while
// the returned structs reference cached C strings.
func (o *Overlay) convertHintsToC(hintsGo []*Hint, currentInput string) []C.HintData {
	if len(hintsGo) == 0 {
		return nil
	}

	cHints := make([]C.HintData, len(hintsGo))

	for hintIndex, hint := range hintsGo {
		cLabel := o.getOrCacheLabel(hint.Label())

		matchedPrefixLength := 0
		if currentInput != "" {
			matchedPrefixLength = len(hint.MatchedPrefix())
		}

		cHints[hintIndex] = C.HintData{
			label: cLabel,
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
