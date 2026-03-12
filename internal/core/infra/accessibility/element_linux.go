//go:build linux

package accessibility

import (
	"image"

	"go.uber.org/zap"
)

// Element represents a UI element for Linux (e.g., AT-SPI).
type Element struct {
	// TODO: Add Linux-specific fields (e.g., DBus path)
}

// SetClickableRoles configures which accessibility roles are treated as clickable.
func SetClickableRoles(roles []string, logger *zap.Logger) {
	// TODO: Implement for Linux
}

// ClickableRoles returns the configured clickable roles.
func ClickableRoles() []string {
	// TODO: Implement for Linux
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

func (ei *ElementInfo) Position() image.Point   { return ei.position }
func (ei *ElementInfo) Size() image.Point       { return ei.size }
func (ei *ElementInfo) Title() string           { return ei.title }
func (ei *ElementInfo) Role() string            { return ei.role }
func (ei *ElementInfo) RoleDescription() string { return ei.roleDescription }
func (ei *ElementInfo) IsEnabled() bool         { return ei.isEnabled }
func (ei *ElementInfo) IsFocused() bool         { return ei.isFocused }
func (ei *ElementInfo) PID() int                { return ei.pid }

// CheckAccessibilityPermissions verifies permissions for Linux.
func CheckAccessibilityPermissions() bool {
	// TODO: Implement for Linux
	return true
}

func SystemWideElement() *Element {
	// TODO: Implement for Linux
	return nil
}

func FocusedApplication() *Element {
	// TODO: Implement for Linux
	return nil
}

func ApplicationByPID(pid int) *Element {
	// TODO: Implement for Linux
	return nil
}

func ApplicationByBundleID(bundleID string) *Element {
	// TODO: Implement for Linux
	return nil
}

func ElementAtPosition(x, y int) *Element {
	// TODO: Implement for Linux
	return nil
}

func (e *Element) Info() (*ElementInfo, error) {
	// TODO: Implement for Linux
	return &ElementInfo{}, nil
}

func (e *Element) Children(cache *InfoCache) ([]*Element, error) {
	// TODO: Implement for Linux
	return nil, nil
}

func (e *Element) SetFocus() error {
	// TODO: Implement for Linux
	return nil
}

func (e *Element) Attribute(name string) (string, error) {
	// TODO: Implement for Linux
	return "", nil
}

func (e *Element) Release() {
	// TODO: Implement for Linux
}

func ReleaseAll(elements []*Element) {
	// TODO: Implement for Linux
}

func (e *Element) Hash() (uint64, error) {
	// TODO: Implement for Linux
	return 0, nil
}

func (e *Element) Equal(other *Element) bool {
	// TODO: Implement for Linux
	return false
}

func (e *Element) Clone() (*Element, error) {
	// TODO: Implement for Linux
	return nil, nil
}

func AllWindows() ([]*Element, error) {
	// TODO: Implement for Linux
	return nil, nil
}

func FrontmostWindow() *Element {
	// TODO: Implement for Linux
	return nil
}

func (e *Element) MenuBar() *Element {
	// TODO: Implement for Linux
	return nil
}

func (e *Element) ApplicationName() string {
	// TODO: Implement for Linux
	return ""
}

func (e *Element) BundleIdentifier() string {
	// TODO: Implement for Linux
	return ""
}

func (e *Element) ScrollBounds() image.Rectangle {
	// TODO: Implement for Linux
	return image.Rectangle{}
}

func SetLeftMouseDown(down bool, position image.Point) {}
func IsLeftMouseDown() bool                            { return false }
func GetLastMouseDownPosition() image.Point            { return image.Point{} }
func ClearLeftMouseDownState()                         {}
func EnsureMouseUp()                                   {}

func MoveMouseToPoint(point image.Point, bypassSmooth bool) {}
func LeftClickAtPoint(point image.Point, restoreCursor bool) error {
	return nil
}

func RightClickAtPoint(point image.Point, restoreCursor bool) error {
	return nil
}

func MiddleClickAtPoint(point image.Point, restoreCursor bool) error {
	return nil
}

func LeftMouseDownAtPoint(point image.Point) error {
	return nil
}

func LeftMouseUpAtPoint(point image.Point) error {
	return nil
}

func LeftMouseUp() error {
	return nil
}

func ScrollAtCursor(deltaX, deltaY int) error {
	return nil
}

func CurrentCursorPosition() image.Point {
	return image.Point{}
}

func (e *Element) IsClickable(
	info *ElementInfo,
	allowedRoles map[string]struct{},
	cache *InfoCache,
) bool {
	return false
}

func IsMissionControlActive() bool {
	return false
}
