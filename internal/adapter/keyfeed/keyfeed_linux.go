//go:build linux

package keyfeed

import (
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform/linux/inject"
	"github.com/y3owk1n/neru/internal/ports"
)

// postKey injects an already-normalized key into the focused application.
//
// X11 goes through XTest, with the event tap releasing its grab while a mode
// holds the keyboard (inject.FeedKeyX11). On Wayland linux.FeedKey uses
// a uinput virtual keyboard when /dev/uinput is writable, which works on
// wlroots and KWin. Otherwise it uses zwp_virtual_keyboard_v1, which only
// wlroots-based compositors (niri, Sway, Hyprland, River) expose.
func postKey(normalized string) error {
	if !platform.DetectLinuxBackend().IsWayland() {
		return inject.FeedKeyX11(normalized)
	}

	return inject.YieldKeyboard(func() error { return linux.FeedKey(normalized) })
}

// ReachesApp implements ports.KeyFeedGate. A key fed while the Wayland
// fallback holds the keyboard through Neru's overlay would come back to Neru.
func (a *Adapter) ReachesApp() error {
	return inject.KeysReachApp()
}

var _ ports.KeyFeedGate = (*Adapter)(nil)
