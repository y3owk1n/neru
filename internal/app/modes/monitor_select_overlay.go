package modes

import (
	"image"
	"strings"

	"go.uber.org/zap"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
)

type monitorSelectRenderTarget struct {
	Bounds           image.Rectangle
	Label            string
	Subtitle         string
	IsCurrent        bool
	IsSelected       bool
	MatchedPrefixLen int
}

type monitorSelectRenderStyle struct {
	FontSize               int
	SubtitleFontSize       int
	FontFamily             string
	SubtitleFontFamily     string
	BorderRadius           int
	PaddingX               int
	PaddingY               int
	BorderWidth            int
	BackgroundColor        string
	TextColor              string
	MatchedTextColor       string
	BorderColor            string
	BackdropColor          string
	CurrentBackgroundColor string
	CurrentTextColor       string
	CurrentBorderColor     string
	SubtitleTextColor      string
	HideInScreenShare      bool
}

type monitorSelectRenderState struct {
	Input   string
	Targets []monitorSelectRenderTarget
	Style   monitorSelectRenderStyle
}

func (h *Handler) redrawMonitorSelectLocked() {
	if h.monitorSelect == nil {
		return
	}

	err := h.showMonitorSelectLocked()
	if err != nil {
		h.logger.Debug("Failed to redraw monitor_select overlay", zap.Error(err))
	}
}

func (h *Handler) currentMonitorSelectRenderStateLocked() monitorSelectRenderState {
	cfg := h.config.MonitorSelect
	uiCfg := cfg.UI

	theme := h.themeProvider

	state := monitorSelectRenderState{
		Input: h.monitorSelect.Input(),
		Style: monitorSelectRenderStyle{
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
			CurrentBackgroundColor: uiCfg.CurrentBackgroundColor.ForTheme(theme,
				configpkg.MonitorSelectCurrentBackgroundColorLight,
				configpkg.MonitorSelectCurrentBackgroundColorDark),
			CurrentTextColor: uiCfg.CurrentTextColor.ForTheme(theme,
				configpkg.MonitorSelectCurrentTextColorLight,
				configpkg.MonitorSelectCurrentTextColorDark),
			CurrentBorderColor: uiCfg.CurrentBorderColor.ForTheme(theme,
				configpkg.MonitorSelectCurrentBorderColorLight,
				configpkg.MonitorSelectCurrentBorderColorDark),
			SubtitleTextColor: uiCfg.SubtitleTextColor.ForTheme(theme,
				configpkg.MonitorSelectSubtitleTextColorLight,
				configpkg.MonitorSelectSubtitleTextColorDark),
			HideInScreenShare: h.config.General.HideOverlayInScreenShare,
		},
	}

	selected := h.monitorSelect.Selected()

	selectedName := ""
	if selected != nil {
		selectedName = selected.Name
	}

	if current := h.monitorSelect.Current(); current != nil {
		state.Targets = append(state.Targets, monitorSelectRenderTarget{
			Bounds:     current.Bounds,
			Label:      "Current",
			Subtitle:   current.Name,
			IsCurrent:  true,
			IsSelected: false,
		})
	}

	for _, target := range h.monitorSelect.Targets() {
		state.Targets = append(state.Targets, monitorSelectRenderTarget{
			Bounds:           target.Bounds,
			Label:            target.Label,
			Subtitle:         target.Name,
			IsCurrent:        false,
			IsSelected:       target.Name == selectedName,
			MatchedPrefixLen: matchedPrefixLength(target.Label, state.Input),
		})
	}

	return state
}

// RefreshMonitorSelectForThemeChange redraws the monitor_select overlay using
// the latest theme-resolved colors when the mode is active.
func (h *Handler) RefreshMonitorSelectForThemeChange() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() != domain.ModeMonitorSelect || h.monitorSelect == nil {
		return
	}

	h.redrawMonitorSelectLocked()
}

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
