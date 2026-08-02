//go:build linux

package native

import "github.com/y3owk1n/neru/internal/adapter/accessibility/native/linux"

// The element and tree layer for this platform lives in the linuxax package.
// These aliases and forwards are the dispatch slot: the shell in client.go
// and query.go is written once against these names, and the directory the
// implementation lives in says which platform it is.

type (
	// Element is this platform's accessibility element.
	Element = linux.Element
	// ElementInfo is this platform's element attribute bundle.
	ElementInfo = linux.ElementInfo
	// TreeNode is this platform's tree node.
	// TreeNode is this platform's tree node.
	TreeNode = linux.TreeNode
	// TreeOptions is this platform's tree-walk options.
	TreeOptions = linux.TreeOptions
)

// The platform functions the client shell calls.
var (
	AllWindows                    = linux.AllWindows
	ApplicationByBundleID         = linux.ApplicationByBundleID
	ApplicationByPID              = linux.ApplicationByPID
	BuildTree                     = linux.BuildTree
	CheckAccessibilityPermissions = linux.CheckAccessibilityPermissions
	ClickableRoles                = linux.ClickableRoles
	CurrentCursorPosition         = linux.CurrentCursorPosition
	DefaultTreeOptions            = linux.DefaultTreeOptions
	DetectBundleType              = linux.DetectBundleType
	ElementAtPosition             = linux.ElementAtPosition
	FocusedApplication            = linux.FocusedApplication
	FrontmostAndPopoverWindows    = linux.FrontmostAndPopoverWindows
	FrontmostWindow               = linux.FrontmostWindow
	IsMissionControlActive        = linux.IsMissionControlActive
	IsMouseButtonDown             = linux.IsMouseButtonDown
	LeftClickAtPoint              = linux.LeftClickAtPoint
	MiddleClickAtPoint            = linux.MiddleClickAtPoint
	MouseDownAtPoint              = linux.MouseDownAtPoint
	MouseUp                       = linux.MouseUp
	MouseUpAtPoint                = linux.MouseUpAtPoint
	MoveMouseToPoint              = linux.MoveMouseToPoint
	ProcessClickableNodes         = linux.ProcessClickableNodes
	ReleaseAll                    = linux.ReleaseAll
	ReleaseTree                   = linux.ReleaseTree
	RightClickAtPoint             = linux.RightClickAtPoint
	ScrollAtCursor                = linux.ScrollAtCursor
	SetClickableRoles             = linux.SetClickableRoles
	SystemWideElement             = linux.SystemWideElement
)

// The shell calls these by their unexported names, so the dispatch slot binds
// them here rather than renaming every call site.
var (
	ensureMouseUp     = linux.EnsureMouseUp
	ReleaseTreeExcept = linux.ReleaseTreeExcept
)
