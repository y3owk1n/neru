//go:build darwin

package virtualpointer

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

// Overlay renders a cursor-following virtual pointer in its own small window.
const (
	// NSWindowSharingNone represents NSWindowSharingNone (0) - hidden from screen sharing.
	NSWindowSharingNone = 0
	// NSWindowSharingReadOnly represents NSWindowSharingReadOnly (1) - visible in screen sharing.
	NSWindowSharingReadOnly = 1
)

// Overlay is a virtual pointer overlay window.
type Overlay struct {
	window C.OverlayWindow
	config config.VirtualPointerConfig
	theme  config.ThemeProvider
	logger *zap.Logger

	configMu sync.RWMutex
}

// NewOverlay creates a dedicated virtual pointer overlay window.
func NewOverlay(
	cfg config.VirtualPointerConfig,
	theme config.ThemeProvider,
	logger *zap.Logger,
) (*Overlay, error) {
	base, err := overlayutil.NewBaseOverlay(logger)
	if err != nil {
		return nil, err
	}

	return &Overlay{
		window: C.OverlayWindow(base.Window),
		config: cfg,
		theme:  theme,
		logger: logger,
	}, nil
}

// SetConfig updates the virtual pointer configuration.
func (o *Overlay) SetConfig(cfg config.VirtualPointerConfig) {
	o.configMu.Lock()
	defer o.configMu.Unlock()

	o.config = cfg
}

// Show displays the virtual pointer overlay window.
func (o *Overlay) Show() {
	C.NeruShowOverlayWindow(o.window)
}

// Hide conceals the virtual pointer overlay window.
func (o *Overlay) Hide() {
	C.NeruHideOverlayWindow(o.window)
}

// Clear removes the virtual pointer from the overlay.
func (o *Overlay) Clear() {
	C.NeruHideCursorIndicator(o.window)
}

// SetSharingType sets the window sharing type for screen sharing visibility.
func (o *Overlay) SetSharingType(hide bool) {
	sharingType := C.int(NSWindowSharingReadOnly)
	if hide {
		sharingType = C.int(NSWindowSharingNone)
	}

	C.NeruSetOverlaySharingType(o.window, sharingType)
}

// ResizeToActiveScreen is a no-op; positioning is handled per draw.
func (o *Overlay) ResizeToActiveScreen() {}

// Draw positions the overlay on the cursor and renders the virtual pointer indicator.
// When a char is configured in the overlay's config, the character is drawn using the
// configured font and text color. Otherwise falls back to a filled circle using fillColor.
func (o *Overlay) Draw(xCoordinate, yCoordinate, size int, fillColor string) {
	if size < 1 || fillColor == "" {
		return
	}

	o.configMu.RLock()
	cfg := o.config
	o.configMu.RUnlock()

	cFillColor := C.CString(fillColor)
	cLabelChar := C.CString(cfg.UI.Char)
	cFontFamily := C.CString(cfg.UI.FontFamily)
	cTextColor := C.CString(cfg.UI.TextColor.ForTheme(
		o.theme, config.VirtualPointerTextColorLight, config.VirtualPointerTextColorDark,
	))

	defer C.free(unsafe.Pointer(cFillColor))  //nolint:nlreturn
	defer C.free(unsafe.Pointer(cLabelChar))  //nolint:nlreturn
	defer C.free(unsafe.Pointer(cFontFamily)) //nolint:nlreturn
	defer C.free(unsafe.Pointer(cTextColor))  //nolint:nlreturn

	C.NeruPositionAndDrawVirtualPointer(
		o.window,
		C.double(xCoordinate),
		C.double(yCoordinate),
		C.CursorIndicatorStyle{
			radius:     C.double(size),
			fillColor:  cFillColor,
			labelChar:  cLabelChar,
			fontFamily: cFontFamily,
			fontSize:   C.int(cfg.UI.FontSize),
			textColor:  cTextColor,
		},
	)
}

// Destroy releases the overlay window.
func (o *Overlay) Destroy() {
	if o.window != nil {
		C.NeruDestroyOverlayWindow(o.window)
		o.window = nil
	}
}
