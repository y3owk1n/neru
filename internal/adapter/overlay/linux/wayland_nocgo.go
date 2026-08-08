//go:build linux && !cgo

package linux

import (
	"image"
	"sync"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
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
func (o *wlrootsOverlay) setOriginOffset(image.Point)                            {}
func (o *wlrootsOverlay) DrawGrid(*domainGrid.Grid, string, gridcomponent.Style) {}
func (o *wlrootsOverlay) DrawHints(
	[]*hintscomponent.Hint,
	hintscomponent.StyleMode,
	hintBadgeOffset,
) {
}

func (o *wlrootsOverlay) DrawRecursiveGridWithSubKeyPreview(
	image.Rectangle,
	int,
	string,
	domain.GridDimensions,
	string,
	domain.GridDimensions,
	recursivegridcomponent.Style,
	recursivegridcomponent.VirtualPointerState,
	bool,
	int,
) {
}
func (o *wlrootsOverlay) Flush() {}

func (o *wlrootsOverlay) DrawBadge(int, int, string, overlayColors, overlayBadgeStyle) {}

func (o *wlrootsOverlay) DrawMonitorSelect(
	[]manager.MonitorSelectTarget,
	manager.MonitorSelectStyle,
) {
}

func (o *wlrootsOverlay) DrawMouseActionIndicator(image.Point, ports.MouseActionIndicatorStyle) {}

func (o *wlrootsOverlay) cancelAnimation()               {}
func (o *wlrootsOverlay) setDisplayMu(_ *sync.Mutex)     {}
func (o *wlrootsOverlay) setKeyboardCaptureEnabled(bool) {}
func (o *wlrootsOverlay) startPoller()                   {}
