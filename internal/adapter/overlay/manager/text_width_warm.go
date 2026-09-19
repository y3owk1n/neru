package manager

import (
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
)

// WarmBadgeTextWidths measures, ahead of any draw, the characters of every face
// a backend sizes a box around with badge.TextWidth: hint labels, the hint
// search badge, recursive-grid and bisect label plates, the mode and
// sticky-modifier indicators, and the monitor picker's badge.
//
// A backend that sizes boxes that way calls it when it is handed a
// configuration. Hint narrowing and the search badge redraw on every
// keystroke, and the first draw in a face would otherwise ask the platform's
// text layer once per new character with a key waiting on it. macOS measures
// its boxes natively and has no use for it.
//
// Each face is warmed at the weight its text is drawn at. Families are
// resolved the way the draws resolve them, so the table warmed is the table
// read.
func WarmBadgeTextWidths(cfg *config.Config) {
	if cfg == nil {
		return
	}

	const (
		bold    = true
		regular = false
	)

	indicatorTexts := cfg.ModeIndicator.Scroll.Text + cfg.ModeIndicator.Hints.Text +
		cfg.ModeIndicator.Grid.Text + cfg.ModeIndicator.RecursiveGrid.Text +
		cfg.ModeIndicator.Bisect.Text + cfg.ModeIndicator.MonitorSelect.Text

	monitorSubtitleFamily := cfg.MonitorSelect.UI.SubtitleFontFamily
	if monitorSubtitleFamily == "" {
		monitorSubtitleFamily = cfg.MonitorSelect.UI.FontFamily
	}

	for _, face := range []struct {
		family string
		bold   bool
		extra  string
	}{
		{cfg.Hints.UI.FontFamily, bold, cfg.Hints.HintCharacters},
		{cfg.Hints.SearchInputUI.FontFamily, regular, ""},
		{cfg.RecursiveGrid.UI.FontFamily, regular, cfg.RecursiveGrid.UI.LabelChar},
		{cfg.Bisect.UI.FontFamily, regular, cfg.Bisect.UI.LabelChar},
		{cfg.ModeIndicator.UI.FontFamily, bold, indicatorTexts},
		{cfg.StickyModifiers.UI.FontFamily, bold, ""},
		{cfg.MonitorSelect.UI.FontFamily, bold, cfg.MonitorSelect.Characters},
		{monitorSubtitleFamily, regular, ""},
	} {
		badge.WarmTextWidths(ports.ResolveFont(face.family), face.bold, face.extra)
	}
}
