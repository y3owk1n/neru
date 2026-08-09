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
