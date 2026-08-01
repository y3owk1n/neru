package ports

import "context"

// KeyFeedPort injects synthetic keystrokes into the focused application.
//
// This is the inverse of EventTapPort. The event tap intercepts what the user
// types so Neru can act on it; KeyFeedPort posts keys *to* whatever application
// currently has focus, as if the user had pressed them. It backs the
// `neru key <key>...` command.
//
// Implementations must report derrors.CodeNotSupported when the platform has no
// injection path, and the key_feed entry in PlatformCapabilities must agree.
type KeyFeedPort interface {
	// Feed posts a single key or chord to the focused application.
	//
	// key uses the canonical form produced by
	// config.CanonicalHotkeyForPlatform: "a", "B", "Return", "F1",
	// "Ctrl+c", "Ctrl+Shift+Space". A single uppercase letter without an
	// explicit modifier has Shift injected so it produces the uppercase
	// character.
	//
	// Returns CodeInvalidInput for an unparseable or empty key, and
	// CodeNotSupported on platforms with no injection path.
	Feed(ctx context.Context, key string) error
}
