package ports

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// ElementDiscovery defines the interface for discovering UI elements.
type ElementDiscovery interface {
	// ClickableElements retrieves all clickable UI elements matching the filter.
	ClickableElements(ctx context.Context, filter ElementFilter) ([]*element.Element, error)
}

// RoleConfiguration defines the interface for reconfiguring which accessibility
// roles count as clickable.
//
// It is separate from ElementDiscovery so a consumer that only reads elements
// does not have to accept a mutator it will never call.
type RoleConfiguration interface {
	// UpdateClickableRoles replaces the set of roles treated as clickable.
	// Config reload calls it when hints.clickable_roles changes.
	//
	// roles are native platform role names (AXButton, push-button,
	// ControlType.Button), already resolved from the semantic vocabulary by
	// element.RoleVocabulary. Backends that do not filter by role — where the
	// tree walk decides clickability another way — accept and ignore them.
	UpdateClickableRoles(roles []string)
}

// ActionExecution defines the interface for executing actions on UI elements.
type ActionExecution interface {
	// PerformAction executes an action on the specified element.
	PerformAction(ctx context.Context, elem *element.Element, actionType action.Type) error

	// PerformActionAtPoint executes an action at the specified point.
	PerformActionAtPoint(
		ctx context.Context,
		actionType action.Type,
		point image.Point,
		modifiers action.Modifiers,
	) error

	// Scroll performs a scroll action at the current cursor position.
	Scroll(ctx context.Context, deltaX, deltaY int) error

	// ReleaseHeldButtons releases any mouse button this process is still
	// holding down.
	//
	// Neru can leave a button held when a drag-style action is interrupted —
	// the mode exits, the daemon is paused, or a chain bails between the
	// press and the release. Without this the button stays down at the OS
	// level and the desktop behaves as if the user were mid-drag.
	//
	// Implementations must be idempotent and safe to call when nothing is
	// held, because it runs on every mode-exit path.
	ReleaseHeldButtons(ctx context.Context) error
}

// ApplicationInfo defines the interface for getting application information.
type ApplicationInfo interface {
	// FocusedAppBundleID returns the platform application identifier of the
	// currently focused application. On macOS this is a bundle ID
	// (e.g. "com.apple.Safari"). On Linux this will be a desktop ID or
	// executable name; on Windows an AppUserModelID or executable path.
	FocusedAppBundleID(ctx context.Context) (string, error)

	// IsAppExcluded checks if the given application identifier is in the
	// configured exclusion list. The identifier format is platform-dependent
	// (see FocusedAppBundleID).
	IsAppExcluded(ctx context.Context, bundleID string) bool
}

// TreePriming defines the interface for readying an application's accessibility
// tree before Neru queries it.
type TreePriming interface {
	// PrimeApplication reports whether the application identified by bundleID
	// has an accessibility tree Neru can hint against, waiting briefly for one
	// to appear.
	//
	// This exists for macOS: Electron, Chromium and Gecko apps build their tree
	// asynchronously after being asked to expose one, so the first hints
	// activation after focusing such an app would otherwise find nothing.
	// Backends whose trees are eagerly available (AT-SPI, UI Automation) report
	// true immediately — this is a genuine "nothing to do", not a stub, so it
	// must not return CodeNotSupported.
	//
	// Callers may retry on false; implementations must be safe to call
	// repeatedly and off the event-tap thread.
	PrimeApplication(ctx context.Context, bundleID string) (bool, error)
}

// AccessibilityPort defines the interface for interacting with the platform
// accessibility API (AXUIElement on macOS, AT-SPI on Linux, UIA on Windows).
// Implementations handle all platform-specific bridge complexity and live in
// internal/adapter/accessibility/.
//
// This interface embeds segregated sub-interfaces to keep each concern focused.
type AccessibilityPort interface {
	HealthCheck
	ElementDiscovery
	RoleConfiguration
	ActionExecution
	ApplicationInfo
	TreePriming
}

// ElementFilter defines criteria for filtering UI elements.
type ElementFilter struct {
	// Roles specifies which accessibility roles to include.
	Roles []element.Role

	// SkipWindowElements skips querying the frontmost window for elements.
	// When true, only supplementary elements (menubar, dock, NC, etc.) are
	// collected. Used by the vision strategy where the frontmost window
	// is detected via Vision Framework instead of the AX tree.
	SkipWindowElements bool

	// MinSize specifies the minimum element size to include.
	MinSize image.Point

	// ExcludeRoles specifies roles to exclude.
	ExcludeRoles []element.Role

	// IncludeMenubar includes menubar elements.
	IncludeMenubar bool

	// AdditionalMenubarTargets specifies additional bundle IDs to scan for menubar elements.
	AdditionalMenubarTargets []string

	// IncludeDock includes dock/taskbar elements.
	// On macOS this queries com.apple.dock.
	// Platform equivalents on Linux/Windows are not yet mapped.
	IncludeDock bool

	// IncludeNotificationCenter includes notification center elements.
	// On macOS this queries com.apple.notificationcenterui.
	// Platform equivalents on Linux/Windows are not yet mapped.
	IncludeNotificationCenter bool

	// IncludeStageManager includes stage manager / window manager elements.
	// On macOS this queries com.apple.WindowManager.
	// Platform equivalents on Linux/Windows are not yet mapped.
	IncludeStageManager bool

	// IncludePIP includes Picture in Picture controls.
	// On macOS this queries com.apple.PIPAgent.
	// Platform equivalents on Linux/Windows are not yet mapped.
	IncludePIP bool

	// IncludeScreenCapture includes screen capture controls.
	// On macOS this queries com.apple.screencaptureui.
	// Platform equivalents on Linux/Windows are not yet mapped.
	IncludeScreenCapture bool

	// TitleContains filters elements whose title contains this substring (case-insensitive).
	TitleContains string

	// DescriptionContains filters elements whose description contains this substring (case-insensitive).
	DescriptionContains string

	// ValueContains filters elements whose value contains this substring (case-insensitive).
	ValueContains string

	// TextContainsList is a list of additional text substrings to match against (OR logic).
	// This is used when multiple text matches are needed - elements matching any of these
	// will be included. The first string should be set in TitleContains, DescriptionContains,
	// and ValueContains, with additional strings in this list.
	TextContainsList []string
}

// DefaultElementFilter returns a filter with sensible defaults.
func DefaultElementFilter() ElementFilter {
	return ElementFilter{
		MinSize:                   image.Point{X: 1, Y: 1},
		IncludeMenubar:            false,
		AdditionalMenubarTargets:  []string{},
		IncludeDock:               false,
		IncludeNotificationCenter: false,
		IncludeStageManager:       false,
		IncludePIP:                false,
		IncludeScreenCapture:      false,
	}
}
