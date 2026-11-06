//go:build linux

package atspi

import (
	"image"
)

// The ax.Window and ax.Node implementations backed by AT-SPI object references.
//
// atspiWindow implements ax.Window for an AT-SPI frame.
type atspiWindow struct {
	ref   accRef
	valid bool
	// focusedAppID and focusedTitle are the focused identity captured at
	// selection (empty on X11/GNOME). ClickableNodes compares them with the live
	// values so a focus change, even to a sibling window, skips the stale frame.
	focusedAppID string
	focusedTitle string
}

func (w *atspiWindow) Release() {}

func (w *atspiWindow) Role() string { return atspiRoleFrame }

// atspiNode implements ax.Node for a clickable AT-SPI element.
type atspiNode struct {
	ref         accRef
	id          string
	role        string
	title       string
	description string
	value       string
	identifier  string
	focused     bool
	rect        image.Rectangle
}

func (n *atspiNode) ID() string { return n.id }

func (n *atspiNode) Bounds() image.Rectangle { return n.rect }

func (n *atspiNode) Role() string { return n.role }

func (n *atspiNode) Title() string { return n.title }

func (n *atspiNode) Description() string { return n.description }

func (n *atspiNode) Value() string { return n.value }

func (n *atspiNode) IsFocused() bool { return n.focused }

func (n *atspiNode) Identifier() string { return n.identifier }

func (n *atspiNode) IsClickable() bool { return true }

func (n *atspiNode) Release() {}
