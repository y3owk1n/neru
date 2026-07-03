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

// DrawModeIndicator is unused on Windows; drawing is handled by the manager.
func (o *Overlay) DrawModeIndicator(mode string) error {
	return nil
}

// Show is unused on Windows; the indicator is drawn on the shared overlay.
func (o *Overlay) Show() {}

// Hide is unused on Windows; the indicator is drawn on the shared overlay.
func (o *Overlay) Hide() {}

// Clear is unused on Windows; the indicator is drawn on the shared overlay.
func (o *Overlay) Clear() {}

// ResizeToActiveScreen resizes the mode indicator overlay to the active screen (Windows stub).
func (o *Overlay) ResizeToActiveScreen() {}

// Destroy is unused on Windows; the indicator is drawn on the shared overlay.
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

// IndicatorConfig returns the indicator configuration.
func (o *Overlay) IndicatorConfig() config.ModeIndicatorConfig {
	o.configMu.RLock()
	defer o.configMu.RUnlock()

	return o.indicatorConfig
}

// ModeConfig returns the mode config for the given mode string.
func (o *Overlay) ModeConfig(mode string) config.ModeIndicatorModeConfig {
	o.configMu.RLock()
	defer o.configMu.RUnlock()

	switch mode {
	case "hints":
		return o.indicatorConfig.Hints
	case "grid":
		return o.indicatorConfig.Grid
	case "scroll":
		return o.indicatorConfig.Scroll
	case "recursive_grid":
		return o.indicatorConfig.RecursiveGrid
	default:
		return config.ModeIndicatorModeConfig{}
	}
}

// Theme returns the theme provider.
func (o *Overlay) Theme() config.ThemeProvider {
	return o.theme
}
