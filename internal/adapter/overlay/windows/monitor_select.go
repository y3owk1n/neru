//go:build windows

package windows

import (
	"image"
	"math"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	winplatform "github.com/y3owk1n/neru/internal/adapter/platform/windows"
)

// Ensure the Windows manager implements the optional monitor-select extension.
var _ manager.MonitorSelector = (*Manager)(nil)

const (
	// These mirror the macOS overlay (monitor_select_overlay_darwin.m) and the
	// Linux Cairo panels so the picker looks the same on every platform: auto
	// padding derived from the label font when the config uses the -1
	// sentinel, a small label/subtitle gap, an 80% cap on panel size relative
	// to the monitor, and default font sizes.
	monitorSelectLabelGap       = 4
	monitorSelectAutoPadXMin    = 24
	monitorSelectAutoPadYMin    = 12
	monitorSelectAutoPadXRatio  = 0.3
	monitorSelectAutoPadYRatio  = 0.15
	monitorSelectMaxFraction    = 0.8
	monitorSelectMaxRadius      = 16.0
	monitorSelectDefaultFont    = 96
	monitorSelectDefaultSubFont = 18
	monitorSelectHalfDivisor    = 2
)

// monitorSelectFontOr returns the configured font size, or a default when unset
// (<= 0), matching the macOS overlay's fallbacks (96 label / 18 subtitle).
func monitorSelectFontOr(value, fallback int) float64 {
	if value <= 0 {
		return float64(fallback)
	}

	return float64(value)
}

// monitorSelectPanelLayout computes, in global pixels, the panel rect centered
// on the monitor, the label and subtitle text rects, and the corner radius.
// It is the Linux layout at scale 1: the layered windows here are placed in
// the physical pixels EnumDisplayMonitors reports, so there is no factor to
// apply. Padding and radius honor the same "auto" (-1) config sentinels.
func monitorSelectPanelLayout(
	monitor image.Rectangle,
	label, subtitle string,
	style manager.MonitorSelectStyle,
) (image.Rectangle, image.Rectangle, image.Rectangle, float64) {
	labelFont := monitorSelectFontOr(style.FontSize, monitorSelectDefaultFont)
	subFont := monitorSelectFontOr(style.SubtitleFontSize, monitorSelectDefaultSubFont)

	padX := style.PaddingX
	if style.PaddingX < 0 {
		padX = max(monitorSelectAutoPadXMin, int(math.Round(labelFont*monitorSelectAutoPadXRatio)))
	}

	padY := style.PaddingY
	if style.PaddingY < 0 {
		padY = max(monitorSelectAutoPadYMin, int(math.Round(labelFont*monitorSelectAutoPadYRatio)))
	}

	labelW := badge.EstimateTextWidth(label, labelFont)
	labelH := badge.EstimateTextHeight(labelFont)

	subW, subH, gap := 0, 0, 0
	if subtitle != "" {
		subW = badge.EstimateTextWidth(subtitle, subFont)
		subH = badge.EstimateTextHeight(subFont)
		gap = monitorSelectLabelGap
	}

	panelW := max(labelW, subW) + padX*winPaddingMultiplier

	panelH := labelH + padY*winPaddingMultiplier
	if subtitle != "" {
		panelH += subH + gap
	}

	panelW = min(panelW, int(float64(monitor.Dx())*monitorSelectMaxFraction))
	panelH = min(panelH, int(float64(monitor.Dy())*monitorSelectMaxFraction))

	center := image.Pt(
		monitor.Min.X+monitor.Dx()/monitorSelectHalfDivisor,
		monitor.Min.Y+monitor.Dy()/monitorSelectHalfDivisor,
	)
	panel := badge.CenteredOn(center, panelW, panelH)

	radius := float64(style.BorderRadius)
	if style.BorderRadius < 0 {
		radius = math.Min(float64(panelH)/monitorSelectHalfDivisor, monitorSelectMaxRadius)
	}

	totalTextH := labelH
	if subtitle != "" {
		totalTextH += gap + subH
	}

	textTop := panel.Min.Y + (panelH-totalTextH)/monitorSelectHalfDivisor
	labelRect := image.Rect(panel.Min.X, textTop, panel.Max.X, textTop+labelH)

	subtitleRect := image.Rectangle{}
	if subtitle != "" {
		subTop := labelRect.Max.Y + gap
		subtitleRect = image.Rect(panel.Min.X, subTop, panel.Max.X, subTop+subH)
	}

	return panel, labelRect, subtitleRect, radius
}

// DrawMonitorSelect renders one labeled panel per monitor for the interactive
// monitor picker. Each target gets a layered window of its own covering that
// monitor, the way macOS gives each display an NSPanel: the shared overlay is
// sized to the active monitor only, and the picker has to reach every one.
// Windows are kept between draws so a narrowing keystroke repaints in place,
// and one left over from a display that went away is hidden rather than
// destroyed, so the next draw with more targets does not recreate it.
func (m *Manager) DrawMonitorSelect(
	targets []manager.MonitorSelectTarget,
	style manager.MonitorSelectStyle,
) error {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	backdrop := badge.ParseHexARGB(style.BackdropColor)
	hasBackdrop := strings.TrimSpace(style.BackdropColor) != ""
	background := badge.ParseHexARGB(style.BackgroundColor)
	border := badge.ParseHexARGB(style.BorderColor)
	text := badge.ParseHexARGB(style.TextColor)
	subtitleText := badge.ParseHexARGB(style.SubtitleTextColor)
	borderWidth := float64(max(style.BorderWidth, 1))
	labelFont := monitorSelectFontOr(style.FontSize, monitorSelectDefaultFont)
	subtitleFont := monitorSelectFontOr(style.SubtitleFontSize, monitorSelectDefaultSubFont)

	drawn := 0

	for _, target := range targets {
		if target.Bounds.Empty() {
			continue
		}

		win, err := m.monitorWindowLocked(drawn, target.Bounds)
		if err != nil {
			// The adapter records a draw only when it succeeds, so a panel
			// already shown here would outlive the refusal.
			m.hideMonitorWindowsLocked()

			return err
		}

		drawn++

		// Every rect below is global; the window's own pixels start at the
		// monitor's origin.
		local := target.Bounds.Sub(target.Bounds.Min)

		win.Clear()

		if hasBackdrop {
			win.FillRect(local, backdrop)
		}

		panel, labelRect, subtitleRect, radius := monitorSelectPanelLayout(
			target.Bounds, target.Label, target.Subtitle, style,
		)
		panel = panel.Sub(target.Bounds.Min)
		labelRect = labelRect.Sub(target.Bounds.Min)
		subtitleRect = subtitleRect.Sub(target.Bounds.Min)

		win.FillRoundedRect(panel, radius, background)
		win.StrokeRoundedRect(panel, radius, border, borderWidth)
		win.DrawTextCentered(target.Label, labelRect, style.FontFamily, labelFont, text)

		if target.Subtitle != "" {
			win.DrawTextCentered(
				target.Subtitle, subtitleRect, style.SubtitleFontFamily, subtitleFont, subtitleText,
			)
		}

		flushErr := win.Flush()
		if flushErr != nil && m.logger != nil {
			m.logger.Error("monitor_select panel flush failed", zap.Error(flushErr))
		}

		win.Show()
	}

	for _, win := range m.monitorWins[drawn:] {
		win.Hide()
	}

	return nil
}

// monitorWindowLocked returns the panel window for the target at index,
// covering bounds, creating or recreating it when there is none or the last
// one is no longer healthy.
func (m *Manager) monitorWindowLocked(
	index int,
	bounds image.Rectangle,
) (*winplatform.OverlayWindow, error) {
	if index < len(m.monitorWins) && m.monitorWins[index].Healthy() {
		win := m.monitorWins[index]

		return win, win.ResizeTo(bounds.Min.X, bounds.Min.Y, bounds.Dx(), bounds.Dy())
	}

	win, err := winplatform.NewOverlayWindowAt(bounds.Min.X, bounds.Min.Y, bounds.Dx(), bounds.Dy())
	if err != nil {
		return nil, err
	}

	if index < len(m.monitorWins) {
		m.monitorWins[index].Destroy()
		m.monitorWins[index] = win
	} else {
		m.monitorWins = append(m.monitorWins, win)
	}

	return win, nil
}

// HideMonitorSelect takes every panel window off the screen.
func (m *Manager) HideMonitorSelect() {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	m.hideMonitorWindowsLocked()
}

func (m *Manager) hideMonitorWindowsLocked() {
	for _, win := range m.monitorWins {
		win.Hide()
	}
}

// destroyMonitorWindowsLocked releases the panel windows. Called from Destroy
// with renderMu held.
func (m *Manager) destroyMonitorWindowsLocked() {
	for _, win := range m.monitorWins {
		win.Destroy()
	}

	m.monitorWins = nil
}
