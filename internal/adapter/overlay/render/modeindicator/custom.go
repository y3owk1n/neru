package modeindicator

import "github.com/y3owk1n/neru/internal/config"

// customModeConfig is the per-mode indicator config of a declared mode, or nil
// for a name no declaration gave a label.
//
// A declared mode has no [mode_indicator.<name>] section: its text is the
// declaration's, and its colors are left zero so every backend falls back to
// [mode_indicator.ui] through the same override it applies to a built-in mode
// without colors of its own. Enabled is what the non-darwin label resolution
// checks, and a label that exists is one that is shown.
func customModeConfig(customLabels map[string]string, mode string) *config.ModeIndicatorModeConfig {
	label, declared := customLabels[mode]
	if !declared {
		return nil
	}

	return &config.ModeIndicatorModeConfig{Enabled: true, Text: label}
}
