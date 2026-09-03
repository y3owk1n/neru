//go:build windows

package windows

import (
	"image"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform/mousestate"
	winplatform "github.com/y3owk1n/neru/internal/adapter/platform/windows"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// windowsHeldButtons records which mouse buttons Neru is currently holding down.
var windowsHeldButtons mousestate.Tracker

// Element is a UI element for Windows.
//
// A window element carries the top-level HWND used to seed UI Automation
// enumeration. Leaf elements (discovered controls) carry pre-extracted info
// and hold no live COM reference, so Release is a no-op.
type Element struct {
	bundleIdentifier string
	pid              int
	hwnd             uintptr
	info             *ElementInfo
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
func (e *Element) Info() (*ElementInfo, error) {
	if e == nil || e.info == nil {
		return &ElementInfo{}, nil
	}

	return e.info, nil
}

// BundleIdentifier returns the bundle identifier (exe path on Windows).
func (e *Element) BundleIdentifier() string {
	if e == nil {
		return ""
	}

	return e.bundleIdentifier
}

// MenuBar returns the menu bar element (stub).
func (e *Element) MenuBar() *Element { return nil }

// IsClickable reports whether the element is a clickable control. Clickability
// is decided during UI Automation extraction (see mapControlType) and stored on
// the element info, so this just reads the cached flag.
func (e *Element) IsClickable(
	info *ElementInfo,
	_ map[string]struct{},
	_ config.Provider,
	_ bool,
) bool {
	if info != nil {
		return info.clickable
	}

	if e != nil && e.info != nil {
		return e.info.clickable
	}

	return false
}

var (
	windowsClickableRolesMu sync.RWMutex
	windowsClickableRoles   []string
)

// SetClickableRoles configures which accessibility roles are treated as clickable.
func SetClickableRoles(roles []string, _ *zap.Logger) {
	windowsClickableRolesMu.Lock()
	defer windowsClickableRolesMu.Unlock()

	windowsClickableRoles = append(windowsClickableRoles[:0:0], roles...)
}

// ClickableRoles returns the configured clickable roles.
func ClickableRoles() []string {
	windowsClickableRolesMu.RLock()
	defer windowsClickableRolesMu.RUnlock()

	return append([]string(nil), windowsClickableRoles...)
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
	clickable       bool
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

// AllWindows returns the windows of the focused application. Windows
// enumeration is limited to the foreground top-level window for now.
func AllWindows() ([]*Element, error) {
	window := FrontmostWindow()
	if window == nil {
		return []*Element{}, nil
	}

	return []*Element{window}, nil
}

// FrontmostAndPopoverWindows returns the foreground window. Popover tracking is
// not modeled on Windows; UI Automation enumerates popups under the same root.
func FrontmostAndPopoverWindows() ([]*Element, error) {
	window := FrontmostWindow()
	if window == nil {
		return []*Element{}, nil
	}

	return []*Element{window}, nil
}

// FrontmostWindow returns the foreground top-level window as an Element seeded
// with its HWND. BuildTree uses that handle to enumerate clickable controls.
func FrontmostWindow() *Element {
	hwnd, ok := winplatform.ForegroundWindowHandle()
	if !ok {
		return nil
	}

	bundleID, pid, _ := winplatform.FocusedApplicationIdentity()

	return &Element{
		bundleIdentifier: bundleID,
		pid:              pid,
		hwnd:             hwnd,
		info: &ElementInfo{
			role:      string(element.RoleWindow),
			isEnabled: true,
			pid:       pid,
		},
	}
}

// IsMouseButtonDown returns whether the given mouse button is held down.
func IsMouseButtonDown(button action.MouseButton) bool {
	return windowsHeldButtons.IsDown(button)
}

// EnsureMouseUp releases every mouse button Neru is currently holding down.
// EnsureMouseUp releases any mouse button left held.
func EnsureMouseUp() {
	for _, button := range windowsHeldButtons.HeldButtons() {
		_ = MouseUp(button)
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

// MouseDownAtPoint presses and holds the given mouse button at the point.
func MouseDownAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	err := winplatform.MouseDown(point, button)
	if err != nil {
		return err
	}

	windowsHeldButtons.SetDown(button, point, modifiers)

	return nil
}

// MouseUpAtPoint releases the given mouse button at the point.
func MouseUpAtPoint(
	point image.Point,
	button action.MouseButton,
	_ action.Modifiers,
) error {
	err := winplatform.MouseUp(point, button)
	if err != nil {
		return err
	}

	windowsHeldButtons.Clear(button)

	return nil
}

// MouseUp releases the given mouse button at the cursor.
func MouseUp(button action.MouseButton) error {
	pos, err := winplatform.CurrentCursorPosition()
	if err != nil {
		return err
	}

	return MouseUpAtPoint(pos, button, 0)
}

// ScrollAtCursor scrolls the mouse on both axes, with modifiers presented as
// held.
func ScrollAtCursor(deltaX int, deltaY int, modifiers action.Modifiers) error {
	return winplatform.ScrollWheel(deltaX, deltaY, modifiers)
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

// DetectBundleType returns "" on Windows (stub).
func DetectBundleType(_ string) string { return "" }
