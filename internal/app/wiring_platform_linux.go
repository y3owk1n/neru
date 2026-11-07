//go:build linux

package app

import (
	"go.uber.org/zap"

	nativelinux "github.com/y3owk1n/neru/internal/adapter/accessibility/native/linux"
	eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
	infrasystray "github.com/y3owk1n/neru/internal/adapter/systray"
)

// The Linux tap is the only tap the factory builds, so a build where it does
// not implement the sink must not link.
var (
	_ tap.SyntheticModifierSink = (*eventtaplinux.EventTap)(nil)
	_ tap.KeyboardYielder       = (*eventtaplinux.EventTap)(nil)
)

// initializePlatformLogger is a no-op on Linux.
func initializePlatformLogger(_ *zap.Logger) {}

// registerSyntheticModifierSink hands the live tap to the X11 injection
// backend, so a modifier key Neru presses is announced to the tap before it
// goes out. Otherwise the tap reads the press and release as a user tap and
// latches a sticky modifier nobody pressed (#1484). A nil tap releases the
// slot, which a destroyed tap's teardown owes. It is wired here because the
// injection backend is package-level functions with nowhere to pass it.
//
// It is wired on Wayland too, where it is inert because only the X11 backend
// announces anything. That avoids asking the compositor question twice.
func registerSyntheticModifierSink(built tap.Tap, logger *zap.Logger) {
	if built == nil {
		nativelinux.SetSyntheticModifierSink(nil)

		return
	}

	// The var above pins the Linux tap's method. This pins that the factory's
	// cross-platform tap.Tap is that tap, which no compile-time check can.
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

// registerKeyboardYielder hands the live tap's yield to X11 key feeding, so a
// key Neru types while a mode holds the keyboard reaches the focused window
// instead of coming back to the tap. A nil tap releases the slot. It is wired
// on Wayland too, where it is inert because only X11 key feeding asks.
func registerKeyboardYielder(built tap.Tap, logger *zap.Logger) {
	if built == nil {
		nativelinux.SetKeyboardYielder(nil)

		return
	}

	yielder, ok := built.(tap.KeyboardYielder)
	if !ok {
		logger.Warn(
			"Event tap cannot release its grab for typed keys, " +
				"so on X11 the keys Neru types during a mode come back to it",
		)

		return
	}

	nativelinux.SetKeyboardYielder(yielder)
}

// platformQuit unblocks the systray loop so App.Quit() (tray menu or signal)
// stops the Linux daemon. Mirrors darwin/windows.
func platformQuit() {
	infrasystray.Quit()
}
