//go:build linux

package app

import (
	"go.uber.org/zap"

	nativelinux "github.com/y3owk1n/neru/internal/adapter/accessibility/native/linux"
	eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
	infrasystray "github.com/y3owk1n/neru/internal/adapter/systray"
)

// The Linux tap is the only thing the factory builds, and it is what the sink
// is for; a build where the two have drifted apart should not link.
var _ tap.SyntheticModifierSink = (*eventtaplinux.EventTap)(nil)

// initializePlatformLogger is a no-op on Linux.
func initializePlatformLogger(_ *zap.Logger) {}

// registerSyntheticModifierSink hands the live tap to the X11 injection
// backend, so a modifier key Neru presses to present an action's modifiers is
// announced to the tap before it goes out. A nil tap lets go of it again, which
// the teardown of a destroyed tap owes the slot.
//
// Without it the tap sees an injected press and release as the user tapping
// that modifier, and latches a sticky modifier nobody pressed (#1484). It is
// wired here, beside the tap's construction, because the injection backend is
// reached through package-level functions with nowhere to pass it.
//
// Wayland needs none of this: its virtual-keyboard modifier path generates no
// evdev or wl_keyboard event, so nothing re-enters. It is wired on a Wayland
// session all the same, because what makes it inert there is that the only code
// announcing anything is the X11 backend, which does not run — leaving the
// compositor question where it is already answered rather than asking it twice.
func registerSyntheticModifierSink(built tap.Tap, logger *zap.Logger) {
	if built == nil {
		nativelinux.SetSyntheticModifierSink(nil)

		return
	}

	// The var above pins that the Linux tap has the method; this pins that the
	// tap the factory handed back is that one. The factory answers with the
	// cross-platform tap.Tap, so nothing here can say so at compile time.
	sink, ok := built.(tap.SyntheticModifierSink)
	if !ok {
		logger.Warn(
			"Event tap cannot disown injected modifier keys; " +
				"a modifier an action presents may register as a sticky-modifier tap",
		)

		return
	}

	nativelinux.SetSyntheticModifierSink(sink)
}

// platformQuit unblocks the systray loop so App.Quit() (tray menu or signal)
// actually stops the Linux daemon; without it systray.Run/RunHeadless keeps
// the process alive after a clean quit. Mirrors darwin/windows.
func platformQuit() {
	infrasystray.Quit()
}
