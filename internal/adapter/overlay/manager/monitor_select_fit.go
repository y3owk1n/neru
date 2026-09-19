package manager

import (
	"math"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
)

// What every backend sizes a monitor-select panel by. The Objective-C overlay
// (monitor_select_overlay_darwin.m) and the Linux and Windows layouts each
// carry these numbers for the panel itself; they are repeated here because the
// text has to be fitted to the panel before any of them lays it out.
const (
	monitorSelectDefaultFontSize         = 96
	monitorSelectDefaultSubtitleFontSize = 18
	monitorSelectLabelGap                = 4
	monitorSelectAutoPadXMin             = 24
	monitorSelectAutoPadYMin             = 12
	monitorSelectAutoPadXRatio           = 0.3
	monitorSelectAutoPadYRatio           = 0.15
	monitorSelectMaxFraction             = 0.8
)

// FittedTo returns the style with its two font sizes fitted to one target.
// These are the sizes at which the label and the subtitle stay inside the
// panel, which every backend caps at a fraction of the monitor
// (badge.MonitorSelectText.FitIn).
// The panel was clamped and the text was not, so the default 96 point label,
// or a long monitor name, ran out past the panel's edge on a small monitor.
//
// scale is device pixels per font unit for target.Bounds, 1 where the two are
// the same. The sizes come back whole, as the configuration writes them, so a
// backend lays the panel out and draws the text from the style it was handed.
func (s MonitorSelectStyle) FittedTo(target MonitorSelectTarget, scale float64) MonitorSelectStyle {
	labelSize := float64(s.FontSize)
	if s.FontSize <= 0 {
		labelSize = monitorSelectDefaultFontSize
	}

	subtitleSize := float64(s.SubtitleFontSize)
	if s.SubtitleFontSize <= 0 {
		subtitleSize = monitorSelectDefaultSubtitleFontSize
	}

	padX := float64(s.PaddingX)
	if s.PaddingX < 0 {
		padX = math.Max(monitorSelectAutoPadXMin, math.Round(labelSize*monitorSelectAutoPadXRatio))
	}

	padY := float64(s.PaddingY)
	if s.PaddingY < 0 {
		padY = math.Max(monitorSelectAutoPadYMin, math.Round(labelSize*monitorSelectAutoPadYRatio))
	}

	fittedLabel, fittedSubtitle := badge.MonitorSelectText{
		Label:          target.Label,
		Subtitle:       target.Subtitle,
		LabelFamily:    s.FontFamily,
		SubtitleFamily: s.SubtitleFontFamily,
		LabelSize:      labelSize,
		SubtitleSize:   subtitleSize,
		PadX:           padX,
		PadY:           padY,
		Gap:            monitorSelectLabelGap,
	}.FitIn(target.Bounds, scale, monitorSelectMaxFraction)

	fitted := s
	fitted.FontSize = max(int(fittedLabel), 1)
	fitted.SubtitleFontSize = max(int(fittedSubtitle), 1)

	return fitted
}
