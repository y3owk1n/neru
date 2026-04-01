//go:build darwin

package overlay

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#include "../../core/infra/platform/darwin/overlay.h"
*/
import "C"

import (
	"image"
	"sync"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/modeindicator"
	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/components/stickyindicator"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
)

const (
	// DefaultSubscriberMapSize is the default size for subscriber map.
	DefaultSubscriberMapSize = 4

	// NSWindowSharingNone represents NSWindowSharingNone (0) - hidden from screen sharing.
	NSWindowSharingNone = 0
	// NSWindowSharingReadOnly represents NSWindowSharingReadOnly (1) - visible in screen sharing.
	NSWindowSharingReadOnly = 1
)

// Manager manages multiple overlay windows.
type Manager struct {
	window C.OverlayWindow
	logger *zap.Logger
	mu     sync.RWMutex
	mode   Mode
	subs   map[uint64]func(StateChange)
	nextID uint64

	// Overlay renderers
	hintOverlay            *hints.Overlay
	gridOverlay            *grid.Overlay
	modeIndicatorOverlay   *modeindicator.Overlay
	recursiveGridOverlay   *recursivegrid.Overlay
	stickyModifiersOverlay *stickyindicator.Overlay
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
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.mode
}

// Logger returns the logger.
func (m *Manager) Logger() *zap.Logger {
	return m.logger
}

// OverlayCapabilities reports current darwin overlay support.
func (m *Manager) OverlayCapabilities() ports.FeatureCapability {
	return ports.FeatureCapability{
		Status: ports.FeatureStatusSupported,
		Detail: "native darwin overlays are available",
	}
}

// Show shows the overlay window.
func (m *Manager) Show() {
	C.NeruShowOverlayWindow(m.window)
}

// Hide hides the overlay window.
func (m *Manager) Hide() {
	C.NeruHideOverlayWindow(m.window)

	if m.modeIndicatorOverlay != nil {
		m.modeIndicatorOverlay.Hide()
	}

	if m.stickyModifiersOverlay != nil {
		m.stickyModifiersOverlay.Hide()
	}

	if m.recursiveGridOverlay != nil {
		m.recursiveGridOverlay.Hide()
	}
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

	if m.modeIndicatorOverlay != nil {
		m.modeIndicatorOverlay.Clear()
	}

	if m.stickyModifiersOverlay != nil {
		m.stickyModifiersOverlay.Clear()
	}

	if m.recursiveGridOverlay != nil {
		m.recursiveGridOverlay.Clear()
	}
}

// ResizeToActiveScreen resizes the overlay window to the active screen.
func (m *Manager) ResizeToActiveScreen() {
	C.NeruResizeOverlayToActiveScreen(m.window)

	if m.modeIndicatorOverlay != nil {
		m.modeIndicatorOverlay.ResizeToActiveScreen()
	}

	if m.stickyModifiersOverlay != nil {
		m.stickyModifiersOverlay.ResizeToActiveScreen()
	}
}

// SwitchTo transitions the overlay to the specified mode and notifies subscribers.
func (m *Manager) SwitchTo(next Mode) {
	m.mu.Lock()
	prev := m.mode
	if prev == next {
		m.mu.Unlock()

		return
	}
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

// Destroy destroys the overlay window and cleans up all overlay resources.
func (m *Manager) Destroy() {
	// Clean up Go-side resources (callbackManager, styleCache, labelCache) for
	// overlays that share the manager's window. We call Cleanup() instead of
	// Destroy() because the shared window is destroyed below — calling each
	// overlay's Destroy() would double-destroy the same native window.
	if m.hintOverlay != nil {
		m.hintOverlay.Cleanup()
		m.hintOverlay = nil
	}
	if m.gridOverlay != nil {
		m.gridOverlay.Cleanup()
		m.gridOverlay = nil
	}
	if m.recursiveGridOverlay != nil {
		m.recursiveGridOverlay.Cleanup()
		m.recursiveGridOverlay = nil
	}

	// Mode indicator owns its own window, so use full Destroy().
	if m.modeIndicatorOverlay != nil {
		m.modeIndicatorOverlay.Destroy()
		m.modeIndicatorOverlay = nil
	}

	// Sticky modifiers indicator owns its own window, so use full Destroy().
	if m.stickyModifiersOverlay != nil {
		m.stickyModifiersOverlay.Destroy()
		m.stickyModifiersOverlay = nil
	}

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

// UseModeIndicatorOverlay sets the shared mode-indicator overlay renderer.
func (m *Manager) UseModeIndicatorOverlay(o *modeindicator.Overlay) {
	m.modeIndicatorOverlay = o
}

// UseStickyModifiersOverlay sets the sticky modifiers overlay renderer.
func (m *Manager) UseStickyModifiersOverlay(o *stickyindicator.Overlay) {
	m.stickyModifiersOverlay = o
}

// UseRecursiveGridOverlay sets the recursive-grid overlay renderer.
func (m *Manager) UseRecursiveGridOverlay(o *recursivegrid.Overlay) {
	m.recursiveGridOverlay = o
}

// HintOverlay returns the hint overlay renderer.
func (m *Manager) HintOverlay() *hints.Overlay {
	return m.hintOverlay
}

// GridOverlay returns the grid overlay renderer.
func (m *Manager) GridOverlay() *grid.Overlay {
	return m.gridOverlay
}

// ModeIndicatorOverlay returns the mode-indicator overlay renderer.
func (m *Manager) ModeIndicatorOverlay() *modeindicator.Overlay {
	return m.modeIndicatorOverlay
}

// StickyModifiersOverlay returns the sticky modifiers overlay renderer.
func (m *Manager) StickyModifiersOverlay() *stickyindicator.Overlay {
	return m.stickyModifiersOverlay
}

// RecursiveGridOverlay returns the recursive-grid overlay renderer.
func (m *Manager) RecursiveGridOverlay() *recursivegrid.Overlay {
	return m.recursiveGridOverlay
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

// DrawModeIndicator renders a mode indicator using the shared overlay renderer.
func (m *Manager) DrawModeIndicator(xCoordinate, yCoordinate int) {
	if m.modeIndicatorOverlay == nil {
		return
	}

	mode := m.Mode()
	if mode == ModeIdle {
		return
	}

	m.modeIndicatorOverlay.DrawModeIndicator(string(mode), xCoordinate, yCoordinate)
}

// DrawStickyModifiersIndicator renders the sticky modifiers indicator using the sticky modifiers overlay renderer.
func (m *Manager) DrawStickyModifiersIndicator(xCoordinate, yCoordinate int, symbols string) {
	if m.stickyModifiersOverlay == nil {
		return
	}

	mode := m.Mode()
	if mode == ModeIdle {
		return
	}

	m.stickyModifiersOverlay.Draw(xCoordinate, yCoordinate, symbols)
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

// DrawRecursiveGrid renders a recursive-grid with the specified style using the recursive-grid overlay renderer.
func (m *Manager) DrawRecursiveGrid(
	bounds image.Rectangle,
	depth int,
	keys string,
	gridCols int,
	gridRows int,
	nextKeys string,
	nextGridCols int,
	nextGridRows int,
	style recursivegrid.Style,
	virtualPointer recursivegrid.VirtualPointerState,
) error {
	if m.recursiveGridOverlay == nil {
		return nil
	}
	drawRecursiveGridErr := m.recursiveGridOverlay.DrawRecursiveGrid(
		bounds,
		depth,
		keys,
		gridCols,
		gridRows,
		nextKeys,
		nextGridCols,
		nextGridRows,
		style,
		virtualPointer,
	)
	if drawRecursiveGridErr != nil {
		return derrors.Wrap(
			drawRecursiveGridErr,
			derrors.CodeOverlayFailed,
			"failed to draw recursive-grid",
		)
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

// SetSharingType sets the window sharing type for screen sharing visibility.
// When hide is true, sets NSWindowSharingNone (hidden from screen share).
// When hide is false, sets NSWindowSharingReadOnly (visible in screen share).
//
// Note: This method holds m.mu during the CGo call to C.NeruSetOverlaySharingType.
// The C function uses dispatch_async (returns immediately), so this is safe.
// If the C function were changed to dispatch_sync, this could deadlock with
// main thread callers since SwitchTo/Subscribe/Unsubscribe also use m.mu.
func (m *Manager) SetSharingType(hide bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sharingType := C.int(NSWindowSharingReadOnly)
	if hide {
		sharingType = C.int(NSWindowSharingNone)
	}

	C.NeruSetOverlaySharingType(m.window, sharingType)

	// Also update grid, recursive_grid, and mode indicator overlay windows if they exist
	if m.gridOverlay != nil {
		m.gridOverlay.SetSharingType(hide)
	}
	if m.recursiveGridOverlay != nil {
		m.recursiveGridOverlay.SetSharingType(hide)
	}
	if m.modeIndicatorOverlay != nil {
		m.modeIndicatorOverlay.SetSharingType(hide)
	}

	if m.stickyModifiersOverlay != nil {
		m.stickyModifiersOverlay.SetSharingType(hide)
	}

	if m.logger != nil {
		m.logger.Info("Overlay screen share visibility toggled",
			zap.Bool("hidden", hide))
	}
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
