//go:build windows

// internal/core/infra/platform/windows/win32.go
// Low-level Win32 helpers for screen, cursor, window, and process queries.
// Does not implement ports.SystemPort; system.go delegates here.

package windows

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

const (
	cchDeviceName                  = 32
	processQueryLimitedInformation = 0x1000
	processNameWin32               = 0
	monitorDefaultToNearest        = 2
)

type monitorInfoEx struct {
	cbSize    uint32
	rcMonitor windows.Rect
	rcWork    windows.Rect
	dwFlags   uint32
	szDevice  [cchDeviceName]uint16
}

type displayDevice struct {
	cb           uint32
	deviceName   [cchDeviceName]uint16
	deviceString [128]uint16
	stateFlags   uint32
	deviceID     [128]uint16
	deviceKey    [128]uint16
}

type displayMonitor struct {
	name   string
	bounds image.Rectangle
}

type winPoint struct {
	x int32
	y int32
}

type winSize struct {
	cx int32
	cy int32
}

var (
	user32 = windows.NewLazySystemDLL("user32.dll")

	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetCursorPos        = user32.NewProc("SetCursorPos")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procMonitorFromPoint    = user32.NewProc("MonitorFromPoint")
	procEnumDisplayDevicesW = user32.NewProc("EnumDisplayDevicesW")
)

func win32Bool(ret uintptr, err error) error {
	if ret == 0 {
		if err != nil && err != syscall.Errno(0) {
			return err
		}

		return syscall.EINVAL
	}

	return nil
}

func rectToImage(rect windows.Rect) image.Rectangle {
	return image.Rect(
		int(rect.Left),
		int(rect.Top),
		int(rect.Right),
		int(rect.Bottom),
	)
}

func cursorPosition() (image.Point, error) {
	var pt winPoint
	ret, _, err := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if callErr := win32Bool(ret, err); callErr != nil {
		return image.Point{}, fmt.Errorf("GetCursorPos: %w", callErr)
	}

	return image.Point{X: int(pt.x), Y: int(pt.y)}, nil
}

func moveCursorTo(point image.Point) error {
	ret, _, err := procSetCursorPos.Call(uintptr(point.X), uintptr(point.Y))
	if callErr := win32Bool(ret, err); callErr != nil {
		return fmt.Errorf("SetCursorPos: %w", callErr)
	}

	return nil
}

func getMonitorInfo(hMonitor windows.Handle) (monitorInfoEx, error) {
	var info monitorInfoEx
	info.cbSize = uint32(unsafe.Sizeof(info))

	ret, _, err := procGetMonitorInfoW.Call(
		uintptr(hMonitor),
		uintptr(unsafe.Pointer(&info)),
	)
	if callErr := win32Bool(ret, err); callErr != nil {
		return monitorInfoEx{}, fmt.Errorf("GetMonitorInfoW: %w", callErr)
	}

	return info, nil
}

func monitorFriendlyName(deviceName string) string {
	var adapter displayDevice
	adapter.cb = uint32(unsafe.Sizeof(adapter))

	adapterName, err := windows.UTF16PtrFromString(deviceName)
	if err != nil {
		return deviceName
	}

	for monitorIndex := uint32(0); ; monitorIndex++ {
		var monitor displayDevice
		monitor.cb = uint32(unsafe.Sizeof(monitor))

		ret, _, _ := procEnumDisplayDevicesW.Call(
			uintptr(unsafe.Pointer(adapterName)),
			uintptr(monitorIndex),
			uintptr(unsafe.Pointer(&monitor)),
			0,
		)
		if ret == 0 {
			break
		}

		if monitor.stateFlags&0x1 == 0 { // DISPLAY_DEVICE_ACTIVE
			continue
		}

		name := windows.UTF16ToString(monitor.deviceString[:])
		if name != "" {
			return name
		}
	}

	var device displayDevice
	device.cb = uint32(unsafe.Sizeof(device))

	ret, _, _ := procEnumDisplayDevicesW.Call(
		uintptr(unsafe.Pointer(adapterName)),
		0,
		uintptr(unsafe.Pointer(&device)),
		0,
	)
	if ret != 0 {
		if name := windows.UTF16ToString(device.deviceString[:]); name != "" {
			return name
		}
	}

	return deviceName
}

type monitorEnumState struct {
	monitors []displayMonitor
}

func enumerateMonitors() ([]displayMonitor, error) {
	state := &monitorEnumState{}

	callback := syscall.NewCallback(func(
		hMonitor uintptr,
		_ uintptr,
		_ uintptr,
		dwData uintptr,
	) uintptr {
		enumState := (*monitorEnumState)(unsafe.Pointer(dwData))

		info, err := getMonitorInfo(windows.Handle(hMonitor))
		if err != nil {
			return 1
		}

		deviceName := windows.UTF16ToString(info.szDevice[:])
		enumState.monitors = append(enumState.monitors, displayMonitor{
			name:   monitorFriendlyName(deviceName),
			bounds: rectToImage(info.rcMonitor),
		})

		return 1
	})

	ret, _, err := procEnumDisplayMonitors.Call(
		0,
		0,
		callback,
		uintptr(unsafe.Pointer(state)),
	)
	if callErr := win32Bool(ret, err); callErr != nil {
		return nil, fmt.Errorf("EnumDisplayMonitors: %w", callErr)
	}

	if len(state.monitors) == 0 {
		return nil, fmt.Errorf("EnumDisplayMonitors: no monitors found")
	}

	return state.monitors, nil
}

func activeScreenBounds() (image.Rectangle, error) {
	cursor, err := cursorPosition()
	if err == nil {
		pt := winPoint{x: int32(cursor.X), y: int32(cursor.Y)}
		ret, _, pointErr := procMonitorFromPoint.Call(
			uintptr(unsafe.Pointer(&pt)),
			uintptr(monitorDefaultToNearest),
		)
		if ret != 0 {
			info, infoErr := getMonitorInfo(windows.Handle(ret))
			if infoErr == nil {
				return rectToImage(info.rcMonitor), nil
			}
		} else if pointErr != nil && pointErr != syscall.Errno(0) {
			return image.Rectangle{}, fmt.Errorf("MonitorFromPoint: %w", pointErr)
		}
	}

	monitors, err := enumerateMonitors()
	if err != nil {
		return image.Rectangle{}, err
	}

	return monitors[0].bounds, nil
}

func screenBoundsByName(name string) (image.Rectangle, bool, error) {
	monitors, err := enumerateMonitors()
	if err != nil {
		return image.Rectangle{}, false, err
	}

	for _, monitor := range monitors {
		if strings.EqualFold(monitor.name, name) {
			return monitor.bounds, true, nil
		}
	}

	return image.Rectangle{}, false, nil
}

func screenNames() ([]string, error) {
	monitors, err := enumerateMonitors()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(monitors))
	for _, monitor := range monitors {
		names = append(names, monitor.name)
	}

	return names, nil
}

func foregroundWindowHandle() (windows.HWND, error) {
	hwnd := windows.GetForegroundWindow()
	if hwnd == 0 {
		return 0, derrors.New(derrors.CodeElementNotFound, "no foreground window")
	}

	if !windows.IsWindow(hwnd) {
		return 0, derrors.New(derrors.CodeElementNotFound, "foreground handle is not a window")
	}

	desktop := windows.GetDesktopWindow()
	if hwnd == desktop {
		return 0, derrors.New(derrors.CodeElementNotFound, "desktop is focused")
	}

	return hwnd, nil
}

func focusedWindowBounds() (image.Rectangle, bool, error) {
	hwnd, err := foregroundWindowHandle()
	if err != nil {
		if derrors.IsCode(err, derrors.CodeElementNotFound) {
			return image.Rectangle{}, false, nil
		}

		return image.Rectangle{}, false, err
	}

	if !windows.IsWindowVisible(hwnd) {
		return image.Rectangle{}, false, nil
	}

	var rect windows.Rect
	ret, _, err := procGetWindowRect.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&rect)),
	)
	if callErr := win32Bool(ret, err); callErr != nil {
		return image.Rectangle{}, false, fmt.Errorf("GetWindowRect: %w", callErr)
	}

	return rectToImage(rect), true, nil
}

func focusedApplicationPID() (int, error) {
	hwnd, err := foregroundWindowHandle()
	if err != nil {
		return 0, err
	}

	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &pid); err != nil {
		return 0, fmt.Errorf("GetWindowThreadProcessId: %w", err)
	}

	if pid == 0 {
		return 0, derrors.New(derrors.CodeElementNotFound, "foreground window has no process id")
	}

	return int(pid), nil
}

func processImagePath(pid int) (string, error) {
	if pid <= 0 {
		return "", derrors.New(derrors.CodeInvalidInput, "invalid process id")
	}

	handle, err := windows.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return "", fmt.Errorf("OpenProcess: %w", err)
	}
	defer windows.CloseHandle(handle)

	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(
		handle,
		processNameWin32,
		&buf[0],
		&size,
	); err != nil {
		return "", fmt.Errorf("QueryFullProcessImageName: %w", err)
	}

	return windows.UTF16ToString(buf[:size]), nil
}

func applicationNameByPID(pid int) (string, error) {
	path, err := processImagePath(pid)
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), nil
}

func applicationBundleIDByPID(pid int) (string, error) {
	path, err := processImagePath(pid)
	if err != nil {
		return "", err
	}

	return path, nil
}
