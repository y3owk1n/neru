package overlay

/*
#cgo CFLAGS: -x objective-c
#include "../../core/infra/bridge/overlay.h"
*/
import "C"

import (
	"sync"
	"unsafe"

	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"go.uber.org/zap"
)

const (
	// DefaultSubscriberMapSize is the default size for subscriber map.
	DefaultSubscriberMapSize = 4
)

// NoOpManager is a no-op implementation of ManagerInterface for headless environments.
type NoOpManager struct{}

// Ensure NoOpManager always implements ManagerInterface.
var _ ManagerInterface = (*NoOpManager)(nil)

// Show is a no-op implementation.
func (n *NoOpManager) Show() {}

// Hide is a no-op implementation.
func (n *NoOpManager) Hide() {}

// Clear is a no-op implementation.
func (n *NoOpManager) Clear() {}

// ResizeToActiveScreen is a no-op implementation.
func (n *NoOpManager) ResizeToActiveScreen() {}

// SwitchTo is a no-op implementation.
func (n *NoOpManager) SwitchTo(next Mode) {}

// Subscribe is a no-op implementation.
func (n *NoOpManager) Subscribe(fn func(StateChange)) uint64 { return 0 }

// Unsubscribe is a no-op implementation.
func (n *NoOpManager) Unsubscribe(id uint64) {}

// Destroy is a no-op implementation.
func (n *NoOpManager) Destroy() {}

// Mode returns ModeIdle.
func (n *NoOpManager) Mode() Mode { return ModeIdle }

// WindowPtr returns nil.
func (n *NoOpManager) WindowPtr() unsafe.Pointer { return nil }

// UseHintOverlay is a no-op implementation.
func (n *NoOpManager) UseHintOverlay(o *hints.Overlay) {}

// UseGridOverlay is a no-op implementation.
func (n *NoOpManager) UseGridOverlay(o *grid.Overlay) {}

// UseScrollOverlay is a no-op implementation.
func (n *NoOpManager) UseScrollOverlay(o *scroll.Overlay) {}

// HintOverlay returns nil.
func (n *NoOpManager) HintOverlay() *hints.Overlay { return nil }

// GridOverlay returns nil.
func (n *NoOpManager) GridOverlay() *grid.Overlay { return nil }

// ScrollOverlay returns nil.
func (n *NoOpManager) ScrollOverlay() *scroll.Overlay { return nil }

// DrawHintsWithStyle is a no-op implementation.
func (n *NoOpManager) DrawHintsWithStyle(
	hs []*hints.Hint,
	style hints.StyleMode,
) error {
	return nil
}

// DrawScrollHighlight is a no-op implementation.
func (n *NoOpManager) DrawScrollHighlight(x, y, w, h int) {}

// DrawGrid is a no-op implementation.
func (n *NoOpManager) DrawGrid(
	g *domainGrid.Grid,
	input string,
	style grid.Style,
) error {
	return nil
}

// UpdateGridMatches is a no-op implementation.
func (n *NoOpManager) UpdateGridMatches(prefix string) {}

// ShowSubgrid is a no-op implementation.
func (n *NoOpManager) ShowSubgrid(cell *domainGrid.Cell, style grid.Style) {}

// SetHideUnmatched is a no-op implementation.
func (n *NoOpManager) SetHideUnmatched(hide bool) {}

// Mode represents the overlay mode.
type Mode string

const (
	// ModeIdle represents the idle mode.
	ModeIdle Mode = "idle"
	// ModeHints represents the hints mode.
	ModeHints Mode = "hints"
	// ModeGrid represents the grid mode.
	ModeGrid Mode = "grid"
	// ModeScroll represents the scroll mode.
	ModeScroll Mode = "scroll"
)

// StateChange represents a change in overlay mode.
type StateChange struct {
	prev Mode
	next Mode
}

// Prev returns the previous mode.
func (sc StateChange) Prev() Mode {
	return sc.prev
}

// Next returns the next mode.
func (sc StateChange) Next() Mode {
	return sc.next
}

// ManagerInterface defines the interface for overlay window management.
type ManagerInterface interface {
	Show()
	Hide()
	Clear()
	ResizeToActiveScreen()
	SwitchTo(next Mode)
	Subscribe(fn func(StateChange)) uint64
	Unsubscribe(id uint64)
	Destroy()
	Mode() Mode
	WindowPtr() unsafe.Pointer

	UseHintOverlay(o *hints.Overlay)
	UseGridOverlay(o *grid.Overlay)
	UseScrollOverlay(o *scroll.Overlay)

	HintOverlay() *hints.Overlay
	GridOverlay() *grid.Overlay
	ScrollOverlay() *scroll.Overlay

	DrawHintsWithStyle(hs []*hints.Hint, style hints.StyleMode) error
	DrawScrollHighlight(x, y, w, h int)
	DrawGrid(g *domainGrid.Grid, input string, style grid.Style) error
	UpdateGridMatches(prefix string)
	ShowSubgrid(cell *domainGrid.Cell, style grid.Style)
	SetHideUnmatched(hide bool)
}

// Manager coordinates overlay window management and mode transitions for all overlay types.
type Manager struct {
	window C.OverlayWindow
	logger *zap.Logger
	mu     sync.Mutex
	mode   Mode
	subs   map[uint64]func(StateChange)
	nextID uint64

	// Overlay renderers
	hintOverlay   *hints.Overlay
	gridOverlay   *grid.Overlay
	scrollOverlay *scroll.Overlay
}

var (
	manager *Manager
	once    sync.Once
)

// Init initializes the singleton overlay manager with a new overlay window.
func Init(logger *zap.Logger) *Manager {
	once.Do(func() {
		window := C.createOverlayWindow()
		manager = &Manager{
			window: window,
			logger: logger,
			mode:   ModeIdle,
			subs: make(
				map[uint64]func(StateChange),
				DefaultSubscriberMapSize,
			),
		}
	})

	return manager
}

// Get returns the singleton instance of the overlay manager.
func Get() *Manager {
	return manager
}

// WindowPtr returns the window pointer.
func (m *Manager) WindowPtr() unsafe.Pointer {
	return unsafe.Pointer(m.window)
}

// Mode returns the current overlay mode.
func (m *Manager) Mode() Mode {
	return m.mode
}

// Logger returns the logger.
func (m *Manager) Logger() *zap.Logger {
	return m.logger
}

// Show shows the overlay window.
func (m *Manager) Show() {
	C.NeruShowOverlayWindow(m.window)
}

// Hide hides the overlay window.
func (m *Manager) Hide() {
	C.NeruHideOverlayWindow(m.window)
}

// Clear clears the overlay window.
func (m *Manager) Clear() {
	C.NeruClearOverlay(m.window)
	if m.gridOverlay != nil {
		m.gridOverlay.Clear()
	}
	if m.hintOverlay != nil {
		m.hintOverlay.Clear()
	}
}

// ResizeToActiveScreen resizes the overlay window to the active screen.
func (m *Manager) ResizeToActiveScreen() {
	C.NeruResizeOverlayToActiveScreen(m.window)
}

// SwitchTo transitions the overlay to the specified mode and notifies subscribers.
func (m *Manager) SwitchTo(next Mode) {
	m.mu.Lock()
	prev := m.mode
	m.mode = next
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Debug(
			"Overlay mode switch",
			zap.String("prev", string(prev)),
			zap.String("next", string(next)),
		)
	}
	m.publish(StateChange{prev: prev, next: next})
}

// Subscribe registers a callback function to be notified of overlay mode changes.
func (m *Manager) Subscribe(fn func(StateChange)) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	id := m.nextID
	m.subs[id] = fn

	return id
}

// Unsubscribe removes a previously registered callback function.
func (m *Manager) Unsubscribe(id uint64) {
	m.mu.Lock()
	delete(m.subs, id)
	m.mu.Unlock()
}

// Destroy destroys the overlay window.
func (m *Manager) Destroy() {
	if m.window != nil {
		C.NeruDestroyOverlayWindow(m.window)
		m.window = nil
	}
}

// UseHintOverlay sets the hint overlay renderer for centralized management.
func (m *Manager) UseHintOverlay(o *hints.Overlay) {
	m.hintOverlay = o
}

// UseGridOverlay sets the grid overlay renderer.
func (m *Manager) UseGridOverlay(o *grid.Overlay) {
	m.gridOverlay = o
}

// UseScrollOverlay sets the scroll overlay renderer.
func (m *Manager) UseScrollOverlay(o *scroll.Overlay) {
	m.scrollOverlay = o
}

// HintOverlay returns the hint overlay renderer.
func (m *Manager) HintOverlay() *hints.Overlay {
	return m.hintOverlay
}

// GridOverlay returns the grid overlay renderer.
func (m *Manager) GridOverlay() *grid.Overlay {
	return m.gridOverlay
}

// ScrollOverlay returns the scroll overlay renderer.
func (m *Manager) ScrollOverlay() *scroll.Overlay {
	return m.scrollOverlay
}

// DrawHintsWithStyle draws hints with the specified style using the hint overlay renderer.
func (m *Manager) DrawHintsWithStyle(hs []*hints.Hint, style hints.StyleMode) error {
	if m.hintOverlay == nil {
		return nil
	}
	drawHintsErr := m.hintOverlay.DrawHintsWithStyle(hs, style)
	if drawHintsErr != nil {
		return derrors.Wrap(
			drawHintsErr,
			derrors.CodeOverlayFailed,
			"failed to draw hints with style",
		)
	}

	return nil
}

// DrawScrollHighlight renders a scroll highlight border using the scroll overlay renderer.
func (m *Manager) DrawScrollHighlight(x, y, w, h int) {
	if m.scrollOverlay == nil {
		return
	}
	m.scrollOverlay.DrawScrollHighlight(x, y, w, h)
}

// DrawGrid renders a grid with the specified style using the grid overlay renderer.
func (m *Manager) DrawGrid(g *domainGrid.Grid, input string, style grid.Style) error {
	if m.gridOverlay == nil {
		return nil
	}
	drawGridErr := m.gridOverlay.DrawGrid(g, input, style)
	if drawGridErr != nil {
		return derrors.Wrap(drawGridErr, derrors.CodeOverlayFailed, "failed to draw grid")
	}

	return nil
}

// UpdateGridMatches updates the grid matches with the specified prefix.
func (m *Manager) UpdateGridMatches(prefix string) {
	if m.gridOverlay == nil {
		return
	}
	m.gridOverlay.UpdateMatches(prefix)
}

// ShowSubgrid shows a subgrid for the specified cell.
func (m *Manager) ShowSubgrid(cell *domainGrid.Cell, style grid.Style) {
	if m.gridOverlay == nil {
		return
	}
	m.gridOverlay.ShowSubgrid(cell, style)
}

// SetHideUnmatched sets whether to hide unmatched cells.
func (m *Manager) SetHideUnmatched(hide bool) {
	if m.gridOverlay == nil {
		return
	}
	m.gridOverlay.SetHideUnmatched(hide)
}

// publish publishes a state change to all subscribers.
func (m *Manager) publish(event StateChange) {
	m.mu.Lock()
	subs := make([]func(StateChange), len(m.subs))
	index := 0
	for _, sub := range m.subs {
		subs[index] = sub
		index++
	}
	m.mu.Unlock()
	for _, sub := range subs {
		sub(event)
	}
}
