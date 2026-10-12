package config

import "github.com/y3owk1n/neru/internal/domain/parity"

// ScreenShareHideOption is the option that hides the overlay from a screen
// share. toggle-screen-share switches the same state at runtime.
const ScreenShareHideOption = "general.hide_overlay_in_screen_share"

// HidesFromScreenShare reports whether hiding the overlay from a screen share
// does anything on platform, as the option's platform column declares. Every
// path that sets the hidden state asks this first, so the state, its event
// and the on_screen_share_hide hook only change where something hides
// the overlay.
func HidesFromScreenShare(platform parity.Platform) bool {
	word, _ := PlatformSupport().Lookup(parity.KindOption, ScreenShareHideOption, "")

	return word.Platforms.Supports(platform)
}

// HidesFromScreenShareHere is HidesFromScreenShare for the platform Neru runs
// on, and false on one Neru declares nothing for.
func HidesFromScreenShareHere() bool {
	platform, known := parity.Current()

	return known && HidesFromScreenShare(platform)
}

// ScreenShareHideNote says why hiding the overlay from a screen share does
// nothing outside macOS, in the words the platform warning uses.
const ScreenShareHideNote = noteScreenShareHide
