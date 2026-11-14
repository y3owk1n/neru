package ports

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain/action"
)

// MousePort synthesizes mouse button and wheel input at the OS level: clicks,
// presses and releases at a point, and scrolls at the cursor. Implementations
// live in internal/adapter/mouse.
//
// The mode handler calls it while holding its lock, so an implementation takes
// only its own leaf locks and never calls back into the app.
//
// A platform with no injection path refuses every call, and the mouse_buttons
// and scroll entries in PlatformCapabilities must agree.
type MousePort interface {
	// PerformActionAtPoint executes a mouse action at the specified point.
	//
	// A left click while the left button is held ends that drag at the point,
	// as a left release would. Any other click first releases every held
	// button. A toggle resolves against the buttons this process holds:
	// a held button is released and a free one pressed. move_mouse and
	// move_mouse_relative move the cursor there. A scroll action is refused,
	// because scrolls go through Scroll. Every failure, a platform without
	// injection included, comes back as CodeActionFailed wrapping the reason.
	PerformActionAtPoint(
		ctx context.Context,
		actionType action.Type,
		point image.Point,
		modifiers action.Modifiers,
	) error

	// Scroll performs a scroll action at the current cursor position,
	// presenting modifiers as held while the scroll is emitted. A ctrl-modified
	// scroll zooms where an unmodified one pans. Every held button is released
	// first.
	//
	// A backend that cannot present a non-zero modifier set returns
	// derrors.CodeNotSupported rather than scrolling without it, because a scroll
	// that silently drops its ctrl looks to the user like a broken binding.
	Scroll(ctx context.Context, deltaX, deltaY int, modifiers action.Modifiers) error

	// ReleaseHeldButtons releases any mouse button this process is still
	// holding down.
	//
	// Neru can leave a button held when a drag-style action is interrupted:
	// the mode exits, the daemon is paused, or a chain bails between the
	// press and the release. Without this the button stays down at the OS
	// level and the desktop behaves as if the user were mid-drag.
	//
	// Implementations must be idempotent and safe to call when nothing is
	// held, because it runs on every mode-exit path.
	ReleaseHeldButtons(ctx context.Context) error
}
