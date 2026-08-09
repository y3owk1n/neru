//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: x11 xrender xfixes xext cairo
#include <stdlib.h>
#include "../../platform/linux/x11_overlay.h"
*/
import "C"

import (
	"image"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	_ "github.com/y3owk1n/neru/internal/adapter/platform/linux"
)

// x11Overlay is the X11 backend: the cgo connection, the HiDPI scale probe,
// and the overlaySurface primitives. Everything above the primitives —
// drawing, animation, and the exported methods the manager calls — lives on
// the embedded sharedOverlay and is promoted from there. What stays here is
// what X11 does differently: Show, Resize and Destroy, the scale probe, and
// the primitives themselves.
type x11Overlay struct {
	sharedOverlay

	raw *C.NeruX11Overlay
	// scale is the desktop-wide HiDPI UI factor from Xft.dpi (>= 1.0). X11 has a
	// single device-pixel coordinate space and no per-monitor scale, so hint/label
	// positions stay in device pixels and only element sizes (fonts, stroke widths,
	// badge geometry) are multiplied by this factor for legibility on HiDPI screens.
	scale  float64
	logger *zap.Logger
}

func newX11Overlay(logger *zap.Logger) *x11Overlay {
	raw := C.neru_x11_overlay_new()
	if raw == nil {
		return nil
	}

	scale := float64(C.neru_x11_overlay_scale(raw))
	if scale <= 0 {
		scale = 1
	}

	overlay := &x11Overlay{raw: raw, logger: logger, scale: scale}
	overlay.srf = overlay

	return overlay
}

// Scale exposes the overlay's HiDPI scale so the manager can size badge
// geometry (and its clear rects) consistently with what the overlay renders.
func (o *x11Overlay) Scale() float64 {
	return o.s()
}

func (o *x11Overlay) Healthy() bool {
	return o.alive()
}

func (o *x11Overlay) WindowPtr() unsafe.Pointer {
	if o == nil {
		return nil
	}

	return unsafe.Pointer(o.raw)
}

func (o *x11Overlay) Show() {
	if o != nil && o.raw != nil {
		C.neru_x11_overlay_show(o.raw)
	}
}

func (o *x11Overlay) Resize() {
	if o != nil && o.raw != nil {
		C.neru_x11_overlay_resize(o.raw)
	}
}

func (o *x11Overlay) Destroy() {
	if o != nil && o.raw != nil {
		o.cancelAnimation()
		C.neru_x11_overlay_destroy(o.raw)
		o.raw = nil
	}
}

// alive answers the overlaySurface question the shared delegates ask before
// they draw: is the native handle still open. It is nil-receiver safe so
// Healthy, which the manager may reach on a backend it never built, has one
// implementation to defer to.
func (o *x11Overlay) alive() bool {
	return o != nil && o.raw != nil
}

// s returns the overlay's HiDPI scale factor, guarding against a zero value.
func (o *x11Overlay) s() float64 {
	if o == nil || o.scale <= 0 {
		return 1
	}

	return o.scale
}

// --- overlaySurface primitives ---

func (o *x11Overlay) surfaceScale() float64 { return o.s() }

// ensureBuffers is a no-op: X11 draws into a single server-side pixmap.
func (o *x11Overlay) ensureBuffers() {}

// beginFrame always succeeds: X11 has no buffer pool to run dry.
func (o *x11Overlay) beginFrame() bool { return true }

func (o *x11Overlay) surfaceClear() {
	C.neru_x11_overlay_clear(o.raw)
}

// clearFrame uses the double-buffered clear so animation frames repaint
// without flicker.
func (o *x11Overlay) clearFrame() {
	C.neru_x11_overlay_clear_buffered(o.raw)
}

func (o *x11Overlay) surfaceClearRect(rect image.Rectangle) {
	C.neru_x11_overlay_clear_rect(
		o.raw,
		C.int(rect.Min.X),
		C.int(rect.Min.Y),
		C.int(rect.Dx()),
		C.int(rect.Dy()),
	)
}

func (o *x11Overlay) surfaceFlush() {
	C.neru_x11_overlay_flush(o.raw)
}

func (o *x11Overlay) surfaceHide() {
	C.neru_x11_overlay_hide(o.raw)
}

func (o *x11Overlay) showIndicator() {
	C.neru_x11_overlay_show(o.raw)
}

// finishIndicator clears before unmapping so no stale pixmap shows on the
// next map.
func (o *x11Overlay) finishIndicator() {
	C.neru_x11_overlay_clear(o.raw)
	C.neru_x11_overlay_hide(o.raw)
}

func (o *x11Overlay) syncBeforeAnimation() {}

func (o *x11Overlay) rectPrim(
	bounds image.Rectangle,
	fill, border uint32, lineWidth float64,
) {
	C.neru_x11_overlay_rect(
		o.raw,
		C.double(bounds.Min.X), C.double(bounds.Min.Y),
		C.double(bounds.Dx()), C.double(bounds.Dy()),
		C.uint(fill), C.uint(border), C.double(lineWidth),
	)
}

func (o *x11Overlay) roundedRectPrim(
	bounds image.Rectangle,
	radius float64,
	fill, border uint32, lineWidth float64,
) {
	C.neru_x11_overlay_rounded_rect(
		o.raw,
		C.double(bounds.Min.X), C.double(bounds.Min.Y),
		C.double(bounds.Dx()), C.double(bounds.Dy()),
		C.double(radius),
		C.uint(fill), C.uint(border), C.double(lineWidth),
	)
}

func (o *x11Overlay) hintBadgePrim(
	badgeRect image.Rectangle, radius float64, edge int, arrow badge.HintArrow,
	fill, border uint32, lineWidth float64,
) {
	C.neru_x11_overlay_hint_badge(
		o.raw,
		C.double(badgeRect.Min.X), C.double(badgeRect.Min.Y),
		C.double(badgeRect.Dx()), C.double(badgeRect.Dy()),
		C.double(radius),
		C.int(edge),
		C.double(arrow.BaseLeft.X), C.double(arrow.BaseRight.X),
		C.double(arrow.Tip.X), C.double(arrow.Tip.Y),
		C.uint(fill), C.uint(border), C.double(lineWidth),
	)
}

func (o *x11Overlay) textPrim(
	text, fontFamily string,
	centerX, centerY, fontSize float64, color uint32,
) {
	cText := C.CString(text)
	cFontFamily := C.CString(fontFamily)

	defer C.free(unsafe.Pointer(cText))
	defer C.free(unsafe.Pointer(cFontFamily))

	C.neru_x11_overlay_text(
		o.raw, cText, cFontFamily,
		C.double(centerX),
		C.double(centerY),
		C.double(fontSize), C.uint(color),
	)
}
