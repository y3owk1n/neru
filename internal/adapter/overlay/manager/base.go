package manager

import (
	"image"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/modeindicator"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/stickyindicator"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/virtualpointer"
	"github.com/y3owk1n/neru/internal/ports"
)

// defaultSubscriberCapacity sizes the subscriber map for the handful of
// long-lived subscribers a session realistically has.
const defaultSubscriberCapacity = 4

// Base carries the state every overlay backend shares: the current mode, the
// mode-change subscriber registry, and the render-component registry. Backends
// embed it and initialize it with NewBase, keeping only genuinely
// platform-specific behavior in their own methods.
//
// A backend may override an individual method to add platform behavior (the
// darwin backend does this for UseVirtualPointerOverlay); the override should
// still delegate to the Base method for the shared bookkeeping.
//
// The registry accessors below are not on Interface: components are built by
// BuildComponents and read by the backend drawing through them, so nothing
// outside this package has a reason to reach one. They stay exported because
// the backends are separate packages.
type Base struct {
	logger *zap.Logger

	mu     sync.RWMutex
	mode   Mode
	subs   map[uint64]func(StateChange)
	nextID uint64

	hintOverlay            *hints.Overlay
	gridOverlay            *grid.Overlay
	modeIndicatorOverlay   *modeindicator.Overlay
	recursiveGridOverlay   *recursivegrid.Overlay
	stickyModifiersOverlay *stickyindicator.Overlay
	virtualPointerOverlay  *virtualpointer.Overlay
}

// NewBase returns a Base ready for embedding. The logger may be nil.
func NewBase(logger *zap.Logger) Base {
	return Base{
		logger: logger,
		mode:   ModeIdle,
		subs:   make(map[uint64]func(StateChange), defaultSubscriberCapacity),
	}
}

// Mode returns the current overlay mode.
func (b *Base) Mode() Mode {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.mode
}

// SwitchTo transitions to the given mode and notifies subscribers. Switching
// to the mode already active is a no-op: subscribers only ever see real
// transitions.
func (b *Base) SwitchTo(next Mode) {
	b.mu.Lock()

	prev := b.mode
	if prev == next {
		b.mu.Unlock()

		return
	}

	b.mode = next

	b.mu.Unlock()

	if b.logger != nil {
		b.logger.Debug(
			"Overlay mode switch",
			zap.String("prev", string(prev)),
			zap.String("next", string(next)),
		)
	}

	b.publish(NewStateChange(prev, next))
}

// Subscribe registers a callback for overlay mode changes.
func (b *Base) Subscribe(subscriber func(StateChange)) uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := b.nextID
	b.subs[id] = subscriber

	return id
}

// Unsubscribe removes a previously registered callback.
func (b *Base) Unsubscribe(id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.subs, id)
}

// UseHintOverlay sets the hints overlay renderer.
func (b *Base) UseHintOverlay(o *hints.Overlay) { b.hintOverlay = o }

// UseGridOverlay sets the grid overlay renderer.
func (b *Base) UseGridOverlay(o *grid.Overlay) { b.gridOverlay = o }

// UseModeIndicatorOverlay sets the mode-indicator overlay renderer.
func (b *Base) UseModeIndicatorOverlay(o *modeindicator.Overlay) { b.modeIndicatorOverlay = o }

// UseStickyModifiersOverlay sets the sticky modifiers overlay renderer.
func (b *Base) UseStickyModifiersOverlay(
	o *stickyindicator.Overlay,
) {
	b.stickyModifiersOverlay = o
}

// UseRecursiveGridOverlay sets the recursive-grid overlay renderer.
func (b *Base) UseRecursiveGridOverlay(o *recursivegrid.Overlay) { b.recursiveGridOverlay = o }

// UseVirtualPointerOverlay sets the cursor-following virtual pointer overlay renderer.
func (b *Base) UseVirtualPointerOverlay(o *virtualpointer.Overlay) { b.virtualPointerOverlay = o }

// HintOverlay returns the hints overlay renderer.
func (b *Base) HintOverlay() *hints.Overlay { return b.hintOverlay }

// GridOverlay returns the grid overlay renderer.
func (b *Base) GridOverlay() *grid.Overlay { return b.gridOverlay }

// ModeIndicatorOverlay returns the mode-indicator overlay renderer.
func (b *Base) ModeIndicatorOverlay() *modeindicator.Overlay { return b.modeIndicatorOverlay }

// StickyModifiersOverlay returns the sticky modifiers overlay renderer.
func (b *Base) StickyModifiersOverlay() *stickyindicator.Overlay { return b.stickyModifiersOverlay }

// RecursiveGridOverlay returns the recursive-grid overlay renderer.
func (b *Base) RecursiveGridOverlay() *recursivegrid.Overlay { return b.recursiveGridOverlay }

// VirtualPointerOverlay returns the cursor-following virtual pointer overlay renderer.
func (b *Base) VirtualPointerOverlay() *virtualpointer.Overlay { return b.virtualPointerOverlay }

// ShowIndicator makes an indicator visible.
func (b *Base) ShowIndicator(indicator ports.Indicator) {
	if surface := b.indicatorSurfaceFor(indicator); surface != nil {
		surface.Show()
	}
}

// HideIndicator clears an indicator's content and hides it. Owning both halves
// of that pair is the point: the order is fixed here once instead of at every
// call site, and content cleared after the window is hidden is content that
// reappears on the next show.
func (b *Base) HideIndicator(indicator ports.Indicator) {
	if surface := b.indicatorSurfaceFor(indicator); surface != nil {
		surface.Clear()
		surface.Hide()
	}
}

// ResizeIndicatorToActiveScreen sizes an indicator to the active display.
func (b *Base) ResizeIndicatorToActiveScreen(indicator ports.Indicator) {
	if surface := b.indicatorSurfaceFor(indicator); surface != nil {
		surface.ResizeToActiveScreen()
	}
}

// DrawGridPointer puts the pointer stand-in on the surface a grid mode draws
// on. A mode names the mode; which render component that is, and whether it
// was ever built, is this package's business.
//
// A render component takes the size and the fill only: the char and the family
// it draws with are the ones ConfigureComponents already handed it. A backend
// that paints the pointer onto its own surface instead reads the rest of the
// appearance out of the argument, which is why it travels whole.
func (b *Base) DrawGridPointer(mode Mode, point image.Point, appearance PointerAppearance) {
	if surface := b.gridPointerSurfaceFor(mode); surface != nil {
		surface.ShowVirtualPointer(point, appearance.FontSize, appearance.FillColor)
	}
}

// HideGridPointer takes that pointer off the surface again.
func (b *Base) HideGridPointer(mode Mode) {
	if surface := b.gridPointerSurfaceFor(mode); surface != nil {
		surface.HideVirtualPointer()
	}
}

// gridPointerSurface is the pointer half the grid and recursive-grid render
// components share, for the same reason indicatorSurface exists: the manager
// answers in modes, not in render types.
type gridPointerSurface interface {
	ShowVirtualPointer(point image.Point, size int, fillColor string)
	HideVirtualPointer()
}

// gridPointerSurfaceFor returns the render component drawing a mode's pointer,
// or nil when that component was never constructed. The nil check is on the
// pointer for the same reason it is in indicatorSurfaceFor.
func (b *Base) gridPointerSurfaceFor(mode Mode) gridPointerSurface {
	switch mode {
	case ModeGrid:
		if b.gridOverlay == nil {
			return nil
		}

		return b.gridOverlay
	case ModeRecursiveGrid:
		if b.recursiveGridOverlay == nil {
			return nil
		}

		return b.recursiveGridOverlay
	case ModeIdle, ModeHints, ModeScroll, ModeMonitorSelect:
		// No other mode draws a pointer of its own.
		return nil
	default:
		return nil
	}
}

// indicatorSurface is the visibility half every indicator's render component
// implements. It exists so the manager can answer the port in terms of an
// indicator rather than making its callers pick a render type.
type indicatorSurface interface {
	Show()
	Hide()
	Clear()
	ResizeToActiveScreen()
}

// indicatorSurfaceFor returns the render component behind an indicator, or nil
// when that component was never constructed — a disabled indicator, or a
// headless backend with nothing to draw on.
//
// The nil check is on the pointer, not on the returned interface: a nil
// *Overlay stored in an interface is a live interface value that passes every
// `!= nil` guard a caller could write.
func (b *Base) indicatorSurfaceFor(indicator ports.Indicator) indicatorSurface {
	switch indicator {
	case ports.ModeIndicator:
		if b.modeIndicatorOverlay == nil {
			return nil
		}

		return b.modeIndicatorOverlay
	case ports.StickyModifiersIndicator:
		if b.stickyModifiersOverlay == nil {
			return nil
		}

		return b.stickyModifiersOverlay
	case ports.VirtualPointerIndicator:
		if b.virtualPointerOverlay == nil {
			return nil
		}

		return b.virtualPointerOverlay
	default:
		return nil
	}
}

// publish delivers a state change to every subscriber. Callbacks run outside
// the lock so a subscriber may call back into the manager.
func (b *Base) publish(event StateChange) {
	b.mu.Lock()

	subs := make([]func(StateChange), 0, len(b.subs))
	for _, sub := range b.subs {
		subs = append(subs, sub)
	}
	b.mu.Unlock()

	for _, sub := range subs {
		sub(event)
	}
}
