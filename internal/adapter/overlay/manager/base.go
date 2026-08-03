package manager

import (
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/modeindicator"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/stickyindicator"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/virtualpointer"
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
