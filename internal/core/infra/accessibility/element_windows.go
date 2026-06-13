//go:build windows

package accessibility

import (
	"image"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	winplatform "github.com/y3owk1n/neru/internal/core/infra/platform/windows"
)

var (
	windowsMouseDownMu  sync.RWMutex
	windowsMouseDown    bool
	windowsMouseDownPos image.Point
)

// Element represents a UI element for Windows.
type Element struct {
	bundleIdentifier string
	pid              int
}

// Children returns the element's children.
func (e *Element) Children(role string) ([]*Element, error) { return nil, nil }

// Hash returns a hash of the element.
func (e *Element) Hash() (uint64, error) { return 0, nil }

// Equal returns true if the elements are equal.
func (e *Element) Equal(other *Element) bool { return false }

// Clone returns a clone of the element.
func (e *Element) Clone() (*Element, error) { return &Element{}, nil }

// Release releases the element.
func (e *Element) Release() {}

// Info retrieves metadata and positioning information for the element.
func (e *Element) Info() (*ElementInfo, error) { return &ElementInfo{}, nil }

// BundleIdentifier returns the bundle identifier (exe path on Windows).
func (e *Element) BundleIdentifier() string {
	if e == nil {
		return ""
	}

	return e.bundleIdentifier
}

// MenuBar returns the menu bar element (stub).
func (e *Element) MenuBar() *Element { return nil }

// IsClickable checks if the element is clickable (stub).
func (e *Element) IsClickable(
	_ *ElementInfo,
	_ map[string]struct{},
	_ config.Provider,
	_ bool,
) bool {
	return false
}

// SetClickableRoles configures which accessibility roles are treated as clickable.
func SetClickableRoles(roles []string, logger *zap.Logger) {}

// ClickableRoles returns the configured clickable roles.
func ClickableRoles() []string {
	return nil
}

// ElementInfo contains metadata and positioning information for a UI element.
type ElementInfo struct {
	position        image.Point
	size            image.Point
	title           string
	description     string
	value           string
	searchText      string
	role            string
	subrole         string
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

// Description returns the element description.
func (ei *ElementInfo) Description() string { return ei.description }

// Value returns the element value.
func (ei *ElementInfo) Value() string { return ei.value }

// SearchText returns extra searchable text collected from descendant elements.
func (ei *ElementInfo) SearchText() string { return ei.searchText }

// Role returns the element role.
func (ei *ElementInfo) Role() string { return ei.role }

// Subrole returns the element subrole.
func (ei *ElementInfo) Subrole() string { return ei.subrole }

// RoleDescription returns the element role description.
func (ei *ElementInfo) RoleDescription() string { return ei.roleDescription }

// IsEnabled returns whether the element is enabled.
func (ei *ElementInfo) IsEnabled() bool { return ei.isEnabled }

// IsFocused returns whether the element is focused.
func (ei *ElementInfo) IsFocused() bool { return ei.isFocused }

// PID returns the element's process ID.
func (ei *ElementInfo) PID() int { return ei.pid }

// CheckAccessibilityPermissions verifies permissions for Windows (stub).
func CheckAccessibilityPermissions() bool {
	return true
}

// SystemWideElement returns the system-wide element (stub).
func SystemWideElement() *Element { return nil }

// FocusedApplication returns the focused application via Win32 foreground window APIs.
func FocusedApplication() *Element {
	bundleID, pid, err := winplatform.FocusedApplicationIdentity()
	if err != nil || (bundleID == "" && pid == 0) {
		return nil
	}

	return &Element{
		bundleIdentifier: bundleID,
		pid:              pid,
	}
}

// ApplicationByPID returns an application by PID (stub).
func ApplicationByPID(pid int) *Element { return nil }

// ApplicationByBundleID returns an application by bundle ID (stub).
func ApplicationByBundleID(bundleID string) *Element { return nil }

// ElementAtPosition returns the element at a position (stub).
func ElementAtPosition(x, y int) *Element { return nil }

// AllWindows returns all windows (stub).
func AllWindows() ([]*Element, error) { return []*Element{}, nil }

// FrontmostAndPopoverWindows returns frontmost/popover windows (Windows stub).
func FrontmostAndPopoverWindows() ([]*Element, error) { return []*Element{}, nil }

// FrontmostWindow returns the frontmost window (stub).
func FrontmostWindow() *Element { return nil }

// SetLeftMouseDown sets the left mouse down state.
func SetLeftMouseDown(down bool, position image.Point) {
	windowsMouseDownMu.Lock()
	defer windowsMouseDownMu.Unlock()

	windowsMouseDown = down
	windowsMouseDownPos = position
}

// IsLeftMouseDown returns whether the left mouse button is down.
func IsLeftMouseDown() bool {
	windowsMouseDownMu.RLock()
	defer windowsMouseDownMu.RUnlock()

	return windowsMouseDown
}

// GetLastMouseDownPosition returns the last mouse down position.
func GetLastMouseDownPosition() image.Point {
	windowsMouseDownMu.RLock()
	defer windowsMouseDownMu.RUnlock()

	return windowsMouseDownPos
}

// ClearLeftMouseDownState clears the mouse down state.
func ClearLeftMouseDownState() {
	windowsMouseDownMu.Lock()
	defer windowsMouseDownMu.Unlock()

	windowsMouseDown = false
	windowsMouseDownPos = image.Point{}
}

// EnsureMouseUp ensures the mouse is up.
func EnsureMouseUp() {
	if IsLeftMouseDown() {
		_ = LeftMouseUp()
	}
}

// MoveMouseToPoint moves the mouse.
func MoveMouseToPoint(point image.Point, _ bool) {
	_ = winplatform.MoveMouseTo(point)
}

// LeftClickAtPoint clicks the mouse.
func LeftClickAtPoint(
	point image.Point,
	_ bool,
	_ action.Modifiers,
) error {
	return winplatform.LeftClickAt(point)
}

// RightClickAtPoint clicks the mouse.
func RightClickAtPoint(
	point image.Point,
	_ bool,
	_ action.Modifiers,
) error {
	return winplatform.RightClickAt(point)
}

// MiddleClickAtPoint clicks the mouse.
func MiddleClickAtPoint(
	point image.Point,
	_ bool,
	_ action.Modifiers,
) error {
	return winplatform.MiddleClickAt(point)
}

// LeftMouseDownAtPoint presses the mouse.
func LeftMouseDownAtPoint(point image.Point, _ action.Modifiers) error {
	if err := winplatform.LeftMouseDown(point); err != nil {
		return err
	}

	SetLeftMouseDown(true, point)

	return nil
}

// LeftMouseUpAtPoint releases the mouse.
func LeftMouseUpAtPoint(point image.Point, _ action.Modifiers) error {
	if err := winplatform.LeftMouseUp(point); err != nil {
		return err
	}

	ClearLeftMouseDownState()

	return nil
}

// LeftMouseUp releases the mouse.
func LeftMouseUp() error {
	pos, err := winplatform.CurrentCursorPosition()
	if err != nil {
		return err
	}

	return LeftMouseUpAtPoint(pos, 0)
}

// ScrollAtCursor scrolls the mouse.
func ScrollAtCursor(_ int, deltaY int) error {
	return winplatform.ScrollWheel(deltaY)
}

// CurrentCursorPosition returns the cursor position.
func CurrentCursorPosition() image.Point {
	pos, err := winplatform.CurrentCursorPosition()
	if err != nil {
		return image.Point{}
	}

	return pos
}

// IsMissionControlActive returns whether Mission Control is active (stub).
func IsMissionControlActive() bool { return false }
