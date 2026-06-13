//go:build darwin

// Package stickyindicator provides the sticky modifiers indicator overlay component.
package stickyindicator

/*
#cgo CFLAGS: -x objective-c
#include "../../../core/infra/platform/darwin/overlay.h"
#include <stdlib.h>
*/
import "C"

import (
	"sync"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components/overlayutil"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/ports"
)

const (
	// NSWindowSharingNone represents NSWindowSharingNone (0) - hidden from screen sharing.
	NSWindowSharingNone = 0
	// NSWindowSharingReadOnly represents NSWindowSharingReadOnly (1) - visible in screen sharing.
	NSWindowSharingReadOnly = 1
)

// Overlay manages the rendering of sticky modifiers indicator overlay.
type Overlay struct {
	window     C.OverlayWindow
	uiConfig   config.StickyModifiersUI
	theme      config.ThemeProvider
	logger     *zap.Logger
	styleCache *overlayutil.StyleCache

	configMu sync.RWMutex

	// Cached C strings for labels to avoid malloc/free per draw.
	labelCacheMu sync.RWMutex
	cachedLabels map[string]*C.char

	// drawMu serializes draw operations against cache invalidation.
	drawMu sync.Mutex
}

// NewOverlay initializes a new sticky modifiers indicator overlay.
func NewOverlay(
	uiConfig config.StickyModifiersUI,
	theme config.ThemeProvider,
	logger *zap.Logger,
) (*Overlay, error) {
	base, err := overlayutil.NewBaseOverlay(logger)
	if err != nil {
		return nil, err
	}

	return &Overlay{
		window:       (C.OverlayWindow)(base.Window),
		uiConfig:     uiConfig,
		theme:        theme,
		logger:       logger,
		styleCache:   base.StyleCache,
		cachedLabels: make(map[string]*C.char),
	}, nil
}

// NewOverlayWithWindow initializes a sticky modifiers indicator overlay using a shared window.
func NewOverlayWithWindow(
	uiConfig config.StickyModifiersUI,
	theme config.ThemeProvider,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) (*Overlay, error) {
	base := overlayutil.NewBaseOverlayWithWindow(logger, windowPtr)

	return &Overlay{
		window:       (C.OverlayWindow)(base.Window),
		uiConfig:     uiConfig,
		theme:        theme,
		logger:       logger,
		styleCache:   base.StyleCache,
		cachedLabels: make(map[string]*C.char),
	}, nil
}

// Show displays the sticky modifiers indicator overlay window.
func (o *Overlay) Show() {
	C.NeruShowOverlayWindow(o.window)
}

// Hide conceals the sticky modifiers indicator overlay window.
func (o *Overlay) Hide() {
	C.NeruHideOverlayWindow(o.window)
}

// Clear removes all content from the overlay.
func (o *Overlay) Clear() {
	C.NeruClearOverlay(o.window)
}

// ResizeToActiveScreen is a no-op for sticky indicator overlays.
// The sticky indicator uses a small window positioned dynamically by Draw via
// NeruPositionAndSizeOverlayToFitHint, so full-screen resizing is unnecessary and
// would defeat the small-window memory optimization.
func (o *Overlay) ResizeToActiveScreen() {
	// No-op: positioning is handled dynamically in Draw.
}

// Draw draws the sticky modifier symbols near the specified cursor position.
// xCoordinate and yCoordinate are absolute Quartz screen coordinates. The
// overlay positions a small window around the indicator badge instead of
// using a full-screen window, saving some memory of backing store per Retina
// display. The native side clamps the window to a single display to prevent
// the multi-monitor flicker regression.
//
// X/Y offsets from uiConfig are applied internally (under configMu) to match
// the mode indicator pattern. The caller must call Show() before Draw() for
// the content to be visible.
func (o *Overlay) Draw(xCoordinate, yCoordinate int, symbols string) {
	if symbols == "" {
		return
	}

	// Hold configMu.RLock for entire draw to prevent SetConfig from
	// writing to uiConfig while we read it.
	o.configMu.RLock()
	defer o.configMu.RUnlock()

	// Offset from cursor to avoid covering it
	xOffset := o.uiConfig.IndicatorXOffset
	yOffset := o.uiConfig.IndicatorYOffset

	o.drawMu.Lock()
	defer o.drawMu.Unlock()

	label := o.getOrCacheLabel(symbols)

	cachedStyle := o.styleCache.Get(func(cached *overlayutil.CachedStyle) {
		cached.FontFamily = unsafe.Pointer(
			C.CString(ports.ResolveFont(o.uiConfig.FontFamily, false)),
		)
		cached.BgColor = unsafe.Pointer(
			C.CString(
				o.uiConfig.BackgroundColor.ForTheme(
					o.theme,
					config.StickyModifiersBackgroundColorLight,
					config.StickyModifiersBackgroundColorDark,
				),
			),
		)
		cached.TextColor = unsafe.Pointer(
			C.CString(
				o.uiConfig.TextColor.ForTheme(
					o.theme,
					config.StickyModifiersTextColorLight,
					config.StickyModifiersTextColorDark,
				),
			),
		)
		// No matching in indicator mode; reuse TextColor.
		cached.MatchedTextColor = unsafe.Pointer(
			C.CString(
				o.uiConfig.TextColor.ForTheme(
					o.theme,
					config.StickyModifiersTextColorLight,
					config.StickyModifiersTextColorDark,
				),
			),
		)
		cached.BorderColor = unsafe.Pointer(
			C.CString(
				o.uiConfig.BorderColor.ForTheme(
					o.theme,
					config.StickyModifiersBorderColorLight,
					config.StickyModifiersBorderColorDark,
				),
			),
		)
	})

	style := C.HintStyle{
		fontSize:         C.int(o.uiConfig.FontSize),
		fontFamily:       (*C.char)(cachedStyle.FontFamily),
		backgroundColor:  (*C.char)(cachedStyle.BgColor),
		textColor:        (*C.char)(cachedStyle.TextColor),
		matchedTextColor: (*C.char)(cachedStyle.MatchedTextColor),
		borderColor:      (*C.char)(cachedStyle.BorderColor),
		borderRadius:     C.int(o.uiConfig.BorderRadius),
		borderWidth:      C.int(o.uiConfig.BorderWidth),
		paddingX:         C.int(o.uiConfig.PaddingX),
		paddingY:         C.int(o.uiConfig.PaddingY),
		showArrow:        0,
		forceFlush:       1, // Indicator must flush synchronously to avoid first-show empty-badge flash
	}

	// Position the small window centered on the indicator's absolute position.
	// We calculate the hint size dynamically and size/position the window accordingly,
	// returning the calculated width and height.
	indicatorAbsX := xCoordinate + xOffset
	indicatorAbsY := yCoordinate + yOffset
	var windowWidth, windowHeight C.double
	C.NeruPositionAndSizeOverlayToFitHint(
		o.window,
		C.double(indicatorAbsX),
		C.double(indicatorAbsY),
		label,
		style,
		&windowWidth,
		&windowHeight,
	)

	hint := C.HintData{
		label: label,
		position: C.CGPoint{
			x: C.double(windowWidth / 2),  //nolint:mnd
			y: C.double(windowHeight / 2), //nolint:mnd
		},
		size: C.CGSize{
			width:  windowWidth,
			height: windowHeight,
		},
		matchedPrefixLength: 0,
	}

	C.NeruDrawHints(o.window, &hint, 1, style)
}

// SetConfig updates the overlay configuration.
func (o *Overlay) SetConfig(uiCfg config.StickyModifiersUI) {
	o.configMu.Lock()
	o.uiConfig = uiCfg

	o.configMu.Unlock()

	o.freeAllCaches()
}

// SetSharingType sets the window sharing type for screen sharing visibility.
func (o *Overlay) SetSharingType(hide bool) {
	sharingType := C.int(NSWindowSharingReadOnly)
	if hide {
		sharingType = C.int(NSWindowSharingNone)
	}
	C.NeruSetOverlaySharingType(o.window, sharingType)
}

// Destroy releases the overlay window resources.
func (o *Overlay) Destroy() {
	o.freeAllCaches()
	if o.window != nil {
		C.NeruDestroyOverlayWindow(o.window)
		o.window = nil
	}
}

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
func (o *Overlay) getOrCacheLabel(text string) *C.char {
	o.labelCacheMu.RLock()
	if label, ok := o.cachedLabels[text]; ok {
		o.labelCacheMu.RUnlock()

		return label
	}

	o.labelCacheMu.RUnlock()
	o.labelCacheMu.Lock()
	defer o.labelCacheMu.Unlock()

	// Double-check
	if label, ok := o.cachedLabels[text]; ok {
		return label
	}

	label := C.CString(text)
	o.cachedLabels[text] = label

	return label
}
