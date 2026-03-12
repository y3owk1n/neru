//go:build windows

// Package modeindicator provides mode indicator overlay components.
package modeindicator

import (
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

// Overlay manages the rendering of mode indicator overlays using native platform APIs (Windows stub).
type Overlay struct {
	indicatorConfig config.ModeIndicatorConfig
	theme           config.ThemeProvider
	logger          *zap.Logger
	configMu        sync.RWMutex
}

// NewOverlay initializes a new mode indicator overlay instance with its own window (Windows stub).
func NewOverlay(
	indicatorCfg config.ModeIndicatorConfig,
	theme config.ThemeProvider,
	logger *zap.Logger,
) (*Overlay, error) {
	return &Overlay{
		indicatorConfig: indicatorCfg,
		theme:           theme,
		logger:          logger,
	}, nil
}

// DrawModeIndicator draws the mode indicator for the specified mode (Windows stub).
func (o *Overlay) DrawModeIndicator(mode string) error {
	return nil
}

// Show shows the mode indicator overlay (Windows stub).
func (o *Overlay) Show() {}

// Hide hides the mode indicator overlay (Windows stub).
func (o *Overlay) Hide() {}

// Clear clears the mode indicator overlay (Windows stub).
func (o *Overlay) Clear() {}

// ResizeToActiveScreen resizes the mode indicator overlay to the active screen (Windows stub).
func (o *Overlay) ResizeToActiveScreen() {}

// Destroy destroys the mode indicator overlay (Windows stub).
func (o *Overlay) Destroy() {}

// SetConfig updates the indicator configuration (Windows stub).
func (o *Overlay) SetConfig(cfg config.ModeIndicatorConfig) {
	o.configMu.Lock()
	defer o.configMu.Unlock()

	o.indicatorConfig = cfg
}

// SetIndicatorConfig updates the indicator configuration (Windows stub).
func (o *Overlay) SetIndicatorConfig(cfg config.ModeIndicatorConfig) {
	o.SetConfig(cfg)
}

// IndicatorConfig returns the indicator configuration (Windows stub).
func (o *Overlay) IndicatorConfig() config.ModeIndicatorConfig {
	o.configMu.RLock()
	defer o.configMu.RUnlock()

	return o.indicatorConfig
}
