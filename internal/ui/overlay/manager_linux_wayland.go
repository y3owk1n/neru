//go:build linux && !cgo

package overlay

// Linux Wayland overlay manager backend placeholder.
//
// Native Wayland/layer-shell overlay management should be implemented here.

import (
	"image"
	"sync"
	"unsafe"

	"go.uber.org/zap"

	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
)

type wlrootsOverlay struct {
	sublayerKeys string
}

func newWlrootsOverlay(logger *zap.Logger) *wlrootsOverlay {
	_ = logger

	return nil
}

func (o *wlrootsOverlay) Healthy() bool { return false }
func (o *wlrootsOverlay) WindowPtr() unsafe.Pointer {
	return nil
}
func (o *wlrootsOverlay) Show()                                                  {}
func (o *wlrootsOverlay) Hide()                                                  {}
func (o *wlrootsOverlay) Clear()                                                 {}
func (o *wlrootsOverlay) ClearRect(image.Rectangle)                              {}
func (o *wlrootsOverlay) Resize()                                                {}
func (o *wlrootsOverlay) Destroy()                                               {}
func (o *wlrootsOverlay) UpdateGridMatches(string)                               {}
func (o *wlrootsOverlay) ShowSubgrid(*domainGrid.Cell, gridcomponent.Style)      {}
func (o *wlrootsOverlay) SetHideUnmatched(bool)                                  {}
func (o *wlrootsOverlay) DrawGrid(*domainGrid.Grid, string, gridcomponent.Style) {}
func (o *wlrootsOverlay) DrawHints([]*hintscomponent.Hint, hintscomponent.StyleMode) {
}

func (o *wlrootsOverlay) DrawRecursiveGrid(
	image.Rectangle,
	int,
	string,
	int,
	int,
	string,
	int,
	int,
	recursivegridcomponent.Style,
	recursivegridcomponent.VirtualPointerState,
	bool,
	int,
) {
}

func (o *wlrootsOverlay) DrawRecursiveGridWithSubKeyPreview(
	image.Rectangle,
	int,
	string,
	int,
	int,
	string,
	int,
	int,
	recursivegridcomponent.Style,
	recursivegridcomponent.VirtualPointerState,
	bool,
	int,
) {
}
func (o *wlrootsOverlay) Flush() {}

func (o *wlrootsOverlay) DrawBadge(int, int, string, overlayColors, overlayBadgeStyle) {}

func (o *wlrootsOverlay) cancelAnimation()               {}
func (o *wlrootsOverlay) setDisplayMu(_ *sync.Mutex)     {}
func (o *wlrootsOverlay) setKeyboardCaptureEnabled(bool) {}
func (o *wlrootsOverlay) startPoller()                   {}
