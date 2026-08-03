//go:build linux

package atspi

import (
	"image"
)

// The ax.Window and ax.Node implementations backed by AT-SPI object references:
// role, bounds, title, description, value, and clickability for one element.
//
// atspiWindow implements ax.Window for an AT-SPI frame.
type atspiWindow struct {
	ref   accRef
	valid bool
	// focusedAppID and focusedTitle are the compositor's focused app_id and
	// window title captured when this window was selected (empty on X11/GNOME).
	// ClickableNodes compares both against the live values so a focus change
	// between selection and the walk — including a switch to another window of
	// the same application (same app_id, different title) — is detected and the
	// stale frame is not walked or offset.
	focusedAppID string
	focusedTitle string
}

func (w *atspiWindow) Release() {}

func (w *atspiWindow) Role() string { return atspiRoleFrame }

// atspiNode implements ax.Node for a clickable AT-SPI element.
type atspiNode struct {
	id    string
	role  string
	title string
	rect  image.Rectangle
}

func (n *atspiNode) ID() string { return n.id }

func (n *atspiNode) Bounds() image.Rectangle { return n.rect }

func (n *atspiNode) Role() string { return n.role }

func (n *atspiNode) Title() string { return n.title }

func (n *atspiNode) Description() string { return "" }

func (n *atspiNode) Value() string { return "" }

func (n *atspiNode) IsClickable() bool { return true }

func (n *atspiNode) Release() {}
