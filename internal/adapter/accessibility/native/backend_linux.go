//go:build linux

package native

import "github.com/y3owk1n/neru/internal/adapter/accessibility/native/linux"

// These aliases and forwards are the dispatch slot. The shell in client.go
// and query.go is written once against these names, and the directory an
// implementation lives in says which platform it is.
type (
	// Element is this platform's accessibility element.
	Element = linux.Element
	// ElementInfo is this platform's element attribute bundle.
	ElementInfo = linux.ElementInfo
	// TreeNode is this platform's tree node.
	TreeNode = linux.TreeNode
	// TreeOptions is this platform's tree-walk options.
	TreeOptions = linux.TreeOptions
)

// The platform functions the client shell calls.
var (
	ApplicationByBundleID         = linux.ApplicationByBundleID
	BuildTree                     = linux.BuildTree
	CheckAccessibilityPermissions = linux.CheckAccessibilityPermissions
	ClickableRoles                = linux.ClickableRoles
	DefaultTreeOptions            = linux.DefaultTreeOptions
	FocusedApplication            = linux.FocusedApplication
	FrontmostAndPopoverWindows    = linux.FrontmostAndPopoverWindows
	FrontmostWindow               = linux.FrontmostWindow
	IsMissionControlActive        = linux.IsMissionControlActive
	SetClickableRoles             = linux.SetClickableRoles
)

// The shell calls these by their unexported names, so the dispatch slot binds
// them here rather than renaming every call site.
var (
	ReleaseTreeExcept = linux.ReleaseTreeExcept
)

// MissionControlClickableElements is the platform's Mission Control collector.
var MissionControlClickableElements = linux.MissionControlClickableElements
