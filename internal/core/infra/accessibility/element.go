package accessibility

/*
#cgo CFLAGS: -x objective-c
#include "../bridge/accessibility.h"
#include <stdlib.h>
*/
import "C"

import (
	"image"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"go.uber.org/zap"
)

// Element represents a UI element in the macOS accessibility hierarchy.
type Element struct {
	ref unsafe.Pointer
}

var (
	clickableRoles   = make(map[string]struct{})
	clickableRolesMu sync.RWMutex

	isLeftMouseDown       bool
	isLeftMouseDownMu     sync.RWMutex
	lastMouseDownPosition image.Point
)

var (
	errSetFocusNil = derrors.New(
		derrors.CodeAccessibilityFailed,
		"cannot set focus: element reference is nil",
	)
	errSetFocusFailed = derrors.New(
		derrors.CodeAccessibilityFailed,
		"failed to set focus on element",
	)
	errGetChildrenNil = derrors.New(
		derrors.CodeAccessibilityFailed,
		"cannot get children: element reference is nil",
	)
	errGetAttributeNil = derrors.New(
		derrors.CodeAccessibilityFailed,
		"cannot get attribute: element reference is nil",
	)
	errGetInfoNil    = derrors.New(derrors.CodeAccessibilityFailed, "element reference is nil")
	errGetInfoFailed = derrors.New(
		derrors.CodeAccessibilityFailed,
		"failed to retrieve element info from accessibility API",
	)
)

// SetClickableRoles configures which accessibility roles are treated as clickable.
func SetClickableRoles(roles []string, logger *zap.Logger) {
	clickableRolesMu.Lock()
	defer clickableRolesMu.Unlock()

	clickableRoles = make(map[string]struct{}, len(roles))
	for _, role := range roles {
		trimmed := strings.TrimSpace(role)
		if trimmed == "" {
			continue
		}
		clickableRoles[trimmed] = struct{}{}
	}

	logger.Debug("Updated clickable roles",
		zap.Int("count", len(clickableRoles)),
		zap.Strings("roles", roles))
}

// ClickableRoles returns the configured clickable roles.
func ClickableRoles() []string {
	clickableRolesMu.RLock()
	defer clickableRolesMu.RUnlock()

	// Pre-allocate with exact capacity
	roles := make([]string, 0, len(clickableRoles))
	for role := range clickableRoles {
		roles = append(roles, role)
	}
	sort.Strings(roles)

	return roles
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
func (ei *ElementInfo) Position() image.Point {
	return ei.position
}

// Size returns the element size.
func (ei *ElementInfo) Size() image.Point {
	return ei.size
}

// Title returns the element title.
func (ei *ElementInfo) Title() string {
	return ei.title
}

// Role returns the element role.
func (ei *ElementInfo) Role() string {
	return ei.role
}

// RoleDescription returns the element role description.
func (ei *ElementInfo) RoleDescription() string {
	return ei.roleDescription
}

// IsEnabled returns whether the element is enabled.
func (ei *ElementInfo) IsEnabled() bool {
	return ei.isEnabled
}

// IsFocused returns whether the element is focused.
func (ei *ElementInfo) IsFocused() bool {
	return ei.isFocused
}

// PID returns the process ID.
func (ei *ElementInfo) PID() int {
	return ei.pid
}

// CheckAccessibilityPermissions verifies that the application has been granted accessibility permissions.
func CheckAccessibilityPermissions() bool {
	result := C.checkAccessibilityPermissions()

	return result == 1
}

// SystemWideElement returns the system-wide accessibility element representing the entire screen.
func SystemWideElement() *Element {
	ref := C.getSystemWideElement()
	if ref == nil {
		return nil
	}

	return &Element{ref: ref}
}

// FocusedApplication returns the currently focused application element.
func FocusedApplication() *Element {
	ref := C.getFocusedApplication()
	if ref == nil {
		return nil
	}

	return &Element{ref: ref}
}

// ApplicationByPID returns an application element identified by its process ID.
func ApplicationByPID(pid int) *Element {
	ref := C.getApplicationByPID(C.int(pid))
	if ref == nil {
		return nil
	}

	return &Element{ref: ref}
}

// ApplicationByBundleID returns an application element identified by its bundle identifier.
func ApplicationByBundleID(bundleID string) *Element {
	cBundle := C.CString(bundleID)
	defer C.free(unsafe.Pointer(cBundle)) //nolint:nlreturn

	ref := C.getApplicationByBundleId(cBundle)
	if ref == nil {
		return nil
	}

	return &Element{ref: ref}
}

// ElementAtPosition returns the UI element at the specified screen coordinates.
func ElementAtPosition(x, y int) *Element {
	pos := C.CGPoint{x: C.double(x), y: C.double(y)}
	ref := C.getElementAtPosition(pos)
	if ref == nil {
		return nil
	}

	return &Element{ref: ref}
}

// Info retrieves metadata and positioning information for the element.
func (e *Element) Info() (*ElementInfo, error) {
	if e.ref == nil {
		return nil, errGetInfoNil
	}

	cInfo := C.getElementInfo(e.ref) //nolint:nlreturn
	if cInfo == nil {
		return nil, errGetInfoFailed
	}
	defer C.freeElementInfo(cInfo) //nolint:nlreturn

	info := &ElementInfo{
		position: image.Point{
			X: int(cInfo.position.x),
			Y: int(cInfo.position.y),
		},
		size: image.Point{
			X: int(cInfo.size.width),
			Y: int(cInfo.size.height),
		},
		isEnabled: bool(cInfo.isEnabled),
		isFocused: bool(cInfo.isFocused),
		pid:       int(cInfo.pid),
	}

	if cInfo.title != nil {
		info.title = C.GoString(cInfo.title)
	}
	if cInfo.role != nil {
		info.role = C.GoString(cInfo.role)
	}
	if cInfo.roleDescription != nil {
		info.roleDescription = C.GoString(cInfo.roleDescription)
	}

	return info, nil
}

// Children returns all child elements of this element.
func (e *Element) Children() ([]*Element, error) {
	if e.ref == nil {
		return nil, errGetChildrenNil
	}

	var count C.int
	var rawChildren unsafe.Pointer

	var info *ElementInfo
	if globalCache != nil {
		info = globalCache.Get(e)
	}

	if info == nil {
		var err error
		info, err = e.Info()
		if err != nil {
			return nil, derrors.Wrap(
				err,
				derrors.CodeAccessibilityFailed,
				"failed to get element info",
			)
		}
		if globalCache != nil {
			globalCache.Set(e, info)
		}
	}

	if info != nil {
		switch info.Role() {
		case "AXList", "AXTable", "AXOutline":
			ptr := unsafe.Pointer(C.getVisibleRows(e.ref, &count)) //nolint:nlreturn
			if ptr != nil {
				rawChildren = ptr
			} else {
				rawChildren = unsafe.Pointer(C.getChildren(e.ref, &count)) //nolint:nlreturn
			}
		default:
			rawChildren = unsafe.Pointer(C.getChildren(e.ref, &count)) //nolint:nlreturn
		}
	}

	if rawChildren == nil || count == 0 {
		return nil, nil
	}
	defer C.free(rawChildren) //nolint:nlreturn

	countInt := int(count)
	childSlice := (*[1 << 30]unsafe.Pointer)(rawChildren)[:countInt:countInt]
	// Pre-allocate and directly create elements
	children := make([]*Element, countInt)
	for i := range children {
		children[i] = &Element{ref: childSlice[i]}
	}

	return children, nil
}

// SetFocus sets focus to the element.
func (e *Element) SetFocus() error {
	if e.ref == nil {
		return errSetFocusNil
	}

	result := C.setFocus(e.ref) //nolint:nlreturn
	if result == 0 {
		return errSetFocusFailed
	}

	return nil
}

// Attribute gets a custom attribute value.
func (e *Element) Attribute(name string) (string, error) {
	if e.ref == nil {
		return "", errGetAttributeNil
	}

	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName)) //nolint:nlreturn

	cValue := C.getElementAttribute(e.ref, cName) //nolint:nlreturn
	if cValue == nil {
		return "", derrors.Newf(
			derrors.CodeAccessibilityFailed,
			"attribute %q not found on element",
			name,
		)
	}
	defer C.freeString(cValue)

	return C.GoString(cValue), nil
}

// Release releases the element reference.
func (e *Element) Release() {
	if e.ref != nil {
		C.releaseElement(e.ref)
		e.ref = nil
	}
}

// ReleaseAll releases all elements in a slice.
func ReleaseAll(elements []*Element) {
	for _, element := range elements {
		if element != nil {
			element.Release()
		}
	}
}

// AllWindows returns all windows of the focused application.
func AllWindows() ([]*Element, error) {
	var count C.int
	windows := C.getAllWindows(&count)
	if windows == nil || count == 0 {
		return []*Element{}, nil
	}
	defer C.free(unsafe.Pointer(windows)) //nolint:nlreturn

	countInt := int(count)
	windowSlice := (*[1 << 30]unsafe.Pointer)(unsafe.Pointer(windows))[:countInt:countInt]
	result := make([]*Element, countInt)

	for index := range result {
		result[index] = &Element{ref: windowSlice[index]}
	}

	return result, nil
}

// FrontmostWindow returns the frontmost window.
func FrontmostWindow() *Element {
	ref := C.getFrontmostWindow()
	if ref == nil {
		return nil
	}

	return &Element{ref: ref}
}

// MenuBar returns the menu bar element for the given application element.
func (e *Element) MenuBar() *Element {
	if e.ref == nil {
		return nil
	}
	ref := C.getMenuBar(e.ref) //nolint:nlreturn
	if ref == nil {
		return nil
	}

	return &Element{ref: ref}
}

// ApplicationName returns the application name.
func (e *Element) ApplicationName() string {
	if e.ref == nil {
		return ""
	}

	cName := C.getApplicationName(e.ref) //nolint:nlreturn
	if cName == nil {
		return ""
	}
	defer C.freeString(cName)

	return C.GoString(cName)
}

// BundleIdentifier returns the bundle identifier.
func (e *Element) BundleIdentifier() string {
	if e.ref == nil {
		return ""
	}

	cBundleID := C.getBundleIdentifier(e.ref) //nolint:nlreturn
	if cBundleID == nil {
		return ""
	}
	defer C.freeString(cBundleID)

	return C.GoString(cBundleID)
}

// ScrollBounds returns the scroll area bounds.
func (e *Element) ScrollBounds() image.Rectangle {
	if e.ref == nil {
		return image.Rectangle{}
	}

	rect := C.getScrollBounds(e.ref) //nolint:nlreturn

	return image.Rectangle{
		Min: image.Point{
			X: int(rect.origin.x),
			Y: int(rect.origin.y),
		},
		Max: image.Point{
			X: int(rect.origin.x + rect.size.width),
			Y: int(rect.origin.y + rect.size.height),
		},
	}
}

// SetLeftMouseDown sets the left mouse button down state.
func SetLeftMouseDown(down bool, position image.Point) {
	isLeftMouseDownMu.Lock()
	defer isLeftMouseDownMu.Unlock()
	isLeftMouseDown = down
	lastMouseDownPosition = position
}

// IsLeftMouseDown returns whether the left mouse button is down.
func IsLeftMouseDown() bool {
	isLeftMouseDownMu.RLock()
	defer isLeftMouseDownMu.RUnlock()

	return isLeftMouseDown
}

// GetLastMouseDownPosition returns the last position where mouse down occurred.
func GetLastMouseDownPosition() image.Point {
	isLeftMouseDownMu.RLock()
	defer isLeftMouseDownMu.RUnlock()

	return lastMouseDownPosition
}

// ClearLeftMouseDownState clears the left mouse button down state.
func ClearLeftMouseDownState() {
	isLeftMouseDownMu.Lock()
	defer isLeftMouseDownMu.Unlock()
	isLeftMouseDown = false
}

// EnsureMouseUp ensures that if the left mouse button is down, it is released.
// This should be called before any action that is incompatible with a drag operation.
func EnsureMouseUp() {
	if IsLeftMouseDown() {
		pos := GetLastMouseDownPosition()
		_ = LeftMouseUpAtPoint(pos)
	}
}

// MoveMouseToPoint moves the cursor to a specific screen point.
// If smooth cursor movement is enabled in the configuration, it will use smooth movement.
func MoveMouseToPoint(point image.Point) {
	var eventType C.CGEventType = C.CGEventType(C.kCGEventMouseMoved)
	if IsLeftMouseDown() {
		eventType = C.CGEventType(C.kCGEventLeftMouseDragged)
	}

	config := config.Global()
	if config != nil && config.SmoothCursor.MoveMouseEnabled {
		MoveMouseToPointSmooth(
			point,
			config.SmoothCursor.Steps,
			config.SmoothCursor.Delay,
			eventType,
		)
	} else {
		pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
		C.moveMouseWithType(pos, eventType)
	}
}

// MoveMouseToPointSmooth moves the cursor smoothly to a specific screen point.
func MoveMouseToPointSmooth(end image.Point, steps, delay int, eventType C.CGEventType) {
	start := CurrentCursorPosition()
	startPos := C.CGPoint{x: C.double(start.X), y: C.double(start.Y)}
	endPos := C.CGPoint{x: C.double(end.X), y: C.double(end.Y)}
	C.moveMouseSmoothWithType(startPos, endPos, C.int(steps), C.int(delay), eventType)
}

// LeftClickAtPoint performs a left mouse click at the specified point.
func LeftClickAtPoint(point image.Point, restoreCursor bool) error {
	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	result := C.performLeftClickAtPosition(pos, C.bool(restoreCursor))
	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform left-click at position (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

// RightClickAtPoint performs a right mouse click at the specified point.
func RightClickAtPoint(point image.Point, restoreCursor bool) error {
	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	result := C.performRightClickAtPosition(pos, C.bool(restoreCursor))
	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform right-click at position (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

// MiddleClickAtPoint performs a middle mouse click at the specified point.
func MiddleClickAtPoint(point image.Point, restoreCursor bool) error {
	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	result := C.performMiddleClickAtPosition(pos, C.bool(restoreCursor))
	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform middle-click at position (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

// LeftMouseDownAtPoint performs a left mouse down action at the specified point.
func LeftMouseDownAtPoint(point image.Point) error {
	SetLeftMouseDown(true, point)
	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	result := C.performLeftMouseDownAtPosition(pos)
	if result == 0 {
		ClearLeftMouseDownState()

		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform left-mouse-down at position (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

// LeftMouseUpAtPoint performs a left mouse up action at the specified point.
func LeftMouseUpAtPoint(point image.Point) error {
	ClearLeftMouseDownState()
	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	result := C.performLeftMouseUpAtPosition(pos)
	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform left-mouse-up at position (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

// LeftMouseUp performs a left mouse up action at the current cursor position.
func LeftMouseUp() error {
	result := C.performLeftMouseUpAtCursor()
	if result == 0 {
		return derrors.New(derrors.CodeActionFailed, "failed to perform left-mouse-up at cursor")
	}

	return nil
}

// ScrollAtCursor scrolls the element at the current cursor position by the specified deltas.
func ScrollAtCursor(deltaX, deltaY int) error {
	result := C.scrollAtCursor(C.int(deltaX), C.int(deltaY))
	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to scroll at cursor with delta (%d, %d)",
			deltaX,
			deltaY,
		)
	}

	return nil
}

// CurrentCursorPosition returns the current cursor position in screen coordinates.
func CurrentCursorPosition() image.Point {
	pos := C.getCurrentCursorPosition()

	return image.Point{X: int(pos.x), Y: int(pos.y)}
}

// IsClickable checks if the element is clickable.
func (e *Element) IsClickable(info *ElementInfo) bool {
	if e.ref == nil {
		return false
	}

	config := config.Global()

	if config != nil {
		// Check if clickable check should be ignored for this app
		bundleID := e.BundleIdentifier()
		if config.ShouldIgnoreClickableCheckForApp(bundleID) {
			return true
		}
	}

	// If info is not provided, try to get it
	if info == nil {
		var infoErr error
		// Try cache first if available
		if globalCache != nil {
			info = globalCache.Get(e)
		}

		if info == nil {
			info, infoErr = e.Info()
			if infoErr != nil {
				return false
			}
			if globalCache != nil {
				globalCache.Set(e, info)
			}
		}
	}

	// First check if the role is in the clickable roles list
	clickableRolesMu.RLock()
	_, ok := clickableRoles[info.Role()]
	clickableRolesMu.RUnlock()

	if ok {
		// Also verify it actually has click action
		result := C.hasClickAction(e.ref) //nolint:nlreturn

		return result == 1
	}

	return false
}

// IsClickable checks if the element is clickable

// IsMissionControlActive checks if Mission Control is currently active.
func IsMissionControlActive() bool {
	result := C.isMissionControlActive()

	return bool(result)
}
