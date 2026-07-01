//go:build darwin

package modes

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#include "../../core/infra/platform/darwin/overlay.h"
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"
)

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
			isCurrent:        C.int(boolToCInt(target.IsCurrent)),
			isSelected:       C.int(boolToCInt(target.IsSelected)),
			matchedPrefixLen: C.int(target.MatchedPrefixLen),
		}
	}

	cStyle := C.MonitorSelectStyle{
		fontSize:               C.int(style.FontSize),
		subtitleFontSize:       C.int(style.SubtitleFontSize),
		fontFamily:             cStringOrNil(style.FontFamily),
		subtitleFontFamily:     cStringOrNil(style.SubtitleFontFamily),
		borderRadius:           C.int(style.BorderRadius),
		paddingX:               C.int(style.PaddingX),
		paddingY:               C.int(style.PaddingY),
		borderWidth:            C.int(style.BorderWidth),
		backgroundColor:        cStringOrNil(style.BackgroundColor),
		textColor:              cStringOrNil(style.TextColor),
		matchedTextColor:       cStringOrNil(style.MatchedTextColor),
		borderColor:            cStringOrNil(style.BorderColor),
		backdropColor:          cStringOrNil(style.BackdropColor),
		currentBackgroundColor: cStringOrNil(style.CurrentBackgroundColor),
		currentTextColor:       cStringOrNil(style.CurrentTextColor),
		currentBorderColor:     cStringOrNil(style.CurrentBorderColor),
		subtitleTextColor:      cStringOrNil(style.SubtitleTextColor),
		hideInScreenShare:      C.int(boolToCInt(style.HideInScreenShare)),
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
	if styl.currentBackgroundColor != nil {
		C.free(unsafe.Pointer(styl.currentBackgroundColor))
	}
	if styl.currentTextColor != nil {
		C.free(unsafe.Pointer(styl.currentTextColor))
	}
	if styl.currentBorderColor != nil {
		C.free(unsafe.Pointer(styl.currentBorderColor))
	}
	if styl.subtitleTextColor != nil {
		C.free(unsafe.Pointer(styl.subtitleTextColor))
	}
}
