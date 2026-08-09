//go:build linux && cgo

package linux

/*
#cgo linux pkg-config: wayland-client cairo xkbcommon
#include <stdlib.h>
#include "../../platform/linux/overlay_wayland.h"
*/
import "C"

import (
	"image"
	"time"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	_ "github.com/y3owk1n/neru/internal/adapter/platform/linux"
	_ "github.com/y3owk1n/neru/internal/adapter/platform/linux/wlr_protocol"
)

func init() {
	wlrootsKeyboardCh = make(chan string, keyboardChanBuffer)
}

// wlrootsOverlay is the wlroots layer-shell backend: the cgo connection, the
// shared-memory buffer pool, the keyboard poller, and the overlaySurface
// primitives. Everything above the primitives — drawing, animation, and the
// exported methods the manager calls — lives on the embedded sharedOverlay and
// is promoted from there. What stays here is what Wayland does differently:
// Show (buffer setup first), Resize (nothing; layer shells auto-resize),
// Destroy (waits on the poller), the poller itself, and the primitives.
type wlrootsOverlay struct {
	sharedOverlay

	raw    *C.NeruWaylandOverlay
	logger *zap.Logger

	// stopCh/doneCh belong to the keyboard poller's lifecycle, distinct from
	// the animation channels on sharedOverlay. Destroy closes stopCh exactly
	// once and then waits on doneCh before tearing down the display.
	stopCh chan struct{}
	doneCh chan struct{}
}

func newWlrootsOverlay(logger *zap.Logger) *wlrootsOverlay {
	raw := C.neru_wayland_overlay_new()
	if raw == nil {
		return nil
	}

	C.neru_wayland_overlay_setup_buffers(raw)

	overlay := &wlrootsOverlay{
		raw:    raw,
		logger: logger,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	overlay.srf = overlay

	return overlay
}

func (o *wlrootsOverlay) Healthy() bool {
	return o.alive()
}

func (o *wlrootsOverlay) WindowPtr() unsafe.Pointer {
	if o == nil {
		return nil
	}

	return unsafe.Pointer(o.raw)
}

func (o *wlrootsOverlay) Show() {
	if o != nil && o.raw != nil {
		C.neru_wayland_overlay_setup_buffers(o.raw)
		C.neru_wayland_overlay_show(o.raw)
	}
}

// Resize is a no-op: Wayland layer shells auto-resize with their output.
func (o *wlrootsOverlay) Resize() {}

func (o *wlrootsOverlay) Destroy() {
	if o == nil || o.raw == nil {
		return
	}

	o.cancelAnimation()
	close(o.stopCh)
	<-o.doneCh

	C.neru_wayland_overlay_destroy(o.raw)
	o.raw = nil
}

// alive answers the overlaySurface question the shared delegates ask before
// they draw: is the native handle still open. It is nil-receiver safe so
// Healthy, which the manager may reach on a backend it never built, has one
// implementation to defer to.
func (o *wlrootsOverlay) alive() bool {
	return o != nil && o.raw != nil
}

// startPoller launches the keyboard poller goroutine. Called once per overlay
// by the manager; the poller exits when Destroy closes stopCh.
func (o *wlrootsOverlay) startPoller() {
	go o.keyboardPoller()
}

// setKeyboardCaptureEnabled toggles whether the layer surface requests
// exclusive keyboard focus. Disabled for the click indicator and whenever an
// evdev grab already owns the keyboard (a layer-surface grab would deactivate
// the focused toplevel).
func (o *wlrootsOverlay) setKeyboardCaptureEnabled(enabled bool) {
	if o == nil || o.raw == nil {
		return
	}

	cEnabled := C.int(0)
	if enabled {
		cEnabled = 1
	}

	C.neru_wayland_overlay_set_keyboard_capture(o.raw, cEnabled)
}

// keyboardPoller pumps the wl_display for keyboard events and forwards decoded
// keys to wlrootsKeyboardCh, taking the render mutex around every native call.
func (o *wlrootsOverlay) keyboardPoller() {
	defer close(o.doneCh)

	const pollInterval = 5 * time.Millisecond

	for {
		select {
		case <-o.stopCh:
			return
		default:
		}

		var keys []string

		if o.renderMu != nil {
			o.renderMu.Lock()
		}

		if C.neru_wayland_overlay_poll(o.raw) < 0 {
			if o.renderMu != nil {
				o.renderMu.Unlock()
			}

			return
		}

		for {
			key := C.neru_wayland_overlay_get_key(o.raw)
			if key == nil {
				break
			}

			keys = append(keys, C.GoString(key))
		}

		if o.renderMu != nil {
			o.renderMu.Unlock()
		}

		if len(keys) > 0 {
			for _, k := range keys {
				select {
				case wlrootsKeyboardCh <- k:
				default:
				}
			}
		} else {
			time.Sleep(pollInterval)
		}
	}
}

// selectAvailableBuffer picks a buffer that the compositor has released.
// Falls back to sync (roundtrip) if none free, which forces release processing.
func (o *wlrootsOverlay) selectAvailableBuffer() bool {
	if o == nil || o.raw == nil {
		return false
	}
	C.neru_wayland_overlay_dispatch_pending(o.raw)
	bufIdx := C.neru_wayland_overlay_available_buffer(o.raw)
	if bufIdx < 0 {
		C.neru_wayland_overlay_sync(o.raw)
		bufIdx = C.neru_wayland_overlay_available_buffer(o.raw)
	}
	if bufIdx < 0 {
		return false
	}
	C.neru_wayland_overlay_select_buffer(o.raw, bufIdx)

	return true
}

// --- overlaySurface primitives ---

// surfaceScale is 1: Wayland renders in logical coordinates and the
// compositor scales the buffer.
func (o *wlrootsOverlay) surfaceScale() float64 { return 1 }

func (o *wlrootsOverlay) ensureBuffers() {
	C.neru_wayland_overlay_setup_buffers(o.raw)
}

// beginFrame makes a released shared-memory buffer current; reports false
// when none is available and the frame must be dropped.
func (o *wlrootsOverlay) beginFrame() bool {
	return o.selectAvailableBuffer()
}

func (o *wlrootsOverlay) surfaceClear() {
	C.neru_wayland_overlay_clear(o.raw)
}

// clearFrame is a plain clear: beginFrame already selected the buffer.
func (o *wlrootsOverlay) clearFrame() {
	C.neru_wayland_overlay_clear(o.raw)
}

func (o *wlrootsOverlay) surfaceClearRect(rect image.Rectangle) {
	C.neru_wayland_overlay_clear_rect(
		o.raw,
		C.double(rect.Min.X),
		C.double(rect.Min.Y),
		C.double(rect.Dx()),
		C.double(rect.Dy()),
	)
}

func (o *wlrootsOverlay) surfaceFlush() {
	C.neru_wayland_overlay_flush(o.raw)
}

func (o *wlrootsOverlay) surfaceHide() {
	C.neru_wayland_overlay_hide(o.raw)
}

// showIndicator maps the indicator surface under the render lock — the
// keyboard poller runs concurrently on this overlay's wl_display. The surface
// must never steal keyboard focus from the app it decorates, so keyboard
// capture is switched off before mapping.
func (o *wlrootsOverlay) showIndicator() {
	if o.renderMu != nil {
		o.renderMu.Lock()
	}

	C.neru_wayland_overlay_setup_buffers(o.raw)
	C.neru_wayland_overlay_set_keyboard_capture(o.raw, C.int(0))
	C.neru_wayland_overlay_show(o.raw)
	C.neru_wayland_overlay_sync(o.raw)

	if o.renderMu != nil {
		o.renderMu.Unlock()
	}
}

// finishIndicator only hides: clearing would write into an unselected buffer.
func (o *wlrootsOverlay) finishIndicator() {
	C.neru_wayland_overlay_hide(o.raw)
}

func (o *wlrootsOverlay) syncBeforeAnimation() {
	C.neru_wayland_overlay_sync(o.raw)
}

func (o *wlrootsOverlay) rectPrim(
	bounds image.Rectangle,
	fill, border uint32, lineWidth float64,
) {
	C.neru_wayland_overlay_rect(
		o.raw,
		C.double(bounds.Min.X), C.double(bounds.Min.Y),
		C.double(bounds.Dx()), C.double(bounds.Dy()),
		C.uint(fill), C.uint(border), C.double(lineWidth),
	)
}

func (o *wlrootsOverlay) roundedRectPrim(
	bounds image.Rectangle,
	radius float64,
	fill, border uint32, lineWidth float64,
) {
	C.neru_wayland_overlay_rounded_rect(
		o.raw,
		C.double(bounds.Min.X), C.double(bounds.Min.Y),
		C.double(bounds.Dx()), C.double(bounds.Dy()),
		C.double(radius),
		C.uint(fill), C.uint(border), C.double(lineWidth),
	)
}

func (o *wlrootsOverlay) hintBadgePrim(
	badgeRect image.Rectangle, radius float64, edge int, arrow badge.HintArrow,
	fill, border uint32, lineWidth float64,
) {
	C.neru_wayland_overlay_hint_badge(
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

func (o *wlrootsOverlay) textPrim(
	text, fontFamily string,
	centerX, centerY, fontSize float64, color uint32,
) {
	cText := C.CString(text)
	cFontFamily := C.CString(fontFamily)

	defer C.free(unsafe.Pointer(cText))
	defer C.free(unsafe.Pointer(cFontFamily))

	C.neru_wayland_overlay_text(
		o.raw, cText, cFontFamily,
		C.double(centerX),
		C.double(centerY),
		C.double(fontSize), C.uint(color),
	)
}
