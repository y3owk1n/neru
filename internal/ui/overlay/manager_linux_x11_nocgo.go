//go:build linux && !cgo

package overlay

import (
	"image"
	"unsafe"

	"go.uber.org/zap"

	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
)

type x11Overlay struct{}

func newX11Overlay(logger *zap.Logger) *x11Overlay {
	_ = logger
	return nil
}

func (o *x11Overlay) Healthy() bool                                          { return false }
func (o *x11Overlay) WindowPtr() unsafe.Pointer                              { return nil }
func (o *x11Overlay) Show()                                                  {}
func (o *x11Overlay) Hide()                                                  {}
func (o *x11Overlay) Clear()                                                 {}
func (o *x11Overlay) Resize()                                                {}
func (o *x11Overlay) Destroy()                                               {}
func (o *x11Overlay) UpdateGridMatches(string)                               {}
func (o *x11Overlay) ShowSubgrid(*domainGrid.Cell, gridcomponent.Style)      {}
func (o *x11Overlay) SetHideUnmatched(bool)                                  {}
func (o *x11Overlay) DrawGrid(*domainGrid.Grid, string, gridcomponent.Style) {}
func (o *x11Overlay) DrawHints([]*hintscomponent.Hint, hintscomponent.StyleMode) {
}

func (o *x11Overlay) DrawRecursiveGrid(
	image.Rectangle,
	int,
	string,
	int,
	int,
	int,
	recursivegridcomponent.Style,
	recursivegridcomponent.VirtualPointerState,
) {
}
func (o *x11Overlay) DrawBadge(int, int, string, overlayColors, overlayBadgeStyle) {}
