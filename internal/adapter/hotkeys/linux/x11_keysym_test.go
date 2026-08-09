//go:build linux && cgo

package linux

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

// X11 keysym values from X11/keysym.h. The production lookup names them through
// cgo, which a test file cannot do (`go test` rejects cgo in in-package test
// files), so the protocol constants are pinned here as literals — they are
// frozen X11 protocol numbers. Untyped constants compare against the cgo return
// type at the call site. The X11 event tap pins the same four navigation
// keysyms for the same reason in eventtap/linux/x11_keys_test.go.
const (
	x11KeysymSpace    = 0x0020 // XK_space
	x11KeysymTab      = 0xFF09 // XK_Tab
	x11KeysymReturn   = 0xFF0D // XK_Return
	x11KeysymEscape   = 0xFF1B // XK_Escape
	x11KeysymHome     = 0xFF50 // XK_Home
	x11KeysymLeft     = 0xFF51 // XK_Left
	x11KeysymUp       = 0xFF52 // XK_Up
	x11KeysymRight    = 0xFF53 // XK_Right
	x11KeysymDown     = 0xFF54 // XK_Down
	x11KeysymPageUp   = 0xFF55 // XK_Page_Up / XK_Prior
	x11KeysymPageDown = 0xFF56 // XK_Page_Down / XK_Next
	x11KeysymEnd      = 0xFF57 // XK_End
	x11KeysymF5       = 0xFFC2 // XK_F5
	x11KeysymJ        = 0x006A // XK_j
)

// TestX11KeysymFor_NavigationKeys pins the four navigation keys on the X11
// global-hotkey path. The names come from the named-key vocabulary — the
// spellings a config file writes and the taps emit — and the keysyms from the
// X11 protocol, so this fails if either side stops agreeing with the other.
// XStringToKeysym knows "Page_Up" and "Prior", not Neru's "PageUp", so these
// only resolve when the lookup maps them itself.
func TestX11KeysymFor_NavigationKeys(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		key  string
		want uint32
	}{
		{name: "page up", key: keyvocab.KeyPageUp, want: x11KeysymPageUp},
		{name: "page down", key: keyvocab.KeyPageDown, want: x11KeysymPageDown},
		{name: "home", key: keyvocab.KeyHome, want: x11KeysymHome},
		{name: "end", key: keyvocab.KeyEnd, want: x11KeysymEnd},
		{name: "lowercased page up", key: "pageup", want: x11KeysymPageUp},
		{name: "lowercased page down", key: "pagedown", want: x11KeysymPageDown},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := uint32(x11KeysymFor(testCase.key)); got != testCase.want {
				t.Fatalf("x11KeysymFor(%q) = %#x, want %#x", testCase.key, got, testCase.want)
			}
		})
	}
}

// TestX11KeysymFor_ExistingSpellings pins the keys that resolved before the
// navigation keys joined the switch. The lookup now canonicalizes through the
// named-key vocabulary rather than lowercasing in place, so every spelling a
// hotkey could already be written in has to keep landing on the same keysym —
// including the ones that resolve through XStringToKeysym rather than the
// switch (a function key, a single letter).
func TestX11KeysymFor_ExistingSpellings(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		key  string
		want uint32
	}{
		{name: "space", key: keyvocab.KeySpace, want: x11KeysymSpace},
		{name: "return", key: keyvocab.KeyReturn, want: x11KeysymReturn},
		{name: "enter means return", key: keyvocab.KeyEnter, want: x11KeysymReturn},
		{name: "tab", key: keyvocab.KeyTab, want: x11KeysymTab},
		{name: "escape", key: keyvocab.KeyEscape, want: x11KeysymEscape},
		{name: "esc shorthand", key: "esc", want: x11KeysymEscape},
		{name: "lowercased escape", key: "escape", want: x11KeysymEscape},
		{name: "up", key: keyvocab.KeyUp, want: x11KeysymUp},
		{name: "down", key: keyvocab.KeyDown, want: x11KeysymDown},
		{name: "left", key: keyvocab.KeyLeft, want: x11KeysymLeft},
		{name: "right", key: keyvocab.KeyRight, want: x11KeysymRight},
		{name: "function key", key: "F5", want: x11KeysymF5},
		{name: "single letter", key: "j", want: x11KeysymJ},
		{name: "uppercase single letter", key: "J", want: x11KeysymJ},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := uint32(x11KeysymFor(testCase.key)); got != testCase.want {
				t.Fatalf("x11KeysymFor(%q) = %#x, want %#x", testCase.key, got, testCase.want)
			}
		})
	}
}
