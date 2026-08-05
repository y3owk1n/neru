//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#include "../../platform/darwin/overlay.h"
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
)

// Ensure the darwin manager implements the optional monitor-select extension.
var _ manager.MonitorSelector = (*Manager)(nil)

// DrawMonitorSelect renders one labeled panel per monitor.
//
// Unlike the other overlay draws (dispatch_async), the C sinks here use
// dispatch_sync onto the main queue, because the dispatched block borrows the
// C target array this function frees on return. The mode handler calls this
// with its lock held, so the lock blocks on the main queue draining — safe
// only while nothing that runs on the main queue acquires the handler lock
// (true today: event tap, hotkey, theme and systray callbacks all hop to
// goroutines before reaching the handler).
func (m *Manager) DrawMonitorSelect(
	targets []manager.MonitorSelectTarget,
	style manager.MonitorSelectStyle,
) error {
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
			isSelected:       C.int(boolToInt(target.Selected)),
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
		hideInScreenShare:  C.int(boolToInt(style.HideInScreenShare)),
	}

	C.NeruShowMonitorSelectPanels(&cTargets[0], C.int(len(cTargets)), cStyle)

	for idx := range cTargets {
		C.free(unsafe.Pointer(cTargets[idx].label))
		C.free(unsafe.Pointer(cTargets[idx].subtitle))
	}
	freeMonitorSelectStyle(cStyle)

	return nil
}

// HideMonitorSelect removes the monitor-select panels.
func (m *Manager) HideMonitorSelect() {
	C.NeruHideMonitorSelectPanels()
}

func cStringOrNil(s string) *C.char {
	if s == "" {
		return nil
	}

	return C.CString(s)
}

func freeMonitorSelectStyle(styl C.MonitorSelectStyle) {
	for _, ptr := range []*C.char{
		styl.fontFamily,
		styl.subtitleFontFamily,
		styl.backgroundColor,
		styl.textColor,
		styl.matchedTextColor,
		styl.borderColor,
		styl.backdropColor,
		styl.subtitleTextColor,
	} {
		if ptr != nil {
			C.free(unsafe.Pointer(ptr))
		}
	}
}
