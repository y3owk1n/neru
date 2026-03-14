//go:build linux

package accessibility

import (
	"image"

	"github.com/y3owk1n/neru/internal/core/domain/action"
	"go.uber.org/zap"
)

// Element represents a UI element for Linux (e.g., AT-SPI).
type Element struct{}

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

// CheckAccessibilityPermissions verifies permissions for Linux (stub).
func CheckAccessibilityPermissions() bool { return true }

// SystemWideElement returns the system-wide element (Linux stub).
func SystemWideElement() *Element { return nil }

// FocusedApplication returns the focused application (Linux stub).
func FocusedApplication() *Element { return nil }

// ApplicationByPID returns an application by PID (Linux stub).
func ApplicationByPID(_ int) *Element { return nil }

// ApplicationByBundleID returns an application by bundle ID (Linux stub).
func ApplicationByBundleID(_ string) *Element { return nil }

// ElementAtPosition returns the element at a position (Linux stub).
func ElementAtPosition(_, _ int) *Element { return nil }

// Info retrieves metadata and positioning information for the element (Linux stub).
func (e *Element) Info() (*ElementInfo, error) { return &ElementInfo{}, nil }

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

// FrontmostWindow returns the frontmost window (Linux stub).
func FrontmostWindow() *Element { return nil }

// MenuBar returns the menu bar element (Linux stub).
func (e *Element) MenuBar() *Element { return nil }

// ApplicationName returns the application name (Linux stub).
func (e *Element) ApplicationName() string { return "" }

// BundleIdentifier returns the bundle identifier (Linux stub).
func (e *Element) BundleIdentifier() string { return "" }

// ScrollBounds returns the scroll bounds (Linux stub).
func (e *Element) ScrollBounds() image.Rectangle { return image.Rectangle{} }

// SetLeftMouseDown sets the left mouse down state (Linux stub).
func SetLeftMouseDown(_ bool, _ image.Point) {}

// IsLeftMouseDown returns whether the left mouse button is down (Linux stub).
func IsLeftMouseDown() bool { return false }

// GetLastMouseDownPosition returns the last mouse down position (Linux stub).
func GetLastMouseDownPosition() image.Point { return image.Point{} }

// ClearLeftMouseDownState clears the mouse down state (Linux stub).
func ClearLeftMouseDownState() {}

// EnsureMouseUp ensures the mouse is up (Linux stub).
func EnsureMouseUp() {}

// MoveMouseToPoint moves the mouse (Linux stub).
func MoveMouseToPoint(_ image.Point, _ bool) {}

// LeftClickAtPoint performs a left click (Linux stub).
func LeftClickAtPoint(_ image.Point, _ bool, _ action.Modifiers) error { return nil }

// RightClickAtPoint performs a right click (Linux stub).
func RightClickAtPoint(_ image.Point, _ bool, _ action.Modifiers) error { return nil }

// MiddleClickAtPoint performs a middle click (Linux stub).
func MiddleClickAtPoint(_ image.Point, _ bool, _ action.Modifiers) error { return nil }

// LeftMouseDownAtPoint performs a left mouse down (Linux stub).
func LeftMouseDownAtPoint(_ image.Point, _ action.Modifiers) error { return nil }

// LeftMouseUpAtPoint performs a left mouse up (Linux stub).
func LeftMouseUpAtPoint(_ image.Point, _ action.Modifiers) error { return nil }

// LeftMouseUp performs a left mouse up at cursor (Linux stub).
func LeftMouseUp() error { return nil }

// ScrollAtCursor scrolls at the cursor (Linux stub).
func ScrollAtCursor(_, _ int) error { return nil }

// CurrentCursorPosition returns the cursor position (Linux stub).
func CurrentCursorPosition() image.Point { return image.Point{} }

// IsClickable checks if the element is clickable (Linux stub).
func (e *Element) IsClickable(
	_ *ElementInfo,
	_ map[string]struct{},
	_ *InfoCache,
) bool {
	return false
}

// IsMissionControlActive returns whether Mission Control is active (Linux stub).
func IsMissionControlActive() bool { return false }
