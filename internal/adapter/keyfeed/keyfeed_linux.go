//go:build linux

package keyfeed

import (
	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
)

// postKey injects an already-normalized key into the focused application.
//
// linux.FeedKey picks the path. It uses a uinput virtual keyboard when
// /dev/uinput is writable, which works across X11, wlroots, and KWin.
// Otherwise it uses zwp_virtual_keyboard_v1, which only wlroots-based
// compositors (niri, Sway, Hyprland, River) expose.
func postKey(normalized string) error {
	return linux.FeedKey(normalized)
}
