//go:build windows

// Package stickyindicator provides sticky modifiers indicator overlay components.
package stickyindicator

import (
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

// Overlay manages the rendering of sticky modifiers indicator overlay.
type Overlay struct {
	uiConfig config.StickyModifiersUI
	theme    config.ThemeProvider
	logger   *zap.Logger
	configMu sync.RWMutex
}

// NewOverlay initializes a new sticky modifiers indicator overlay.
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

// Draw is unused on Windows; drawing is handled by the manager.
func (o *Overlay) Draw(x, y int, symbols string) {}

// Show is unused on Windows; the indicator is drawn on the shared overlay.
func (o *Overlay) Show() {}

// Hide is unused on Windows; the indicator is drawn on the shared overlay.
func (o *Overlay) Hide() {}

// Clear is unused on Windows; the indicator is drawn on the shared overlay.
func (o *Overlay) Clear() {}

// ResizeToActiveScreen is unused on Windows; the indicator is drawn on the shared overlay.
func (o *Overlay) ResizeToActiveScreen() {}

// Destroy is unused on Windows; the indicator is drawn on the shared overlay.
func (o *Overlay) Destroy() {}

// Cleanup is unused on Windows.
func (o *Overlay) Cleanup() {}

// SetSharingType is unused on Windows.
func (o *Overlay) SetSharingType(_ bool) {}

// SetConfig updates the overlay configuration.
func (o *Overlay) SetConfig(cfg config.StickyModifiersUI) {
	o.configMu.Lock()
	defer o.configMu.Unlock()

	o.uiConfig = cfg
}

// UI returns the indicator UI config.
func (o *Overlay) UI() config.StickyModifiersUI {
	o.configMu.RLock()
	defer o.configMu.RUnlock()

	return o.uiConfig
}

// Theme returns the theme provider.
func (o *Overlay) Theme() config.ThemeProvider {
	return o.theme
}
