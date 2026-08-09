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
	"sync"
	"time"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	_ "github.com/y3owk1n/neru/internal/adapter/platform/linux"
	_ "github.com/y3owk1n/neru/internal/adapter/platform/linux/wlr_protocol"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

func init() {
	wlrootsKeyboardCh = make(chan string, keyboardChanBuffer)
}

// wlrootsOverlay is the wlroots layer-shell backend: the cgo connection, the
// shared-memory buffer pool, the keyboard poller, and the overlaySurface
// primitives. All drawing and animation logic lives on the embedded
// sharedOverlay; the exported methods below are nil-guarded delegates into it
// (the manager calls them on possibly-nil pointers).
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
	return o != nil && o.raw != nil
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

func (o *wlrootsOverlay) Hide() {
	if o != nil && o.raw != nil {
		o.hide()
	}
}

func (o *wlrootsOverlay) Clear() {
	if o != nil && o.raw != nil {
		o.clear()
	}
}

func (o *wlrootsOverlay) ClearRect(rect image.Rectangle) {
	if o != nil && o.raw != nil && !rect.Empty() {
		o.clearRect(rect)
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

func (o *wlrootsOverlay) UpdateGridMatches(prefix string) {
	if o == nil || o.raw == nil {
		return
	}

	o.updateGridMatches(prefix)
}

func (o *wlrootsOverlay) ShowSubgrid(cell *domainGrid.Cell, _ gridcomponent.Style) {
	if o == nil || o.raw == nil || cell == nil {
		return
	}

	o.showSubgrid(cell)
}

func (o *wlrootsOverlay) SetHideUnmatched(hide bool) {
	if o == nil {
		return
	}

	o.hideUnmatched = hide
}

func (o *wlrootsOverlay) DrawGrid(g *domainGrid.Grid, input string, style gridcomponent.Style) {
	if o == nil || o.raw == nil || g == nil {
		return
	}

	o.drawGrid(g, input, style)
}

func (o *wlrootsOverlay) DrawRecursiveGridWithSubKeyPreview(
	bounds image.Rectangle,
	depth int,
	keys string,
	dims domain.GridDimensions,
	nextKeys string,
	nextDims domain.GridDimensions,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
	animEnabled bool,
	animDurationMS int,
) {
	if o == nil || o.raw == nil || bounds.Empty() || dims.Cols <= 0 || dims.Rows <= 0 {
		return
	}

	o.drawRecursiveGridWithSubKeyPreview(
		bounds, depth, keys, dims,
		nextKeys, nextDims,
		style, virtualPointer, animEnabled, animDurationMS,
	)
}

func (o *wlrootsOverlay) DrawBadge(
	posX, posY int,
	text string,
	colors overlayColors,
	style overlayBadgeStyle,
) {
	if o == nil || o.raw == nil || text == "" {
		return
	}

	o.drawBadge(posX, posY, text, colors, style)
}

func (o *wlrootsOverlay) Flush() {
	if o == nil || o.raw == nil {
		return
	}
	C.neru_wayland_overlay_flush(o.raw)
}

func (o *wlrootsOverlay) DrawMonitorSelect(
	targets []manager.MonitorSelectTarget,
	style manager.MonitorSelectStyle,
) {
	if o == nil || o.raw == nil {
		return
	}

	o.drawMonitorSelect(targets, style)
}

func (o *wlrootsOverlay) DrawHints(
	hintsSlice []*hintscomponent.Hint,
	style hintscomponent.StyleMode,
	offset badge.HintOffset,
) {
	if o == nil || o.raw == nil {
		return
	}

	o.drawHints(hintsSlice, style, offset)
}

func (o *wlrootsOverlay) DrawMouseActionIndicator(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
) {
	if o == nil || o.raw == nil {
		return
	}

	o.drawMouseActionIndicator(point, style)
}

// setDisplayMu wires the mutex that serializes wl_display access between
// rendering and the keyboard poller — the Wayland client API is not
// thread-safe. It is the Wayland-flavored name for the shared render mutex;
// the manager passes the same lock it holds around synchronous draws.
func (o *wlrootsOverlay) setDisplayMu(mu *sync.Mutex) {
	o.setRenderMuShared(mu)
}

func (o *wlrootsOverlay) setOriginOffset(origin image.Point) {
	if o == nil {
		return
	}

	o.sharedOverlay.setOriginOffset(origin)
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
