//go:build windows

package windows

import (
	"image"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The process is per-monitor-v2 DPI aware (the manifest go-winres embeds),
// so Windows hands it physical pixels and scales nothing for it. A font size
// or padding from config is then a physical size too. On a 150% display the
// overlay drew it a third smaller than macOS or an Xft.dpi-scaled X11 desktop
// draws the same value. The overlay reads the monitor's effective DPI here
// and multiplies what it draws by that factor.
const (
	// mdtEffectiveDPI is MDT_EFFECTIVE_DPI: the DPI the user's scaling
	// percentage sets, which is the one text should follow.
	mdtEffectiveDPI = 0
	// baseDPI is the 100% scale Windows measures fonts against.
	baseDPI = 96.0
)

var (
	shcore               = windows.NewLazySystemDLL("shcore.dll")
	procGetDpiForMonitor = shcore.NewProc("GetDpiForMonitor")
)

// DPIScaleAt returns the scale factor of the monitor nearest point, 1 at
// 100% and 1.5 at 150%. A monitor or DPI that cannot be read gives 1, so the
// caller draws at 100% rather than not at all.
func DPIScaleAt(point image.Point) float64 {
	hMonitor, _, _ := procMonitorFromPoint.Call(
		packMonitorPoint(point),
		uintptr(monitorDefaultToNearest),
	)
	if hMonitor == 0 {
		return 1
	}

	return monitorDPIScale(hMonitor)
}

// monitorDPIScale returns hMonitor's effective DPI over 96, or 1 when
// GetDpiForMonitor is unavailable (before Windows 8.1) or fails.
func monitorDPIScale(hMonitor uintptr) float64 {
	if procGetDpiForMonitor.Find() != nil {
		return 1
	}

	var dpiX, dpiY uint32

	result, _, _ := procGetDpiForMonitor.Call(
		hMonitor,
		uintptr(mdtEffectiveDPI),
		uintptr(unsafe.Pointer(&dpiX)),
		uintptr(unsafe.Pointer(&dpiY)),
	)
	if result != 0 || dpiX == 0 {
		return 1
	}

	return float64(dpiX) / baseDPI
}
