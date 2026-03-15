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
)

const (
	stickyIndicatorWidth  = 60
	stickyIndicatorHeight = 20
)

// Overlay manages the rendering of sticky modifiers indicator overlay.
type Overlay struct {
	window     C.OverlayWindow
	uiConfig   config.StickyModifiersUI
	theme      config.ThemeProvider
	logger     *zap.Logger
	styleCache *overlayutil.StyleCache

	configMu sync.RWMutex
	drawMu   sync.Mutex

	labelCacheMu sync.RWMutex
	cachedLabels map[string]*C.char
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

// Draw draws the sticky modifier symbols at the specified position.
func (o *Overlay) Draw(xCoordinate, yCoordinate int, symbols string) {
	if symbols == "" {
		return
	}

	o.configMu.RLock()
	uiConfig := o.uiConfig
	o.configMu.RUnlock()

	bgColor := config.ResolveColor(
		uiConfig.BackgroundColorLight,
		uiConfig.BackgroundColorDark,
		o.theme,
		"#000000",
		"#FFFFFF",
	)

	textColor := config.ResolveColor(
		uiConfig.TextColorLight,
		uiConfig.TextColorDark,
		o.theme,
		"#FFFFFF",
		"#000000",
	)

	borderColor := config.ResolveColor(
		uiConfig.BorderColorLight,
		uiConfig.BorderColorDark,
		o.theme,
		"#FFFFFF",
		"#000000",
	)

	o.drawMu.Lock()
	defer o.drawMu.Unlock()

	C.NeruShowOverlayWindow(o.window)

	o.styleCache.Free()

	label := o.getOrCacheLabel(symbols)

	bgColorC := C.CString(bgColor)
	defer C.free(unsafe.Pointer(bgColorC)) //nolint:nlreturn

	textColorC := C.CString(textColor)
	defer C.free(unsafe.Pointer(textColorC)) //nolint:nlreturn

	borderColorC := C.CString(borderColor)
	defer C.free(unsafe.Pointer(borderColorC)) //nolint:nlreturn

	hint := C.HintData{
		label: label,
		position: C.CGPoint{
			x: C.double(xCoordinate),
			y: C.double(yCoordinate),
		},
		size: C.CGSize{
			width:  stickyIndicatorWidth,
			height: stickyIndicatorHeight,
		},
		matchedPrefixLength: 0,
	}

	style := C.HintStyle{
		fontSize:        C.int(uiConfig.FontSize),
		fontFamily:      nil,
		backgroundColor: bgColorC,
		textColor:       textColorC,
		borderColor:     borderColorC,
		borderRadius:    C.int(uiConfig.BorderRadius),
		borderWidth:     C.int(uiConfig.BorderWidth),
		paddingX:        C.int(uiConfig.PaddingX),
		paddingY:        C.int(uiConfig.PaddingY),
		showArrow:       0,
	}

	C.NeruDrawHints(o.window, &hint, 1, style)
}

// SetConfig updates the overlay configuration.
func (o *Overlay) SetConfig(uiCfg config.StickyModifiersUI) {
	o.configMu.Lock()
	defer o.configMu.Unlock()
	o.uiConfig = uiCfg
}

// ResizeToActiveScreen adjusts the overlay window size to the active screen.
func (o *Overlay) ResizeToActiveScreen() {
	C.NeruResizeOverlayToActiveScreen(o.window)
}

func (o *Overlay) getOrCacheLabel(text string) *C.char {
	o.labelCacheMu.RLock()
	label, exists := o.cachedLabels[text]
	o.labelCacheMu.RUnlock()

	if exists {
		return label
	}

	o.labelCacheMu.Lock()
	defer o.labelCacheMu.Unlock()

	if label, exists := o.cachedLabels[text]; exists {
		return label
	}

	cText := C.CString(text)
	o.cachedLabels[text] = cText

	return cText
}
