//go:build linux && !cgo

package linux

import (
	"image"
	"sync"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

// x11Overlay is the no-cgo stand-in. Unlike the cgo build, both twins keep
// their methods per-backend on purpose. Every body is empty, and their
// constructors always return nil, so these bodies exist to be reached on a
// nil receiver, which a promoted method cannot be (../AGENTS.md).
type x11Overlay struct {
	sublayerKeys string
}

func newX11Overlay(logger *zap.Logger) *x11Overlay {
	_ = logger

	return nil
}

func (o *x11Overlay) Healthy() bool                                          { return false }
func (o *x11Overlay) WindowPtr() unsafe.Pointer                              { return nil }
func (o *x11Overlay) Show()                                                  {}
func (o *x11Overlay) Hide()                                                  {}
func (o *x11Overlay) Clear()                                                 {}
func (o *x11Overlay) ClearRect(image.Rectangle)                              {}
func (o *x11Overlay) Resize()                                                {}
func (o *x11Overlay) Destroy()                                               {}
func (o *x11Overlay) UpdateGridMatches(string)                               {}
func (o *x11Overlay) SetHideUnmatched(bool)                                  {}
func (o *x11Overlay) setOriginOffset(image.Point)                            {}
func (o *x11Overlay) DrawGrid(*domainGrid.Grid, string, gridcomponent.Style) {}
func (o *x11Overlay) forgetGridPointer()                                     {}

func (o *x11Overlay) ShowSubgrid(
	*domainGrid.Cell,
	gridcomponent.Style,
	recursivegridcomponent.VirtualPointerState,
) {
}

func (o *x11Overlay) SetGridPointer(recursivegridcomponent.VirtualPointerState) {}

func (o *x11Overlay) DrawHints([]*hintscomponent.Hint, hintscomponent.StyleMode, badge.HintOffset) {
}

func (o *x11Overlay) DrawRecursiveGridWithSubKeyPreview(
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
func (o *x11Overlay) Flush() {}

func (o *x11Overlay) DrawBadge(int, int, string, overlayColors, overlayBadgeStyle) {}

func (o *x11Overlay) DrawHintSearchInput(
	string,
	hintscomponent.SearchInputFrame,
	hintscomponent.SearchInputStyle,
) image.Rectangle {
	return image.Rectangle{}
}

func (o *x11Overlay) HideHintSearchInput(image.Rectangle) {}

func (o *x11Overlay) Scale() float64 { return 1 }

func (o *x11Overlay) DrawMonitorSelect([]manager.MonitorSelectTarget, manager.MonitorSelectStyle) {}

func (o *x11Overlay) DrawMouseActionIndicator(image.Point, ports.MouseActionIndicatorStyle) {}

func (o *x11Overlay) cancelAnimation()          {}
func (o *x11Overlay) setRenderMu(_ *sync.Mutex) {}

// withBlanked runs capture as it is, because this build draws no overlay.
func (o *x11Overlay) withBlanked(capture func() error) error { return capture() }
