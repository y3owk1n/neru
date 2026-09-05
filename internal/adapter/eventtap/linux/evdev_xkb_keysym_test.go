//go:build linux && cgo

package linux

import "testing"

// XKB keysyms from xkbcommon-keysyms.h, the same frozen numbers as X11's. A
// test file cannot use cgo, so they are pinned as literals; untyped constants
// convert at the call site.
const (
	xkbKeysymSpace      = 0x0020 // XKB_KEY_space
	xkbKeysymBraceLeft  = 0x007B // XKB_KEY_braceleft
	xkbKeysymSemicolon  = 0x003B // XKB_KEY_semicolon
	xkbKeysymUpperM     = 0x004D // XKB_KEY_M
	xkbKeysymSterling   = 0x00A3 // XKB_KEY_sterling
	xkbKeysymADiaeresis = 0x00E4 // XKB_KEY_adiaeresis
	xkbKeysymCyrillicA  = 0x06C1 // XKB_KEY_Cyrillic_a
	xkbKeysymBackSpace  = 0xFF08 // XKB_KEY_BackSpace
	xkbKeysymTab        = 0xFF09 // XKB_KEY_Tab
	xkbKeysymReturn     = 0xFF0D // XKB_KEY_Return
	xkbKeysymPageUp     = 0xFF55 // XKB_KEY_Prior, listed before Page_Up
	xkbKeysymDelete     = 0xFFFF // XKB_KEY_Delete
	xkbKeysymKPEnter    = 0xFF8D // XKB_KEY_KP_Enter
	xkbKeysymKPHome     = 0xFF95 // XKB_KEY_KP_Home
	xkbKeysymKPPrior    = 0xFF9A // XKB_KEY_KP_Prior, listed before KP_Page_Up
	xkbKeysymKPBegin    = 0xFF9D // XKB_KEY_KP_Begin
	xkbKeysymKPAdd      = 0xFFAB // XKB_KEY_KP_Add
	xkbKeysymKP7        = 0xFFB7 // XKB_KEY_KP_7
	xkbKeysymISOLeftTab = 0xFE20 // XKB_KEY_ISO_Left_Tab
	xkbKeysymDeadAcute  = 0xFE51 // XKB_KEY_dead_acute
	xkbKeysymNoSymbol   = 0x0000 // XKB_KEY_NoSymbol
)

// TestXkbKeysymName pins the rule the evdev and Wayland readers name a key by,
// which has to be the rule the X11 tap applies (x11BaseKeyName) or a binding
// written once matches on one Linux backend and not the other. The names here
// are the raw answer, before keyvocab.NormalizeKey folds case and named-key
// spelling on the chord.
func TestXkbKeysymName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		keysym uint32
		want   string
	}{
		{name: "punctuation is its character", keysym: xkbKeysymSemicolon, want: ";"},
		// The shifted level is a keysym of its own, and its *name* is not the
		// character: "braceleft" matched nothing when it was the answer.
		{name: "the shifted level is its character", keysym: xkbKeysymBraceLeft, want: "{"},
		{name: "a letter keeps the case the level chose", keysym: xkbKeysymUpperM, want: "M"},
		// Latin-1 and beyond: the character, as X11 answers, not "sterling".
		{name: "a Latin-1 symbol is its character", keysym: xkbKeysymSterling, want: "£"},
		{name: "an accented letter is its character", keysym: xkbKeysymADiaeresis, want: "ä"},
		{name: "a non-Latin letter is its character", keysym: xkbKeysymCyrillicA, want: "а"},
		// Shift+Tab: XKB calls the shifted Tab ISO_Left_Tab, and it is Tab.
		{name: "shifted tab is tab", keysym: xkbKeysymISOLeftTab, want: evdevKeyNameTab},
		{name: "tab", keysym: xkbKeysymTab, want: evdevKeyNameTab},
		// Space types a character, and is still the named key.
		{name: "space is named, not a blank", keysym: xkbKeysymSpace, want: "space"},
		{name: "return", keysym: xkbKeysymReturn, want: evdevKeyNameReturn},
		{name: "backspace", keysym: xkbKeysymBackSpace, want: "BackSpace"},
		{name: "delete", keysym: xkbKeysymDelete, want: "Delete"},
		// xkbcommon names this keysym Prior, not Page_Up, and the fold has to
		// be keyed by the name it answers or PageUp never reaches a binding.
		{name: "page up", keysym: xkbKeysymPageUp, want: evdevKeyNamePageUp},
		// The keypad: with NumLock off it reports navigation keysyms that fold
		// onto the main-keyboard names; with NumLock on, characters.
		{name: "keypad enter is return", keysym: xkbKeysymKPEnter, want: evdevKeyNameReturn},
		{name: "keypad home folds to home", keysym: xkbKeysymKPHome, want: evdevKeyNameHome},
		{name: "keypad page up folds to page up", keysym: xkbKeysymKPPrior, want: evdevKeyNamePageUp},
		{name: "keypad begin is its digit", keysym: xkbKeysymKPBegin, want: "5"},
		{name: "keypad add is its character", keysym: xkbKeysymKPAdd, want: "+"},
		{name: "keypad digit is its character", keysym: xkbKeysymKP7, want: "7"},
		// Nothing typed and nothing Neru names: the keysym name, which no
		// binding spells, rather than a blank that would be dropped silently.
		{name: "a dead key keeps its name", keysym: xkbKeysymDeadAcute, want: "dead_acute"},
		{name: "no symbol is no name", keysym: xkbKeysymNoSymbol, want: ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := xkbKeysymName(testCase.keysym); got != testCase.want {
				t.Errorf("xkbKeysymName(%#x) = %q, want %q", testCase.keysym, got, testCase.want)
			}
		})
	}
}
