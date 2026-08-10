package ports

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// AccessibilityPort is the interface for interacting with the platform
// accessibility API (AXUIElement on macOS, AT-SPI on Linux, UIA on Windows).
// Implementations handle all platform-specific bridge complexity and live in
// internal/adapter/accessibility/.
type AccessibilityPort interface {
	// Health returns nil if the component is healthy, or an error if it is not.
	Health(ctx context.Context) error

	// ClickableElements retrieves all clickable UI elements matching the filter.
	ClickableElements(ctx context.Context, filter ElementFilter) ([]*element.Element, error)

	// UpdateClickableRoles replaces the set of roles treated as clickable.
	// Config reload calls it when hints.clickable_roles changes.
	//
	// roles are native platform role names (AXButton, push-button,
	// ControlType.Button), already resolved from the semantic vocabulary by
	// element.RoleVocabulary. Backends that do not filter by role — where the
	// tree walk decides clickability another way — accept and ignore them.
	UpdateClickableRoles(roles []string)

	// PerformAction executes an action on the specified element.
	PerformAction(ctx context.Context, elem *element.Element, actionType action.Type) error

	// PerformActionAtPoint executes an action at the specified point.
	PerformActionAtPoint(
		ctx context.Context,
		actionType action.Type,
		point image.Point,
		modifiers action.Modifiers,
	) error

	// Scroll performs a scroll action at the current cursor position,
	// presenting modifiers as held while the scroll is emitted — a ctrl-modified
	// scroll zooms where an unmodified one pans.
	//
	// A backend that cannot present a non-zero modifier set returns
	// derrors.CodeNotSupported rather than scrolling without it: a scroll that
	// silently drops its ctrl looks to the user like a broken binding.
	Scroll(ctx context.Context, deltaX, deltaY int, modifiers action.Modifiers) error

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

	// FocusedAppBundleID returns the platform application identifier of the
	// currently focused application. On macOS this is a bundle ID
	// (e.g. "com.apple.Safari"). On Linux this will be a desktop ID or
	// executable name; on Windows an AppUserModelID or executable path.
	FocusedAppBundleID(ctx context.Context) (string, error)

	// IsAppExcluded checks if the given application identifier is in the
	// configured exclusion list. The identifier format is platform-dependent
	// (see FocusedAppBundleID).
	IsAppExcluded(ctx context.Context, bundleID string) bool

	// PrimeApplication reports whether bundleID has an accessibility tree to
	// hint against, waiting briefly for one. Exists for macOS, where Electron,
	// Chromium and Gecko build their tree asynchronously; eager backends
	// report true immediately (a genuine "nothing to do", never
	// CodeNotSupported). Safe to retry, off the event-tap thread.
	PrimeApplication(ctx context.Context, bundleID string) (bool, error)
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
