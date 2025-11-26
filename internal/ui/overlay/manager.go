package overlay

/*
#cgo CFLAGS: -x objective-c
#include "../../infra/bridge/overlay.h"
*/
import "C"

import (
	"sync"
	"unsafe"

	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	derrors "github.com/y3owk1n/neru/internal/errors"
	"github.com/y3owk1n/neru/internal/features/action"
	"github.com/y3owk1n/neru/internal/features/grid"
	"github.com/y3owk1n/neru/internal/features/hints"
	"github.com/y3owk1n/neru/internal/features/scroll"
	"go.uber.org/zap"
)

const (
	// DefaultSubscriberMapSize is the default size for subscriber map.
	DefaultSubscriberMapSize = 4
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
// ManagerInterface defines the interface for the overlay manager.
//
//nolint:interfacebloat
type ManagerInterface interface {
	Show()
	Hide()
	Clear()
	ResizeToActiveScreenSync()
	SwitchTo(next Mode)
	Subscribe(fn func(StateChange)) uint64
	Unsubscribe(id uint64)
	Destroy()
	Mode() Mode
	WindowPtr() unsafe.Pointer

	UseHintOverlay(o *hints.Overlay)
	UseGridOverlay(o *grid.Overlay)
	UseActionOverlay(o *action.Overlay)
	UseScrollOverlay(o *scroll.Overlay)

	HintOverlay() *hints.Overlay
	GridOverlay() *grid.Overlay
	ActionOverlay() *action.Overlay
	ScrollOverlay() *scroll.Overlay

	DrawHintsWithStyle(hs []*hints.Hint, style hints.StyleMode) error
	DrawActionHighlight(x, y, w, h int)
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
	actionOverlay *action.Overlay
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
			), // Pre-size for typical subscriber count
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
}

// ResizeToActiveScreenSync resizes the overlay window to the active screen synchronously.
func (m *Manager) ResizeToActiveScreenSync() {
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

// UseActionOverlay sets the action overlay renderer.
func (m *Manager) UseActionOverlay(o *action.Overlay) {
	m.actionOverlay = o
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

// ActionOverlay returns the action overlay renderer.
func (m *Manager) ActionOverlay() *action.Overlay {
	return m.actionOverlay
}

// ScrollOverlay returns the scroll overlay renderer.
func (m *Manager) ScrollOverlay() *scroll.Overlay {
	return m.scrollOverlay
}

// DrawHintsWithStyle draws hints with the specified style.
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

// DrawActionHighlight renders an action highlight border using the action overlay renderer.
func (m *Manager) DrawActionHighlight(x, y, w, h int) {
	if m.actionOverlay == nil {
		return
	}
	m.actionOverlay.DrawActionHighlight(x, y, w, h)
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
	drawGridErr := m.gridOverlay.Draw(g, input, style)
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
	// Pre-allocate with exact capacity
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
