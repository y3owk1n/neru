//go:build darwin

package native

import "github.com/y3owk1n/neru/internal/adapter/accessibility/native/darwin"

// These aliases and forwards are the dispatch slot: the shell in client.go
// and query.go is written once against these names, and the directory an
// implementation lives in says which platform it is.
type (
	// Element is this platform's accessibility element.
	Element = darwin.Element
	// ElementInfo is this platform's element attribute bundle.
	ElementInfo = darwin.ElementInfo
	// TreeNode is this platform's tree node.
	// TreeNode is this platform's tree node.
	TreeNode = darwin.TreeNode
	// TreeOptions is this platform's tree-walk options.
	TreeOptions = darwin.TreeOptions
)

// The platform functions the client shell calls.
var (
	AllWindows                    = darwin.AllWindows
	ApplicationByBundleID         = darwin.ApplicationByBundleID
	ApplicationByPID              = darwin.ApplicationByPID
	BuildTree                     = darwin.BuildTree
	CheckAccessibilityPermissions = darwin.CheckAccessibilityPermissions
	ClickableRoles                = darwin.ClickableRoles
	CurrentCursorPosition         = darwin.CurrentCursorPosition
	DefaultTreeOptions            = darwin.DefaultTreeOptions
	DetectBundleType              = darwin.DetectBundleType
	ElementAtPosition             = darwin.ElementAtPosition
	FocusedApplication            = darwin.FocusedApplication
	FrontmostAndPopoverWindows    = darwin.FrontmostAndPopoverWindows
	FrontmostWindow               = darwin.FrontmostWindow
	IsMissionControlActive        = darwin.IsMissionControlActive
	IsMouseButtonDown             = darwin.IsMouseButtonDown
	LeftClickAtPoint              = darwin.LeftClickAtPoint
	MiddleClickAtPoint            = darwin.MiddleClickAtPoint
	MouseDownAtPoint              = darwin.MouseDownAtPoint
	MouseUp                       = darwin.MouseUp
	MouseUpAtPoint                = darwin.MouseUpAtPoint
	MoveMouseToPoint              = darwin.MoveMouseToPoint
	MoveMouseToPointSmooth        = darwin.MoveMouseToPointSmooth
	PrimeApplication              = darwin.PrimeApplication
	ReleaseAll                    = darwin.ReleaseAll
	RightClickAtPoint             = darwin.RightClickAtPoint
	ScrollAtCursor                = darwin.ScrollAtCursor
	SetClickableRoles             = darwin.SetClickableRoles
	SystemWideElement             = darwin.SystemWideElement
)

// The shell calls these by their unexported names, so the dispatch slot binds
// them here rather than renaming every call site.
var (
	ensureMouseUp                 = darwin.EnsureMouseUp
	platformActiveScreenBounds    = darwin.PlatformActiveScreenBounds
	supportsSupplementaryElements = darwin.SupportsSupplementaryElements
	ReleaseTreeExcept             = darwin.ReleaseTreeExcept
)
