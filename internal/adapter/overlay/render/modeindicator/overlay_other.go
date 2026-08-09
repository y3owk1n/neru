//go:build !darwin

package modeindicator

import (
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Overlay manages the rendering of mode indicator overlays (non-darwin stub).
// Drawing happens in the overlay manager's native surface (Cairo on Linux,
// GDI on Windows); this type only carries the configuration and theme, and
// resolves the per-mode label both managers render.
type Overlay struct {
	indicatorConfig config.ModeIndicatorConfig
	theme           config.ThemeProvider
	logger          *zap.Logger
	configMu        sync.RWMutex
}

// NewOverlay creates a new mode indicator overlay instance with its own window (non-darwin stub).
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

// DrawModeIndicator reports CodeNotSupported: off darwin the badge is painted
// onto the manager's shared surface, not by this type. ResolveLabelText below
// is the part of the indicator that does live here.
func (o *Overlay) DrawModeIndicator(_ string) error {
	return derrors.New(
		derrors.CodeNotSupported,
		"mode indicator drawing is handled by the overlay manager's native surface off darwin",
	)
}

// Show shows the mode indicator overlay (non-darwin stub).
func (o *Overlay) Show() {}

// Hide hides the mode indicator overlay (non-darwin stub).
func (o *Overlay) Hide() {}

// Clear clears the mode indicator overlay (non-darwin stub).
func (o *Overlay) Clear() {}

// ResizeToActiveScreen resizes the mode indicator overlay to the active screen (non-darwin stub).
func (o *Overlay) ResizeToActiveScreen() {}

// Destroy destroys the mode indicator overlay (non-darwin stub).
func (o *Overlay) Destroy() {}

// SetConfig updates the indicator configuration (non-darwin stub).
func (o *Overlay) SetConfig(cfg config.ModeIndicatorConfig) {
	o.configMu.Lock()
	defer o.configMu.Unlock()

	o.indicatorConfig = cfg
}

// SetIndicatorConfig updates the indicator configuration (non-darwin stub).
func (o *Overlay) SetIndicatorConfig(cfg config.ModeIndicatorConfig) {
	o.SetConfig(cfg)
}

// IndicatorConfig returns the indicator configuration (non-darwin stub).
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
// It is the single source of truth for label semantics on every non-darwin
// platform: a per-mode disabled indicator draws nothing, and an empty custom
// text falls back to the mode name.
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
