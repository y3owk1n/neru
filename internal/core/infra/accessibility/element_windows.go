//go:build windows

package accessibility

import (
	"image"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/action"
)

// Element represents a UI element for Windows (stub).
type Element struct{}

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

// BundleIdentifier returns the bundle identifier (stub).
func (e *Element) BundleIdentifier() string { return "" }

// MenuBar returns the menu bar element (stub).
func (e *Element) MenuBar() *Element { return nil }

// IsClickable checks if the element is clickable (stub).
func (e *Element) IsClickable(
	_ *ElementInfo,
	_ map[string]struct{},
	_ *InfoCache,
	_ config.Provider,
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

// CheckAccessibilityPermissions verifies permissions for Windows (stub).
func CheckAccessibilityPermissions() bool {
	return true
}

// SystemWideElement returns the system-wide element (stub).
func SystemWideElement() *Element { return nil }

// FocusedApplication returns the focused application (stub).
func FocusedApplication() *Element { return nil }

// ApplicationByPID returns an application by PID (stub).
func ApplicationByPID(pid int) *Element { return nil }

// ApplicationByBundleID returns an application by bundle ID (stub).
func ApplicationByBundleID(bundleID string) *Element { return nil }

// ElementAtPosition returns the element at a position (stub).
func ElementAtPosition(x, y int) *Element { return nil }

// AllWindows returns all windows (stub).
func AllWindows() ([]*Element, error) { return []*Element{}, nil }

// FrontmostWindow returns the frontmost window (stub).
func FrontmostWindow() *Element { return nil }

// SetLeftMouseDown sets the left mouse down state (stub).
func SetLeftMouseDown(down bool, position image.Point) {}

// IsLeftMouseDown returns whether the left mouse button is down (stub).
func IsLeftMouseDown() bool { return false }

// GetLastMouseDownPosition returns the last mouse down position (stub).
func GetLastMouseDownPosition() image.Point { return image.Point{} }

// ClearLeftMouseDownState clears the mouse down state (stub).
func ClearLeftMouseDownState() {}

// EnsureMouseUp ensures the mouse is up (stub).
func EnsureMouseUp() {}

// MoveMouseToPoint moves the mouse (stub).
func MoveMouseToPoint(point image.Point, bypassSmooth bool) {}

// LeftClickAtPoint clicks the mouse (stub).
func LeftClickAtPoint(
	point image.Point,
	restoreCursor bool,
	_ action.Modifiers,
) error {
	return nil
}

// RightClickAtPoint clicks the mouse (stub).
func RightClickAtPoint(
	point image.Point,
	restoreCursor bool,
	_ action.Modifiers,
) error {
	return nil
}

// MiddleClickAtPoint clicks the mouse (stub).
func MiddleClickAtPoint(
	point image.Point,
	restoreCursor bool,
	_ action.Modifiers,
) error {
	return nil
}

// LeftMouseDownAtPoint presses the mouse (stub).
func LeftMouseDownAtPoint(point image.Point, _ action.Modifiers) error { return nil }

// LeftMouseUpAtPoint releases the mouse (stub).
func LeftMouseUpAtPoint(point image.Point, _ action.Modifiers) error { return nil }

// LeftMouseUp releases the mouse (stub).
func LeftMouseUp() error { return nil }

// ScrollAtCursor scrolls the mouse (stub).
func ScrollAtCursor(deltaX, deltaY int) error { return nil }

// CurrentCursorPosition returns the cursor position (stub).
func CurrentCursorPosition() image.Point { return image.Point{} }

// IsMissionControlActive returns whether Mission Control is active (stub).
func IsMissionControlActive() bool { return false }
