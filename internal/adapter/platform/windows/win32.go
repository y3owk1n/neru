//go:build windows

package windows

import (
	"errors"
	"fmt"
	"image"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ports"
)

// Low-level Win32 helpers for screen, cursor, window, and process queries.
// Does not implement ports.SystemPort; system.go delegates here.
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
	device string
	bounds image.Rectangle
}

type winPoint struct {
	x int32
	y int32
}

// dwmExtendedFrameBounds is DWMWA_EXTENDED_FRAME_BOUNDS: the frame a user
// sees, without the invisible resize borders GetWindowRect counts.
const dwmExtendedFrameBounds = 9

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	dwmapi = windows.NewLazySystemDLL("dwmapi.dll")

	procDwmGetWindowAttribute = dwmapi.NewProc("DwmGetWindowAttribute")

	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetCursorPos        = user32.NewProc("SetCursorPos")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procMonitorFromPoint    = user32.NewProc("MonitorFromPoint")
	procEnumDisplayDevicesW = user32.NewProc("EnumDisplayDevicesW")
)

var errNoMonitors = errors.New("EnumDisplayMonitors: no monitors found")

func win32Bool(ret uintptr, err error) error {
	if ret == 0 {
		if err != nil && !errors.Is(err, syscall.Errno(0)) {
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
	var position winPoint

	ret, _, err := procGetCursorPos.Call(uintptr(unsafe.Pointer(&position)))

	callErr := win32Bool(ret, err)
	if callErr != nil {
		return image.Point{}, fmt.Errorf("GetCursorPos: %w", callErr)
	}

	return image.Point{X: int(position.x), Y: int(position.y)}, nil
}

// moveCursorTo brings the cursor to point: a warp, or while a button is held
// a glide (dragGlideTo), since applications only read a drag out of
// intermediate motion. The smooth-cursor animator steps through warpCursor
// directly, as its steps already are that motion.
func moveCursorTo(point image.Point) error {
	if heldButtons.AnyDown() {
		return dragGlideTo(point)
	}

	return warpCursor(point)
}

// warpCursor is one SetCursorPos, followed while a button is held by the
// injected move that gets the pointer redrawn there (dragMotionTo). The
// redraw is best-effort: a warp the pointer still makes beats one that fails
// because SendInput was refused.
func warpCursor(point image.Point) error {
	ret, _, err := procSetCursorPos.Call(uintptr(point.X), uintptr(point.Y))

	callErr := win32Bool(ret, err)
	if callErr != nil {
		return fmt.Errorf("SetCursorPos: %w", callErr)
	}

	if heldButtons.AnyDown() {
		_ = dragMotionTo(point)
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

	callErr := win32Bool(ret, err)
	if callErr != nil {
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

var (
	// monitorEnumMu serializes enumeration passes. It is held across the whole
	// EnumDisplayMonitors call, which is what makes the single shared collector
	// below safe: Windows invokes MONITORENUMPROC synchronously, on the
	// goroutine that made the call, before the call returns.
	//
	// It is an ordinary non-reentrant mutex held across a syscall, so nothing
	// reached from the callback may enumerate again. Nothing does today
	// (getMonitorInfo and monitorFriendlyName are the only calls it makes), and
	// a future caller that did would deadlock rather than misbehave quietly.
	monitorEnumMu sync.Mutex

	// monitorEnumTarget is where collectMonitor appends, installed by the pass
	// holding monitorEnumMu. The collector lives here rather than in the
	// callback's captures because the callback is allocated once for the
	// process (monitorEnumProcPtr) and so cannot capture anything per-call.
	monitorEnumTarget *monitorEnumState

	// monitorEnumProcPtr is the MONITORENUMPROC pointer EnumDisplayMonitors is
	// handed, allocated on first use and never again.
	//
	// Go's runtime keys registered callbacks on the function value and never
	// frees a slot, and the table is a fixed cb_max = 2000 entries
	// (runtime/zcallback_windows.go). Every distinct closure passed to
	// syscall.NewCallback therefore burns one permanently. enumerateMonitors
	// runs on repeated mode activations — activeScreenBounds,
	// screenBoundsByName, screenNames and NewOverlayWindow all reach it — so a
	// per-call closure made a long session a countdown to "too many callback
	// functions", which is a runtime throw rather than a panic: unrecoverable,
	// and it takes the daemon with it.
	//
	// Hoisting it also keeps the property the closure was written for: the
	// per-call state never round-trips through the dwData uintptr, so there is
	// no uintptr-to-pointer conversion for go vet's unsafeptr check to object
	// to.
	monitorEnumProcPtr = sync.OnceValue(func() uintptr {
		return syscall.NewCallback(collectMonitor)
	})
)

// collectMonitor is the MONITORENUMPROC Windows calls once per monitor. It
// appends to whichever collector the enumeration pass installed.
//
// Returning 1 continues the enumeration: a monitor whose info cannot be read is
// dropped rather than costing the caller the monitors after it. A nil target
// cannot happen while the pass holds monitorEnumMu, and is treated the same way
// rather than dereferenced.
func collectMonitor(hMonitor uintptr, _ uintptr, _ uintptr, _ uintptr) uintptr {
	if monitorEnumTarget == nil {
		return 1
	}

	info, err := getMonitorInfo(windows.Handle(hMonitor))
	if err != nil {
		return 1
	}

	deviceName := windows.UTF16ToString(info.szDevice[:])
	monitorEnumTarget.monitors = append(monitorEnumTarget.monitors, displayMonitor{
		name:   monitorFriendlyName(deviceName),
		device: deviceName,
		bounds: rectToImage(info.rcMonitor),
	})

	return 1
}

func enumerateMonitors() ([]displayMonitor, error) {
	monitors, err := runMonitorEnumeration()
	if err != nil {
		return nil, err
	}

	if len(monitors) == 0 {
		return nil, errNoMonitors
	}

	return monitors, nil
}

// runMonitorEnumeration performs one EnumDisplayMonitors pass and returns the
// monitors it collected.
//
// Installing and clearing the shared collector is the whole reason this is its
// own function: the lock and the target have to be released together and on
// every path out, including a panic from the lazy proc lookup, and a deferred
// pair here is what keeps that from being spread across the caller.
func runMonitorEnumeration() ([]displayMonitor, error) {
	state := &monitorEnumState{}

	monitorEnumMu.Lock()

	defer func() {
		monitorEnumTarget = nil

		monitorEnumMu.Unlock()
	}()

	monitorEnumTarget = state

	ret, _, err := procEnumDisplayMonitors.Call(
		0,
		0,
		monitorEnumProcPtr(),
		0,
	)

	callErr := win32Bool(ret, err)
	if callErr != nil {
		return nil, fmt.Errorf("EnumDisplayMonitors: %w", callErr)
	}

	return state.monitors, nil
}

func activeScreenBounds() (image.Rectangle, error) {
	cursor, err := cursorPosition()
	if err == nil {
		ret, _, pointErr := procMonitorFromPoint.Call(
			packMonitorPoint(cursor),
			uintptr(monitorDefaultToNearest),
		)
		if ret != 0 {
			info, infoErr := getMonitorInfo(windows.Handle(ret))
			if infoErr == nil {
				return rectToImage(info.rcMonitor), nil
			}
		} else if pointErr != nil && !errors.Is(pointErr, syscall.Errno(0)) {
			return image.Rectangle{}, fmt.Errorf("MonitorFromPoint: %w", pointErr)
		}
	}

	monitors, err := enumerateMonitors()
	if err != nil {
		return image.Rectangle{}, err
	}

	return monitors[0].bounds, nil
}

func packMonitorPoint(point image.Point) uintptr {
	return uintptr(uint64(uint32(point.X)) | uint64(uint32(point.Y))<<32)
}

// screens is the port's view of one enumeration pass. The friendly name is
// the driver's model string, which is "Generic PnP Monitor" for most
// displays. When a name repeats, each of its screens gets its device name
// as a suffix, such as "(DISPLAY5)", so `move_monitor --name` can tell them
// apart.
func screens() ([]ports.Screen, error) {
	monitors, err := enumerateMonitors()
	if err != nil {
		return nil, err
	}

	return uniquelyNamedScreens(monitors), nil
}

// uniquelyNamedScreens applies the naming rule described on screens. It is a
// separate function so a test can run it without a display.
func uniquelyNamedScreens(monitors []displayMonitor) []ports.Screen {
	seen := make(map[string]int, len(monitors))
	for _, monitor := range monitors {
		seen[strings.ToLower(monitor.name)]++
	}

	result := make([]ports.Screen, 0, len(monitors))
	for _, monitor := range monitors {
		name := monitor.name
		if seen[strings.ToLower(name)] > 1 {
			name = name + " (" + strings.TrimPrefix(monitor.device, `\\.\`) + ")"
		}

		result = append(result, ports.Screen{Name: name, Bounds: monitor.bounds})
	}

	return result
}

func foregroundWindowHandle() (windows.HWND, error) {
	return usableWindowHandle(windows.GetForegroundWindow())
}

// usableWindowHandle rejects the handles that stand for "no application has
// focus": a null handle, a handle that is no longer a window, and the desktop.
func usableWindowHandle(hwnd windows.HWND) (windows.HWND, error) {
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

	rect, err := visibleWindowRect(hwnd)
	if err != nil {
		return image.Rectangle{}, false, err
	}

	return clipToScreen(rect)
}

// visibleWindowRect is the frame a user sees for hwnd, in physical pixels.
//
// GetWindowRect counts the invisible resize borders DWM draws around a
// window, seven or eight pixels on the left, right and bottom on a themed
// desktop, so a region placed from it overhangs the visible window on those
// sides. DwmGetWindowAttribute's extended frame bounds are the visible frame,
// so that is asked first; GetWindowRect is the answer where DWM cannot say,
// on a desktop with composition off or a window it does not manage.
func visibleWindowRect(hwnd windows.HWND) (image.Rectangle, error) {
	var rect windows.Rect

	result, _, _ := procDwmGetWindowAttribute.Call(
		uintptr(hwnd),
		dwmExtendedFrameBounds,
		uintptr(unsafe.Pointer(&rect)),
		unsafe.Sizeof(rect),
	)
	if result == 0 && rect.Right > rect.Left && rect.Bottom > rect.Top {
		return rectToImage(rect), nil
	}

	ret, _, err := procGetWindowRect.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&rect)),
	)

	callErr := win32Bool(ret, err)
	if callErr != nil {
		return image.Rectangle{}, fmt.Errorf("GetWindowRect: %w", callErr)
	}

	return rectToImage(rect), nil
}

// clipToScreen intersects a window rectangle with the virtual screen, the
// rectangle every monitor fits inside.
//
// The visible frame of a maximized window still meets its monitor's edge, and
// the GetWindowRect fallback overhangs it by the invisible resize border on
// every side. That overhang holds nothing hintable, and a screen-capture
// strategy that asked for it would be refused, because a frame that leaves the
// screen cannot be placed. The clip is to the whole desktop rather than to one
// monitor so a window dragged across a seam keeps both halves; BitBlt reads
// across the seam. It lives here, where every consumer of the focused window
// sees the same rectangle, rather than in the capture path alone. A window
// with nothing left on screen reads as no focused window.
func clipToScreen(rect image.Rectangle) (image.Rectangle, bool, error) {
	desktop, err := virtualScreenBounds()
	if err != nil {
		return image.Rectangle{}, false, err
	}

	clipped := rect.Intersect(desktop)
	if clipped.Empty() {
		return image.Rectangle{}, false, nil
	}

	return clipped, true, nil
}

// ForegroundWindowHandle returns the foreground top-level window handle for
// accessibility enumeration. The bool is false when there is no usable
// foreground window (e.g. the desktop is focused).
func ForegroundWindowHandle() (uintptr, bool) {
	hwnd, err := foregroundWindowHandle()
	if err != nil {
		return 0, false
	}

	return uintptr(hwnd), true
}

// WindowFrame returns the visible frame of hwnd clipped to the desktop, in
// physical pixels, or the empty rectangle when the window has none on screen.
func WindowFrame(hwnd uintptr) image.Rectangle {
	rect, err := visibleWindowRect(windows.HWND(hwnd))
	if err != nil {
		return image.Rectangle{}
	}

	clipped, _, _ := clipToScreen(rect)

	return clipped
}

func focusedApplicationPID() (int, error) {
	hwnd, err := foregroundWindowHandle()
	if err != nil {
		return 0, err
	}

	return windowProcessID(hwnd)
}

// windowProcessID returns the PID owning hwnd.
func windowProcessID(hwnd windows.HWND) (int, error) {
	var pid uint32

	_, err := windows.GetWindowThreadProcessId(hwnd, &pid)
	if err != nil {
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

	defer func() { _ = windows.CloseHandle(handle) }()

	buf := make([]uint16, windows.MAX_PATH)

	size := uint32(len(buf))

	err = windows.QueryFullProcessImageName(
		handle,
		processNameWin32,
		&buf[0],
		&size,
	)
	if err != nil {
		return "", fmt.Errorf("QueryFullProcessImageName: %w", err)
	}

	return windows.UTF16ToString(buf[:size]), nil
}

func applicationNameByPID(pid int) (string, error) {
	path, err := processImagePath(pid)
	if err != nil {
		return "", err
	}

	return applicationNameFromPath(path), nil
}

// applicationNameFromPath is the display name for an executable path: the
// base name without its extension.
func applicationNameFromPath(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func applicationBundleIDByPID(pid int) (string, error) {
	path, err := processImagePath(pid)
	if err != nil {
		return "", err
	}

	return path, nil
}
