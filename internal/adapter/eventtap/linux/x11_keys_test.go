//go:build linux && cgo

package linux

import "testing"

// X11 keysym values for the navigation keys, from X11/keysym.h. The production
// lookup names them through cgo, which a test file cannot do (`go test` rejects
// cgo in in-package test files), so the protocol constants are pinned here as
// literals — they are frozen X11 protocol numbers. Untyped constants convert to
// the cgo parameter type at the call site.
const (
	x11KeysymHome     = 0xFF50 // XK_Home
	x11KeysymPageUp   = 0xFF55 // XK_Page_Up / XK_Prior
	x11KeysymPageDown = 0xFF56 // XK_Page_Down / XK_Next
	x11KeysymEnd      = 0xFF57 // XK_End
	x11KeysymInsert   = 0xFF63 // XK_Insert
)

// The keypad keysyms X11 reports while NumLock is off, pinned the same way.
const (
	x11KeysymKPHome     = 0xFF95 // XK_KP_Home
	x11KeysymKPLeft     = 0xFF96 // XK_KP_Left
	x11KeysymKPUp       = 0xFF97 // XK_KP_Up
	x11KeysymKPRight    = 0xFF98 // XK_KP_Right
	x11KeysymKPDown     = 0xFF99 // XK_KP_Down
	x11KeysymKPPageUp   = 0xFF9A // XK_KP_Page_Up / XK_KP_Prior
	x11KeysymKPPageDown = 0xFF9B // XK_KP_Page_Down / XK_KP_Next
	x11KeysymKPEnd      = 0xFF9C // XK_KP_End
	x11KeysymKPBegin    = 0xFF9D // XK_KP_Begin
	x11KeysymKPInsert   = 0xFF9E // XK_KP_Insert
	x11KeysymKPDelete   = 0xFF9F // XK_KP_Delete
	x11KeysymKP0        = 0xFFB0 // XK_KP_0
	x11KeysymKP5        = 0xFFB5 // XK_KP_5
	x11KeysymKP9        = 0xFFB9 // XK_KP_9
)

// TestX11KeyFromLookupNavigationKeys pins the navigation keys on the X11 tap.
// XLookupString yields no character for them, so the keysym lookup is the only
// path by which they reach the handler, and the name it returns has to be the
// spelling config validation accepts. The evdev backend is checked against the
// same literal so both Linux taps keep emitting the one spelling.
func TestX11KeyFromLookupNavigationKeys(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		got       string
		evdevCode uint16
		want      string
	}{
		{
			name:      "page up",
			got:       x11KeyFromLookup(0, nil, x11KeysymPageUp),
			evdevCode: evdevKeyPageUp,
			want:      "PageUp",
		},
		{
			name:      "page down",
			got:       x11KeyFromLookup(0, nil, x11KeysymPageDown),
			evdevCode: evdevKeyPageDown,
			want:      "PageDown",
		},
		{
			name:      "home",
			got:       x11KeyFromLookup(0, nil, x11KeysymHome),
			evdevCode: evdevKeyHome,
			want:      "Home",
		},
		{
			name:      "end",
			got:       x11KeyFromLookup(0, nil, x11KeysymEnd),
			evdevCode: evdevKeyEnd,
			want:      "End",
		},
		{
			name:      "insert",
			got:       x11KeyFromLookup(0, nil, x11KeysymInsert),
			evdevCode: evdevKeyInsert,
			want:      "Insert",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if testCase.got != testCase.want {
				t.Fatalf("x11KeyFromLookup() = %q, want %q", testCase.got, testCase.want)
			}

			if got := evdevKeyName(testCase.evdevCode); got != testCase.want {
				t.Fatalf("evdevKeyName(%d) = %q, want %q",
					testCase.evdevCode, got, testCase.want)
			}
		})
	}
}

// TestX11KeyFromLookupKeypadNavigationKeys pins the keypad half of the same
// vocabulary. With NumLock off the keypad reports its own KP_ keysyms, and
// XLookupString yields no character for any of them, so the keysym lookup is
// again the only path to the handler. The names are copied from the fold table
// neru_normalize_xkb_name applies to these keysyms
// (internal/adapter/platform/linux/wayland_keymap.c) — the table the Wayland
// and evdev taps both read — so this is what pins the X11 tap to it.
func TestX11KeyFromLookupKeypadNavigationKeys(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		got  string
		want string
	}{
		{name: "keypad home", got: x11KeyFromLookup(0, nil, x11KeysymKPHome), want: "Home"},
		{name: "keypad end", got: x11KeyFromLookup(0, nil, x11KeysymKPEnd), want: "End"},
		{name: "keypad page up", got: x11KeyFromLookup(0, nil, x11KeysymKPPageUp), want: "PageUp"},
		{
			name: "keypad page down",
			got:  x11KeyFromLookup(0, nil, x11KeysymKPPageDown),
			want: "PageDown",
		},
		{name: "keypad left", got: x11KeyFromLookup(0, nil, x11KeysymKPLeft), want: "Left"},
		{name: "keypad right", got: x11KeyFromLookup(0, nil, x11KeysymKPRight), want: "Right"},
		{name: "keypad up", got: x11KeyFromLookup(0, nil, x11KeysymKPUp), want: "Up"},
		{name: "keypad down", got: x11KeyFromLookup(0, nil, x11KeysymKPDown), want: "Down"},
		{name: "keypad insert", got: x11KeyFromLookup(0, nil, x11KeysymKPInsert), want: "Insert"},
		{name: "keypad delete", got: x11KeyFromLookup(0, nil, x11KeysymKPDelete), want: "Delete"},
		{name: "keypad begin", got: x11KeyFromLookup(0, nil, x11KeysymKPBegin), want: "5"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if testCase.got != testCase.want {
				t.Fatalf("x11KeyFromLookup() = %q, want %q", testCase.got, testCase.want)
			}
		})
	}
}

// TestX11KeyFromLookupKeypadDigits pins the other half of the NumLock
// question. With NumLock on the keypad reports KP_0 through KP_9, and
// XLookupString resolves those to digit characters — so the digit a person
// types comes from the character branch of x11KeyFromLookup, never from the
// keysym lookup. The lookup must therefore stay silent on them: a name here
// would let the fold speak for a key the character branch already answers.
func TestX11KeyFromLookupKeypadDigits(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		got  string
	}{
		{name: "keypad 0", got: x11KeyFromLookup(0, nil, x11KeysymKP0)},
		{name: "keypad 5", got: x11KeyFromLookup(0, nil, x11KeysymKP5)},
		{name: "keypad 9", got: x11KeyFromLookup(0, nil, x11KeysymKP9)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if testCase.got != "" {
				t.Fatalf("x11KeyFromLookup() = %q, want %q", testCase.got, "")
			}
		})
	}
}
