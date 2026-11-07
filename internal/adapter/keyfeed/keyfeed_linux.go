//go:build linux

package keyfeed

import (
	nativelinux "github.com/y3owk1n/neru/internal/adapter/accessibility/native/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
)

// postKey injects an already-normalized key into the focused application.
//
// X11 goes through XTest, with the event tap releasing its grab while a mode
// holds the keyboard (nativelinux.FeedKeyX11). On Wayland linux.FeedKey uses
// a uinput virtual keyboard when /dev/uinput is writable, which works on
// wlroots and KWin. Otherwise it uses zwp_virtual_keyboard_v1, which only
// wlroots-based compositors (niri, Sway, Hyprland, River) expose.
func postKey(normalized string) error {
	if !platform.DetectLinuxBackend().IsWayland() {
		return nativelinux.FeedKeyX11(normalized)
	}

	return linux.FeedKey(normalized)
}
