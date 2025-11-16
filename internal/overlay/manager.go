// Package overlay provides overlay functionality for the Neru application.
package overlay

/*
#cgo CFLAGS: -x objective-c
#include "../bridge/overlay.h"
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/y3owk1n/neru/internal/action"
	"github.com/y3owk1n/neru/internal/grid"
	"github.com/y3owk1n/neru/internal/hints"
	"github.com/y3owk1n/neru/internal/scroll"
	"go.uber.org/zap"
)

// Mode represents the overlay mode.
type Mode string

const (
	// ModeIdle represents the idle mode.
	ModeIdle Mode = "idle"
	// ModeHints represents the hints mode.
	ModeHints Mode = "hints"
	// ModeGrid represents the grid mode.
	ModeGrid Mode = "grid"
	// ModeAction represents the action mode.
	ModeAction Mode = "action"
	// ModeScroll represents the scroll mode.
	ModeScroll Mode = "scroll"
)

// StateChange represents a change in overlay mode.
type StateChange struct {
	Prev Mode
	Next Mode
}

// Manager manages the overlay window and its state.
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
	actionOverlay *action.Overlay
	scrollOverlay *scroll.Overlay
}

var (
	mgr  *Manager
	once sync.Once
)

// Init initializes the overlay manager.
func Init(logger *zap.Logger) *Manager {
	once.Do(func() {
		w := C.createOverlayWindow()
		mgr = &Manager{
			window: w,
			logger: logger,
			mode:   ModeIdle,
			subs:   make(map[uint64]func(StateChange)),
		}
	})
	return mgr
}

// Get returns the singleton overlay manager.
func Get() *Manager {
	return mgr
}

// GetWindowPtr returns the window pointer.
func (m *Manager) GetWindowPtr() unsafe.Pointer { return unsafe.Pointer(m.window) }

// Show shows the overlay window.
func (m *Manager) Show() { C.NeruShowOverlayWindow(m.window) }

// Hide hides the overlay window.
func (m *Manager) Hide() { C.NeruHideOverlayWindow(m.window) }

// Clear clears the overlay window.
func (m *Manager) Clear() { C.NeruClearOverlay(m.window) }

// ResizeToActiveScreenSync resizes the overlay window to the active screen synchronously.
func (m *Manager) ResizeToActiveScreenSync() { C.NeruResizeOverlayToActiveScreen(m.window) }

// SwitchTo switches the overlay to the specified mode.
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
	m.publish(StateChange{Prev: prev, Next: next})
}

// Subscribe registers a callback function for state changes.
func (m *Manager) Subscribe(fn func(StateChange)) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	id := m.nextID
	m.subs[id] = fn
	return id
}

// Unsubscribe removes a callback function for state changes.
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

// UseHintOverlay sets the hint overlay renderer.
// Wiring overlay renderers.
func (m *Manager) UseHintOverlay(o *hints.Overlay) { m.hintOverlay = o }

// UseGridOverlay sets the grid overlay renderer.
func (m *Manager) UseGridOverlay(o *grid.Overlay) { m.gridOverlay = o }

// UseActionOverlay sets the action overlay renderer.
func (m *Manager) UseActionOverlay(o *action.Overlay) { m.actionOverlay = o }

// UseScrollOverlay sets the scroll overlay renderer.
func (m *Manager) UseScrollOverlay(o *scroll.Overlay) { m.scrollOverlay = o }

// DrawHintsWithStyle draws hints with the specified style.
// Centralized draw methods.
func (m *Manager) DrawHintsWithStyle(hs []*hints.Hint, style hints.StyleMode) error {
	if m.hintOverlay == nil {
		return nil
	}
	err := m.hintOverlay.DrawHintsWithStyle(hs, style)
	if err != nil {
		return fmt.Errorf("failed to draw hints with style: %w", err)
	}
	return nil
}

// DrawActionHighlight draws an action highlight.
func (m *Manager) DrawActionHighlight(x, y, w, h int) {
	if m.actionOverlay == nil {
		return
	}
	m.actionOverlay.DrawActionHighlight(x, y, w, h)
}

// DrawScrollHighlight draws a scroll highlight.
func (m *Manager) DrawScrollHighlight(x, y, w, h int) {
	if m.scrollOverlay == nil {
		return
	}
	m.scrollOverlay.DrawScrollHighlight(x, y, w, h)
}

// DrawGrid draws a grid with the specified style.
func (m *Manager) DrawGrid(g *grid.Grid, input string, style grid.Style) error {
	if m.gridOverlay == nil {
		return nil
	}
	err := m.gridOverlay.Draw(g, input, style)
	if err != nil {
		return fmt.Errorf("failed to draw grid: %w", err)
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
func (m *Manager) ShowSubgrid(cell *grid.Cell, style grid.Style) {
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
	subs := make([]func(StateChange), 0, len(m.subs))
	for _, sub := range m.subs {
		subs = append(subs, sub)
	}
	m.mu.Unlock()
	for _, sub := range subs {
		sub(event)
	}
}
