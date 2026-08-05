package modes

import (
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	overlaymanager "github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
)

// showMonitorSelect renders the interactive monitor picker through the overlay
// manager's optional MonitorSelector extension (implemented by the darwin and
// Linux backends). Backends without it report CodeNotSupported, and the mode
// refuses to activate.
func (h *handlerState) showMonitorSelect() error {
	if h.monitorSelect == nil {
		return nil
	}

	selector, ok := h.overlayManager.(overlaymanager.MonitorSelector)
	if !ok {
		return derrors.New(
			derrors.CodeNotSupported,
			"monitor_select overlay is unavailable on this backend",
		)
	}

	targets, style := h.monitorSelectRenderData()
	if len(targets) == 0 {
		selector.HideMonitorSelect()

		return nil
	}

	return selector.DrawMonitorSelect(targets, style)
}

// hideMonitorSelect removes the monitor-select panels. A backend without the
// MonitorSelector extension has nothing to hide, so this cannot fail.
func (h *handlerState) hideMonitorSelect() {
	if selector, ok := h.overlayManager.(overlaymanager.MonitorSelector); ok {
		selector.HideMonitorSelect()
	}
}

// monitorSelectRenderData maps the active monitor_select session and the
// resolved (theme-applied) config into the overlay's render types.
func (h *handlerState) monitorSelectRenderData() ([]overlay.MonitorSelectTarget, overlay.MonitorSelectStyle) {
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
		HideInScreenShare: h.config.General.HideOverlayInScreenShare,
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

// matchedPrefixLength returns how many leading runes of label are matched by
// the current (case-insensitive) input, or 0 when it is not a prefix.
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

func (h *handlerState) redrawMonitorSelect() {
	if h.monitorSelect == nil {
		return
	}

	err := h.showMonitorSelect()
	if err != nil {
		h.logger.Debug("Failed to redraw monitor_select overlay", zap.Error(err))
	}
}

// RefreshMonitorSelectForThemeChange redraws the monitor_select overlay using
// the latest theme-resolved colors when the mode is active.
func (h *Handler) RefreshMonitorSelectForThemeChange() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() != domain.ModeMonitorSelect || h.monitorSelect == nil {
		return
	}

	h.redrawMonitorSelect()
}
