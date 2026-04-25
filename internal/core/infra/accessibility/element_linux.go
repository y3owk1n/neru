//go:build linux

package accessibility

import (
	"image"
	"os"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/infra/eventtap"
	"github.com/y3owk1n/neru/internal/core/infra/platform"
)

// Element represents a UI element for Linux (e.g., AT-SPI).
type Element struct {
	bundleIdentifier string
	title            string
	pid              int
}

var (
	linuxMouseDown    bool
	linuxMouseDownPos image.Point
	linuxMouseDownMu  sync.RWMutex
)

// SetClickableRoles configures which accessibility roles are treated as clickable (Linux stub).
func SetClickableRoles(_ []string, _ *zap.Logger) {}

// ClickableRoles returns the configured clickable roles (Linux stub).
func ClickableRoles() []string { return nil }

// ElementInfo contains metadata and positioning information for a UI element.
type ElementInfo struct {
	position        image.Point
	size            image.Point
	title           string
	role            string
	roleDescription string
	isEnabled       bool
	isFocused       bool
	pid             int
}

// Position returns the element position.
func (ei *ElementInfo) Position() image.Point { return ei.position }

// Size returns the element size.
func (ei *ElementInfo) Size() image.Point { return ei.size }

// Title returns the element title.
func (ei *ElementInfo) Title() string { return ei.title }

// Role returns the element role.
func (ei *ElementInfo) Role() string { return ei.role }

// RoleDescription returns the element role description.
func (ei *ElementInfo) RoleDescription() string { return ei.roleDescription }

// IsEnabled returns whether the element is enabled.
func (ei *ElementInfo) IsEnabled() bool { return ei.isEnabled }

// IsFocused returns whether the element is focused.
func (ei *ElementInfo) IsFocused() bool { return ei.isFocused }

// PID returns the element's process ID.
func (ei *ElementInfo) PID() int { return ei.pid }

// CheckAccessibilityPermissions verifies permissions for Linux (Linux stub).
func CheckAccessibilityPermissions() bool { return true }

// SystemWideElement returns the system-wide element (Linux stub).
func SystemWideElement() *Element { return nil }

// FocusedApplication returns the focused application.
func FocusedApplication() *Element {
	if currentLinuxBackend() == linuxBackendWayland {
		bundleID, pid := wlrootsFocusedApplicationIdentity()
		if bundleID != "" || pid != 0 {
			return &Element{
				bundleIdentifier: bundleID,
				pid:              pid,
			}
		}

		if os.Getenv("DISPLAY") != "" {
			bundleID, pid = linuxFocusedApplicationIdentity()
			if bundleID != "" || pid != 0 {
				return &Element{
					bundleIdentifier: bundleID,
					pid:              pid,
				}
			}
		}

		return nil
	}

	bundleID, pid := linuxFocusedApplicationIdentity()
	if bundleID == "" && pid == 0 {
		return nil
	}

	return &Element{
		bundleIdentifier: bundleID,
		pid:              pid,
	}
}

// ApplicationByPID returns an application by PID.
func ApplicationByPID(pid int) *Element {
	if currentLinuxBackend() == linuxBackendWayland {
		bundleID := wlrootsApplicationBundleIdentifier(pid)
		if bundleID != "" {
			return &Element{
				bundleIdentifier: bundleID,
				pid:              pid,
			}
		}

		if os.Getenv("DISPLAY") != "" {
			bundleID = linuxApplicationBundleIdentifier(pid)
			if bundleID != "" {
				return &Element{
					bundleIdentifier: bundleID,
					pid:              pid,
				}
			}
		}

		return nil
	}

	bundleID := linuxApplicationBundleIdentifier(pid)
	if bundleID == "" {
		return nil
	}

	return &Element{
		bundleIdentifier: bundleID,
		pid:              pid,
	}
}

// ApplicationByBundleID returns an application by bundle ID.
func ApplicationByBundleID(bundleID string) *Element {
	if bundleID == "" {
		return nil
	}

	return &Element{bundleIdentifier: bundleID}
}

// ElementAtPosition returns the element at a position (Linux stub).
func ElementAtPosition(_, _ int) *Element { return nil }

// Info retrieves metadata and positioning information for the element (Linux stub).
func (e *Element) Info() (*ElementInfo, error) {
	return &ElementInfo{
		title: e.title,
		pid:   e.pid,
	}, nil
}

// Children returns the element's children (Linux stub).
func (e *Element) Children(_ *InfoCache) ([]*Element, error) { return []*Element{}, nil }

// SetFocus sets focus on the element (Linux stub).
func (e *Element) SetFocus() error { return nil }

// Attribute returns the value of the named attribute (Linux stub).
func (e *Element) Attribute(_ string) (string, error) { return "", nil }

// Release releases the element (Linux stub).
func (e *Element) Release() {}

// ReleaseAll releases all elements (Linux stub).
func ReleaseAll(_ []*Element) {}

// Hash returns a hash of the element (Linux stub).
func (e *Element) Hash() (uint64, error) { return 0, nil }

// Equal returns true if the elements are equal (Linux stub).
func (e *Element) Equal(_ *Element) bool { return false }

// Clone returns a clone of the element (Linux stub).
func (e *Element) Clone() (*Element, error) { return &Element{}, nil }

// AllWindows returns all windows (Linux stub).
func AllWindows() ([]*Element, error) { return []*Element{}, nil }

// FrontmostWindow returns the frontmost window.
func FrontmostWindow() *Element {
	if currentLinuxBackend() == linuxBackendWayland {
		bundleID, pid := wlrootsFocusedApplicationIdentity()
		if bundleID != "" || pid != 0 {
			return &Element{
				bundleIdentifier: bundleID,
				pid:              pid,
			}
		}

		if os.Getenv("DISPLAY") != "" {
			bundleID, pid = linuxFocusedApplicationIdentity()
			if bundleID != "" || pid != 0 {
				return &Element{
					bundleIdentifier: bundleID,
					pid:              pid,
				}
			}
		}

		return nil
	}

	bundleID, pid := linuxFocusedApplicationIdentity()
	if bundleID == "" && pid == 0 {
		return nil
	}

	return &Element{
		bundleIdentifier: bundleID,
		pid:              pid,
	}
}

// MenuBar returns the menu bar element (Linux stub).
func (e *Element) MenuBar() *Element { return nil }

// ApplicationName returns the application name (Linux stub).
func (e *Element) ApplicationName() string { return e.bundleIdentifier }

// BundleIdentifier returns the bundle identifier (Linux stub).
func (e *Element) BundleIdentifier() string { return e.bundleIdentifier }

// ScrollBounds returns the scroll bounds (Linux stub).
func (e *Element) ScrollBounds() image.Rectangle { return image.Rectangle{} }

// SetLeftMouseDown sets the left mouse down state.
func SetLeftMouseDown(down bool, pos image.Point) {
	linuxMouseDownMu.Lock()
	defer linuxMouseDownMu.Unlock()

	linuxMouseDown = down
	linuxMouseDownPos = pos
}

// IsLeftMouseDown returns whether the left mouse button is down.
func IsLeftMouseDown() bool {
	linuxMouseDownMu.RLock()
	defer linuxMouseDownMu.RUnlock()

	return linuxMouseDown
}

// GetLastMouseDownPosition returns the last mouse down position.
func GetLastMouseDownPosition() image.Point {
	linuxMouseDownMu.RLock()
	defer linuxMouseDownMu.RUnlock()

	return linuxMouseDownPos
}

// ClearLeftMouseDownState clears the mouse down state.
func ClearLeftMouseDownState() {
	linuxMouseDownMu.Lock()
	defer linuxMouseDownMu.Unlock()

	linuxMouseDown = false
	linuxMouseDownPos = image.Point{}
}

// EnsureMouseUp ensures the mouse is up.
func EnsureMouseUp() {
	if IsLeftMouseDown() {
		_ = LeftMouseUp()
	}
}

// MoveMouseToPoint moves the mouse.
func MoveMouseToPoint(point image.Point, _ bool) {
	if currentLinuxBackend() == linuxBackendX11 {
		_ = x11MoveMouseToPoint(point)
	} else if currentLinuxBackend() == linuxBackendWayland {
		_ = wlrootsMoveMouseToPoint(point)
	}
}

// LeftClickAtPoint performs a left click.
func LeftClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11LeftClickAtPoint(point, restoreCursor, modifiers)
	}

	if currentLinuxBackend() == linuxBackendWayland {
		return wlrootsLeftClickAtPoint(point, restoreCursor, modifiers)
	}

	return nil
}

// RightClickAtPoint performs a right click.
func RightClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11RightClickAtPoint(point, restoreCursor, modifiers)
	}

	if currentLinuxBackend() == linuxBackendWayland {
		return wlrootsRightClickAtPoint(point, restoreCursor, modifiers)
	}

	return nil
}

// MiddleClickAtPoint performs a middle click.
func MiddleClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11MiddleClickAtPoint(point, restoreCursor, modifiers)
	}

	if currentLinuxBackend() == linuxBackendWayland {
		return wlrootsMiddleClickAtPoint(point, restoreCursor, modifiers)
	}

	return nil
}

// LeftMouseDownAtPoint performs a left mouse down.
func LeftMouseDownAtPoint(point image.Point, modifiers action.Modifiers) error {
	if currentLinuxBackend() == linuxBackendX11 {
		err := x11LeftMouseDownAtPoint(point, modifiers)
		if err == nil {
			SetLeftMouseDown(true, point)
		}

		return err
	}

	if currentLinuxBackend() == linuxBackendWayland {
		err := wlrootsLeftMouseDownAtPoint(point, modifiers)
		if err == nil {
			SetLeftMouseDown(true, point)
		}

		return err
	}

	return nil
}

// LeftMouseUpAtPoint performs a left mouse up.
func LeftMouseUpAtPoint(point image.Point, modifiers action.Modifiers) error {
	if currentLinuxBackend() == linuxBackendX11 {
		err := x11LeftMouseUpAtPoint(point, modifiers)
		if err == nil {
			ClearLeftMouseDownState()
		}

		return err
	}

	if currentLinuxBackend() == linuxBackendWayland {
		err := wlrootsLeftMouseUpAtPoint(point, modifiers)
		if err == nil {
			ClearLeftMouseDownState()
		}

		return err
	}

	return nil
}

// LeftMouseUp performs a left mouse up at cursor.
func LeftMouseUp() error {
	if currentLinuxBackend() == linuxBackendX11 {
		err := x11LeftMouseUp()
		if err == nil {
			ClearLeftMouseDownState()
		}

		return err
	}

	if currentLinuxBackend() == linuxBackendWayland {
		err := wlrootsLeftMouseUp()
		if err == nil {
			ClearLeftMouseDownState()
		}

		return err
	}

	return nil
}

// ScrollAtCursor scrolls at the cursor.
func ScrollAtCursor(deltaX, deltaY int) error {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11ScrollAtCursor(deltaX, deltaY)
	}

	if currentLinuxBackend() == linuxBackendWayland {
		// Scale factor: similar to X11 backend's scaling.
		// Each uinput event scrolls ~1 line, so we scale delta to
		// get number of scroll events.
		const (
			scrollScale     = 30
			maxScrollEvents = 50
		)

		yDone, xDone := false, false

		if deltaY != 0 {
			numEvents := abs(deltaY) / scrollScale
			if numEvents == 0 {
				numEvents = 1
			}

			if numEvents > maxScrollEvents {
				numEvents = maxScrollEvents
			}

			axis := 0

			value := 1
			if deltaY < 0 {
				value = -1
			}

			err := func() error {
				for range numEvents {
					err := eventtap.ScrollDeviceScroll(axis, value)
					if err != nil {
						return err
					}
				}

				return nil
			}()
			if err == nil {
				yDone = true
			}
		}

		if deltaX != 0 {
			numEvents := abs(deltaX) / scrollScale
			if numEvents == 0 {
				numEvents = 1
			}

			if numEvents > maxScrollEvents {
				numEvents = maxScrollEvents
			}

			axis := 1

			value := 1
			if deltaX < 0 {
				value = -1
			}

			err := func() error {
				for range numEvents {
					err := eventtap.ScrollDeviceScroll(axis, value)
					if err != nil {
						return err
					}
				}

				return nil
			}()
			if err == nil {
				xDone = true
			}
		}

		if yDone && xDone {
			return nil
		}

		remainX, remainY := deltaX, deltaY
		if yDone {
			remainY = 0
		}
		if xDone {
			remainX = 0
		}

		return wlrootsScrollAtCursor(remainX, remainY)
	}

	return nil
}

// CurrentCursorPosition returns the cursor position.
func CurrentCursorPosition() image.Point {
	if currentLinuxBackend() == linuxBackendX11 {
		return x11CurrentCursorPosition()
	}

	if currentLinuxBackend() == linuxBackendWayland {
		return wlrootsCurrentCursorPosition()
	}

	return image.Point{}
}

// IsClickable checks if the element is clickable (Linux stub).
func (e *Element) IsClickable(
	_ *ElementInfo,
	_ map[string]struct{},
	_ *InfoCache,
	_ config.Provider,
) bool {
	return false
}

// IsMissionControlActive returns whether Mission Control is active (Linux stub).
func IsMissionControlActive() bool { return false }

type linuxBackend string

const (
	linuxBackendUnknown linuxBackend = "unknown"
	linuxBackendX11     linuxBackend = "x11"
	linuxBackendWayland linuxBackend = "wayland"
)

// currentLinuxBackend delegates to the canonical platform.DetectLinuxBackend
// so that compositor-family detection (GNOME, KDE, wlroots, etc.) is
// consistent across all layers.
func currentLinuxBackend() linuxBackend {
	switch platform.DetectLinuxBackend() {
	case platform.BackendX11:
		return linuxBackendX11
	case platform.BackendWaylandWlroots:
		return linuxBackendWayland
	case platform.BackendUnknown, platform.BackendWaylandGNOME,
		platform.BackendWaylandKDE, platform.BackendWaylandOther:
		return linuxBackendUnknown
	}

	return linuxBackendUnknown
}
