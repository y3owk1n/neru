//go:build linux

// Package modeindicator provides mode indicator overlay components.
package modeindicator

import (
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

// Overlay manages the rendering of mode indicator overlays using native platform APIs (Linux stub).
type Overlay struct {
	indicatorConfig config.ModeIndicatorConfig
	theme           config.ThemeProvider
	logger          *zap.Logger
	configMu        sync.RWMutex
}

// NewOverlay initializes a new mode indicator overlay instance with its own window (Linux stub).
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

// DrawModeIndicator draws the mode indicator for the specified mode (Linux stub).
func (o *Overlay) DrawModeIndicator(mode string) error {
	return nil
}

// Show shows the mode indicator overlay (Linux stub).
func (o *Overlay) Show() {}

// Hide hides the mode indicator overlay (Linux stub).
func (o *Overlay) Hide() {}

// Clear clears the mode indicator overlay (Linux stub).
func (o *Overlay) Clear() {}

// ResizeToActiveScreen resizes the mode indicator overlay to the active screen (Linux stub).
func (o *Overlay) ResizeToActiveScreen() {}

// Destroy destroys the mode indicator overlay (Linux stub).
func (o *Overlay) Destroy() {}

// SetConfig updates the indicator configuration (Linux stub).
func (o *Overlay) SetConfig(cfg config.ModeIndicatorConfig) {
	o.configMu.Lock()
	defer o.configMu.Unlock()

	o.indicatorConfig = cfg
}

// SetIndicatorConfig updates the indicator configuration (Linux stub).
func (o *Overlay) SetIndicatorConfig(cfg config.ModeIndicatorConfig) {
	o.SetConfig(cfg)
}

// IndicatorConfig returns the indicator configuration (Linux stub).
func (o *Overlay) IndicatorConfig() config.ModeIndicatorConfig {
	o.configMu.RLock()
	defer o.configMu.RUnlock()

	return o.indicatorConfig
}

// ThemeProvider returns the active theme provider used to resolve colors.
func (o *Overlay) ThemeProvider() config.ThemeProvider {
	return o.theme
}

// ResolveLabelText returns the configured indicator label for the mode.
func (o *Overlay) ResolveLabelText(mode string) string {
	o.configMu.RLock()
	defer o.configMu.RUnlock()

	modeCfg := o.resolveModeConfigLocked(mode)
	if modeCfg == nil || !modeCfg.Enabled {
		return ""
	}

	if modeCfg.Text != "" {
		return modeCfg.Text
	}

	return mode
}

// ResolveModeConfig returns the configured per-mode indicator settings.
func (o *Overlay) ResolveModeConfig(mode string) (config.ModeIndicatorModeConfig, bool) {
	o.configMu.RLock()
	defer o.configMu.RUnlock()

	modeCfg := o.resolveModeConfigLocked(mode)
	if modeCfg == nil {
		return config.ModeIndicatorModeConfig{}, false
	}

	return *modeCfg, true
}

func (o *Overlay) resolveModeConfigLocked(mode string) *config.ModeIndicatorModeConfig {
	switch mode {
	case "hints":
		return &o.indicatorConfig.Hints
	case "grid":
		return &o.indicatorConfig.Grid
	case "scroll":
		return &o.indicatorConfig.Scroll
	case "recursive_grid":
		return &o.indicatorConfig.RecursiveGrid
	default:
		return nil
	}
}
