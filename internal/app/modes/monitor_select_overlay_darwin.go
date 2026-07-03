//go:build darwin

package modes

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#include "../../core/infra/platform/darwin/overlay.h"
#include <stdlib.h>
*/
import "C"

import (
	"image"
	"strings"
	"unsafe"

	configpkg "github.com/y3owk1n/neru/internal/config"
)

type monitorSelectRenderTarget struct {
	Bounds           image.Rectangle
	Label            string
	Subtitle         string
	IsSelected       bool
	MatchedPrefixLen int
}

type monitorSelectRenderStyle struct {
	FontSize           int
	SubtitleFontSize   int
	FontFamily         string
	SubtitleFontFamily string
	BorderRadius       int
	PaddingX           int
	PaddingY           int
	BorderWidth        int
	BackgroundColor    string
	TextColor          string
	MatchedTextColor   string
	BorderColor        string
	BackdropColor      string
	SubtitleTextColor  string
	HideInScreenShare  bool
}

type monitorSelectRenderState struct {
	Input   string
	Targets []monitorSelectRenderTarget
	Style   monitorSelectRenderStyle
}

func (h *Handler) showMonitorSelectLocked() error {
	if h.monitorSelect == nil {
		return nil
	}

	state := h.currentMonitorSelectRenderStateLocked()
	targets := state.Targets
	style := state.Style

	if len(targets) == 0 {
		C.NeruHideMonitorSelectPanels()

		return nil
	}

	cTargets := make([]C.MonitorSelectTargetData, len(targets))
	for idx, target := range targets {
		cTargets[idx] = C.MonitorSelectTargetData{
			x:                C.int(target.Bounds.Min.X),
			y:                C.int(target.Bounds.Min.Y),
			width:            C.int(target.Bounds.Dx()),
			height:           C.int(target.Bounds.Dy()),
			label:            C.CString(target.Label),
			subtitle:         C.CString(target.Subtitle),
			isSelected:       C.int(boolToCInt(target.IsSelected)),
			matchedPrefixLen: C.int(target.MatchedPrefixLen),
		}
	}

	cStyle := C.MonitorSelectStyle{
		fontSize:           C.int(style.FontSize),
		subtitleFontSize:   C.int(style.SubtitleFontSize),
		fontFamily:         cStringOrNil(style.FontFamily),
		subtitleFontFamily: cStringOrNil(style.SubtitleFontFamily),
		borderRadius:       C.int(style.BorderRadius),
		paddingX:           C.int(style.PaddingX),
		paddingY:           C.int(style.PaddingY),
		borderWidth:        C.int(style.BorderWidth),
		backgroundColor:    cStringOrNil(style.BackgroundColor),
		textColor:          cStringOrNil(style.TextColor),
		matchedTextColor:   cStringOrNil(style.MatchedTextColor),
		borderColor:        cStringOrNil(style.BorderColor),
		backdropColor:      cStringOrNil(style.BackdropColor),
		subtitleTextColor:  cStringOrNil(style.SubtitleTextColor),
		hideInScreenShare:  C.int(boolToCInt(style.HideInScreenShare)),
	}

	C.NeruShowMonitorSelectPanels(&cTargets[0], C.int(len(cTargets)), cStyle)

	for idx := range cTargets {
		C.free(unsafe.Pointer(cTargets[idx].label))
		C.free(unsafe.Pointer(cTargets[idx].subtitle))
	}
	freeMonitorSelectStyle(cStyle)

	return nil
}

func (h *Handler) hideMonitorSelectLocked() error {
	C.NeruHideMonitorSelectPanels()

	return nil
}

func boolToCInt(v bool) int {
	if v {
		return 1
	}

	return 0
}

func cStringOrNil(s string) *C.char {
	if s == "" {
		return nil
	}

	return C.CString(s)
}

func freeMonitorSelectStyle(styl C.MonitorSelectStyle) {
	if styl.fontFamily != nil {
		C.free(unsafe.Pointer(styl.fontFamily))
	}
	if styl.subtitleFontFamily != nil {
		C.free(unsafe.Pointer(styl.subtitleFontFamily))
	}
	if styl.backgroundColor != nil {
		C.free(unsafe.Pointer(styl.backgroundColor))
	}
	if styl.textColor != nil {
		C.free(unsafe.Pointer(styl.textColor))
	}
	if styl.matchedTextColor != nil {
		C.free(unsafe.Pointer(styl.matchedTextColor))
	}
	if styl.borderColor != nil {
		C.free(unsafe.Pointer(styl.borderColor))
	}
	if styl.backdropColor != nil {
		C.free(unsafe.Pointer(styl.backdropColor))
	}
	if styl.subtitleTextColor != nil {
		C.free(unsafe.Pointer(styl.subtitleTextColor))
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

	for _, target := range h.monitorSelect.Targets() {
		state.Targets = append(state.Targets, monitorSelectRenderTarget{
			Bounds:           target.Bounds,
			Label:            target.Label,
			Subtitle:         target.Name,
			IsSelected:       target.Name == selectedName,
			MatchedPrefixLen: matchedPrefixLength(target.Label, state.Input),
		})
	}

	return state
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
