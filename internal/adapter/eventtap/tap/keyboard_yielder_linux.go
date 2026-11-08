//go:build linux

package tap

// KeyboardYielder is the optional extension a Tap offers when its keyboard
// capture would catch the keys Neru types into the focused window.
//
// Only X11 needs it. An XTest key carries no origin, so while a mode holds the
// keyboard grab a key Neru types comes straight back to the tap, and vim
// mode's edits and typing never reach the app. The Wayland evdev proxy never
// grabs Neru's own devices, and macOS and Windows tag injected keys. The
// Wayland fallback that takes the keyboard through Neru's overlay has the same
// loop and cannot step aside, because a key sent through uinput or the
// compositor arrives with nothing to say when it has gone to the app. There a
// typed key is refused instead.
type KeyboardYielder interface {
	// YieldKeyboard runs send with the keyboard capture released, and takes
	// it back once send returns. With nothing captured, send runs as is. It
	// returns KeysReachApp's error without sending when that refuses.
	YieldKeyboard(send func() error) error
	// KeysReachApp reports whether a key typed now would reach the focused
	// app, without typing anything.
	KeysReachApp() error
}
