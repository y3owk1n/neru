//go:build linux

// Package stickyindicator provides sticky modifiers indicator overlay components.
package stickyindicator

import (
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

// Overlay manages the rendering of sticky modifiers indicator overlay (Linux stub).
type Overlay struct {
	uiConfig config.StickyModifiersUI
	theme    config.ThemeProvider
	logger   *zap.Logger
	configMu sync.RWMutex
}

// NewOverlay initializes a new sticky modifiers indicator overlay (Linux stub).
func NewOverlay(
	uiConfig config.StickyModifiersUI,
	theme config.ThemeProvider,
	logger *zap.Logger,
) (*Overlay, error) {
	return &Overlay{
		uiConfig: uiConfig,
		theme:    theme,
		logger:   logger,
	}, nil
}

// Draw draws the sticky modifiers indicator at the specified position (Linux stub).
func (o *Overlay) Draw(x, y int, symbols string) {}

// Show shows the sticky modifiers indicator overlay (Linux stub).
func (o *Overlay) Show() {}

// Hide hides the sticky modifiers indicator overlay (Linux stub).
func (o *Overlay) Hide() {}

// Clear clears the sticky modifiers indicator overlay (Linux stub).
func (o *Overlay) Clear() {}

// ResizeToActiveScreen resizes the sticky modifiers indicator overlay to the active screen (Linux stub).
func (o *Overlay) ResizeToActiveScreen() {}

// Destroy destroys the sticky modifiers indicator overlay (Linux stub).
func (o *Overlay) Destroy() {}

// Cleanup frees Go-side resources (Linux stub).
func (o *Overlay) Cleanup() {}

// SetSharingType sets the window sharing type for screen sharing visibility (Linux stub).
func (o *Overlay) SetSharingType(_ bool) {}

// SetConfig updates the overlay configuration (Linux stub).
func (o *Overlay) SetConfig(cfg config.StickyModifiersUI) {
	o.configMu.Lock()
	defer o.configMu.Unlock()

	o.uiConfig = cfg
}
