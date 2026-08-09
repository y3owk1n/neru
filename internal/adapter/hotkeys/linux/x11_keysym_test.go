//go:build linux && cgo

package linux

import (
	"strconv"
	"strings"
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
	x11KeysymSpace     = 0x0020 // XK_space
	x11KeysymBackSpace = 0xFF08 // XK_BackSpace
	x11KeysymTab       = 0xFF09 // XK_Tab
	x11KeysymReturn    = 0xFF0D // XK_Return
	x11KeysymEscape    = 0xFF1B // XK_Escape
	x11KeysymHome      = 0xFF50 // XK_Home
	x11KeysymLeft      = 0xFF51 // XK_Left
	x11KeysymUp        = 0xFF52 // XK_Up
	x11KeysymRight     = 0xFF53 // XK_Right
	x11KeysymDown      = 0xFF54 // XK_Down
	x11KeysymPageUp    = 0xFF55 // XK_Page_Up / XK_Prior
	x11KeysymPageDown  = 0xFF56 // XK_Page_Down / XK_Next
	x11KeysymEnd       = 0xFF57 // XK_End
	x11KeysymInsert    = 0xFF63 // XK_Insert
	x11KeysymF1        = 0xFFBE // XK_F1
	x11KeysymF24       = 0xFFD5 // XK_F24
	x11KeysymDelete    = 0xFFFF // XK_Delete
	x11KeysymJ         = 0x006A // XK_j
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

// TestX11KeysymFor_EditingKeys pins Backspace, Delete and Insert, and pins them
// together because the first two are the pair a hotkey can get wrong without
// noticing. X11 spells the erase-left key "BackSpace", so Neru's "Backspace"
// resolves only when the lookup maps it; and the vocabulary makes "Backspace"
// an alias of "Delete", so a lookup that folded aliases would grab X11's
// forward-delete key instead. Both spellings must reach their own keysym, and
// neither may reach the other's.
func TestX11KeysymFor_EditingKeys(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		key  string
		want uint32
	}{
		{name: "backspace", key: keyvocab.KeyBackspace, want: x11KeysymBackSpace},
		{name: "lowercased backspace", key: "backspace", want: x11KeysymBackSpace},
		{name: "delete", key: keyvocab.KeyDelete, want: x11KeysymDelete},
		{name: "lowercased delete", key: "delete", want: x11KeysymDelete},
		{name: "insert", key: keyvocab.KeyInsert, want: x11KeysymInsert},
		{name: "lowercased insert", key: "insert", want: x11KeysymInsert},
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
// switch. The function keys, the other fallback resolvers, have a test of their
// own below.
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

// TestX11KeysymFor_FunctionKeys walks the whole function-key range rather than
// sampling it. F1-F24 are the one group the lookup leaves to XStringToKeysym,
// because X11's own name for each of them is the name Neru writes; restating 24
// spellings in the switch to say so would be noise, so the claim is checked
// here instead, for every key in the range and in both the spellings a config
// file can use.
//
// XK_F1 through XK_F35 are one contiguous block in keysymdef.h, which is what
// makes the expected value computable rather than a list; the ends of Neru's
// range are pinned as literals above so a wrong base cannot go unnoticed.
func TestX11KeysymFor_FunctionKeys(t *testing.T) {
	t.Parallel()

	functionKeys := functionKeyNames(t)

	for index, key := range functionKeys {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			want := uint32(x11KeysymF1 + index)

			for _, spelling := range []string{key, strings.ToLower(key)} {
				if got := uint32(x11KeysymFor(spelling)); got != want {
					t.Fatalf("x11KeysymFor(%q) = %#x, want %#x", spelling, got, want)
				}
			}
		})
	}

	last := functionKeys[len(functionKeys)-1]
	if got := uint32(x11KeysymFor(last)); got != x11KeysymF24 {
		t.Fatalf("last function key %q = %#x, want %#x", last, got, x11KeysymF24)
	}
}

// TestX11KeysymFor_EveryNamedKeyResolves is the net under the tests above: a
// named key the vocabulary accepts but this lookup cannot resolve is a binding
// `neru config validate` calls fine and a grab then rejects with
// CodeInvalidInput, which is how "Backspace" reached a release. Adding a name to
// keyvocab without teaching X11 about it fails here rather than in someone's
// config.
func TestX11KeysymFor_EveryNamedKeyResolves(t *testing.T) {
	t.Parallel()

	for _, key := range keyvocab.NamedKeys() {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			if x11KeysymFor(key) == 0 {
				t.Fatalf("named key %q resolves to no X11 keysym", key)
			}
		})
	}
}

// functionKeyNames returns the function keys the vocabulary declares, in F1..Fn
// order, so the range is read from its one home rather than restated here.
func functionKeyNames(t *testing.T) []string {
	t.Helper()

	byNumber := make(map[int]string)
	highest := 0

	for _, key := range keyvocab.NamedKeys() {
		if !strings.HasPrefix(key, "F") {
			continue
		}

		number, err := strconv.Atoi(strings.TrimPrefix(key, "F"))
		if err != nil || number < 1 {
			continue
		}

		byNumber[number] = key
		highest = max(highest, number)
	}

	if highest == 0 {
		t.Fatal("no function keys found in the named-key vocabulary")
	}

	names := make([]string, 0, highest)

	for number := 1; number <= highest; number++ {
		key, ok := byNumber[number]
		if !ok {
			t.Fatalf("function key range has a hole at F%d", number)
		}

		names = append(names, key)
	}

	return names
}
