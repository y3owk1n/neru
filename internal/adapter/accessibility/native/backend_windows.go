//go:build windows

package native

import "github.com/y3owk1n/neru/internal/adapter/accessibility/native/windows"

// These aliases and forwards are the dispatch slot: the shell in client.go
// and query.go is written once against these names, and the directory an
// implementation lives in says which platform it is.
type (
	// Element is this platform's accessibility element.
	Element = windows.Element
	// ElementInfo is this platform's element attribute bundle.
	ElementInfo = windows.ElementInfo
	// TreeNode is this platform's tree node.
	// TreeNode is this platform's tree node.
	TreeNode = windows.TreeNode
	// TreeOptions is this platform's tree-walk options.
	TreeOptions = windows.TreeOptions
)

// The platform functions the client shell calls.
var (
	AllWindows                    = windows.AllWindows
	ApplicationByBundleID         = windows.ApplicationByBundleID
	ApplicationByPID              = windows.ApplicationByPID
	BuildTree                     = windows.BuildTree
	CheckAccessibilityPermissions = windows.CheckAccessibilityPermissions
	ClickableRoles                = windows.ClickableRoles
	CurrentCursorPosition         = windows.CurrentCursorPosition
	DefaultTreeOptions            = windows.DefaultTreeOptions
	DetectBundleType              = windows.DetectBundleType
	ElementAtPosition             = windows.ElementAtPosition
	FocusedApplication            = windows.FocusedApplication
	FrontmostAndPopoverWindows    = windows.FrontmostAndPopoverWindows
	FrontmostWindow               = windows.FrontmostWindow
	IsMissionControlActive        = windows.IsMissionControlActive
	IsMouseButtonDown             = windows.IsMouseButtonDown
	LeftClickAtPoint              = windows.LeftClickAtPoint
	MiddleClickAtPoint            = windows.MiddleClickAtPoint
	MouseDownAtPoint              = windows.MouseDownAtPoint
	MouseUp                       = windows.MouseUp
	MouseUpAtPoint                = windows.MouseUpAtPoint
	MoveMouseToPoint              = windows.MoveMouseToPoint
	ProcessClickableNodes         = windows.ProcessClickableNodes
	ReleaseTree                   = windows.ReleaseTree
	RightClickAtPoint             = windows.RightClickAtPoint
	ScrollAtCursor                = windows.ScrollAtCursor
	SetClickableRoles             = windows.SetClickableRoles
	SystemWideElement             = windows.SystemWideElement
)

// The shell calls these by their unexported names, so the dispatch slot binds
// them here rather than renaming every call site.
var (
	ensureMouseUp     = windows.EnsureMouseUp
	ReleaseTreeExcept = windows.ReleaseTreeExcept
)
