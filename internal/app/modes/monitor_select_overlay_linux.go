//go:build linux

package modes

import (
	"strings"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// showMonitorSelectLocked renders the interactive monitor picker on the shared
// Linux overlay (X11 spanning window or wlroots per-output layer surfaces),
// drawing one labeled panel per monitor. Unlike macOS's per-display NSPanels,
// this reuses the same overlay the hints/grid modes draw on.
func (h *Handler) showMonitorSelectLocked() error {
	if h.monitorSelect == nil {
		return nil
	}

	manager := overlay.Get()
	if manager == nil {
		return derrors.New(
			derrors.CodeNotSupported,
			"monitor_select overlay is unavailable on this Linux backend",
		)
	}

	targets, style := h.monitorSelectRenderDataLocked()
	if len(targets) == 0 {
		manager.HideMonitorSelect()

		return nil
	}

	return manager.DrawMonitorSelect(targets, style)
}

func (h *Handler) hideMonitorSelectLocked() error {
	if manager := overlay.Get(); manager != nil {
		manager.HideMonitorSelect()
	}

	return nil
}

// monitorSelectRenderDataLocked maps the active monitor_select session and the
// resolved (theme-applied) config into the overlay's render types.
func (h *Handler) monitorSelectRenderDataLocked() ([]overlay.MonitorSelectTarget, overlay.MonitorSelectStyle) {
	uiCfg := h.config.MonitorSelect.UI
	theme := h.themeProvider

	style := overlay.MonitorSelectStyle{
		FontSize:           uiCfg.FontSize,
		SubtitleFontSize:   uiCfg.SubtitleFontSize,
		FontFamily:         uiCfg.FontFamily,
		SubtitleFontFamily: uiCfg.SubtitleFontFamily,
		BorderRadius:       uiCfg.BorderRadius,
		PaddingX:           uiCfg.PaddingX,
		PaddingY:           uiCfg.PaddingY,
		BorderWidth:        uiCfg.BorderWidth,
		BackgroundColor: uiCfg.BackgroundColor.ForTheme(theme,
			configpkg.MonitorSelectBackgroundColorLight,
			configpkg.MonitorSelectBackgroundColorDark),
		TextColor: uiCfg.TextColor.ForTheme(theme,
			configpkg.MonitorSelectTextColorLight,
			configpkg.MonitorSelectTextColorDark),
		MatchedTextColor: uiCfg.MatchedTextColor.ForTheme(theme,
			configpkg.MonitorSelectMatchedTextColorLight,
			configpkg.MonitorSelectMatchedTextColorDark),
		BorderColor: uiCfg.BorderColor.ForTheme(theme,
			configpkg.MonitorSelectBorderColorLight,
			configpkg.MonitorSelectBorderColorDark),
		BackdropColor: uiCfg.BackdropColor.ForTheme(theme,
			configpkg.MonitorSelectBackdropColorLight,
			configpkg.MonitorSelectBackdropColorDark),
		SubtitleTextColor: uiCfg.SubtitleTextColor.ForTheme(theme,
			configpkg.MonitorSelectSubtitleTextColorLight,
			configpkg.MonitorSelectSubtitleTextColorDark),
	}

	input := h.monitorSelect.Input()

	selectedName := ""
	if selected := h.monitorSelect.Selected(); selected != nil {
		selectedName = selected.Name
	}

	sessionTargets := h.monitorSelect.Targets()

	targets := make([]overlay.MonitorSelectTarget, 0, len(sessionTargets))
	for _, target := range sessionTargets {
		targets = append(targets, overlay.MonitorSelectTarget{
			Bounds:           target.Bounds,
			Label:            target.Label,
			Subtitle:         target.Name,
			Selected:         target.Name == selectedName,
			MatchedPrefixLen: matchedPrefixLength(target.Label, input),
		})
	}

	return targets, style
}

// matchedPrefixLength returns how many leading runes of label are matched by the
// current (case-insensitive) input, or 0 when it is not a prefix. Mirrors the
// darwin implementation; defined per-platform so each build is self-contained.
func matchedPrefixLength(label, input string) int {
	if input == "" {
		return 0
	}

	labelRunes := []rune(label)
	inputRunes := []rune(strings.ToLower(input))
	labelFolded := []rune(strings.ToLower(label))

	if len(inputRunes) > len(labelRunes) {
		return 0
	}

	for idx := range inputRunes {
		if labelFolded[idx] != inputRunes[idx] {
			return 0
		}
	}

	return len(inputRunes)
}
