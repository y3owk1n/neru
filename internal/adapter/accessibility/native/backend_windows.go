//go:build windows

package native

import "github.com/y3owk1n/neru/internal/adapter/accessibility/native/windows"

// These aliases and forwards are the dispatch slot. The shell in client.go
// and query.go is written once against these names, and the directory an
// implementation lives in says which platform it is.
type (
	// Element is this platform's accessibility element.
	Element = windows.Element
	// ElementInfo is this platform's element attribute bundle.
	ElementInfo = windows.ElementInfo
	// TreeNode is this platform's tree node.
	TreeNode = windows.TreeNode
	// TreeOptions is this platform's tree-walk options.
	TreeOptions = windows.TreeOptions
)

// The platform functions the client shell calls.
var (
	ApplicationByBundleID         = windows.ApplicationByBundleID
	BuildTree                     = windows.BuildTree
	CheckAccessibilityPermissions = windows.CheckAccessibilityPermissions
	ClickableRoles                = windows.ClickableRoles
	DefaultTreeOptions            = windows.DefaultTreeOptions
	FocusedApplication            = windows.FocusedApplication
	FrontmostAndPopoverWindows    = windows.FrontmostAndPopoverWindows
	FrontmostWindow               = windows.FrontmostWindow
	IsMissionControlActive        = windows.IsMissionControlActive
	SetClickableRoles             = windows.SetClickableRoles
)

// The shell calls these by their unexported names, so the dispatch slot binds
// them here rather than renaming every call site.
var (
	ReleaseTreeExcept = windows.ReleaseTreeExcept
)

// MissionControlClickableElements is the platform's Mission Control collector.
var MissionControlClickableElements = windows.MissionControlClickableElements
