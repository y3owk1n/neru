//go:build linux

package linux

import "strings"

const (
	evdevEventKey uint16 = 0x01

	evdevValueRelease int32 = 0
	evdevValuePress   int32 = 1
	evdevValueRepeat  int32 = 2

	evdevKeyEsc        uint16 = 1
	evdevKey1          uint16 = 2
	evdevKey2          uint16 = 3
	evdevKey3          uint16 = 4
	evdevKey4          uint16 = 5
	evdevKey5          uint16 = 6
	evdevKey6          uint16 = 7
	evdevKey7          uint16 = 8
	evdevKey8          uint16 = 9
	evdevKey9          uint16 = 10
	evdevKey0          uint16 = 11
	evdevKeyMinus      uint16 = 12
	evdevKeyEqual      uint16 = 13
	evdevKeyBackspace  uint16 = 14
	evdevKeyTab        uint16 = 15
	evdevKeyQ          uint16 = 16
	evdevKeyW          uint16 = 17
	evdevKeyE          uint16 = 18
	evdevKeyR          uint16 = 19
	evdevKeyT          uint16 = 20
	evdevKeyY          uint16 = 21
	evdevKeyU          uint16 = 22
	evdevKeyI          uint16 = 23
	evdevKeyO          uint16 = 24
	evdevKeyP          uint16 = 25
	evdevKeyLeftBrace  uint16 = 26
	evdevKeyRightBrace uint16 = 27
	evdevKeyEnter      uint16 = 28
	evdevKeyLeftCtrl   uint16 = 29
	evdevKeyA          uint16 = 30
	evdevKeyS          uint16 = 31
	evdevKeyD          uint16 = 32
	evdevKeyF          uint16 = 33
	evdevKeyG          uint16 = 34
	evdevKeyH          uint16 = 35
	evdevKeyJ          uint16 = 36
	evdevKeyK          uint16 = 37
	evdevKeyL          uint16 = 38
	evdevKeySemicolon  uint16 = 39
	evdevKeyApostrophe uint16 = 40
	evdevKeyGrave      uint16 = 41
	evdevKeyLeftShift  uint16 = 42
	evdevKeyBackslash  uint16 = 43
	evdevKeyZ          uint16 = 44
	evdevKeyX          uint16 = 45
	evdevKeyC          uint16 = 46
	evdevKeyV          uint16 = 47
	evdevKeyB          uint16 = 48
	evdevKeyN          uint16 = 49
	evdevKeyM          uint16 = 50
	evdevKeyComma      uint16 = 51
	evdevKeyDot        uint16 = 52
	evdevKeySlash      uint16 = 53
	evdevKeyRightShift uint16 = 54
	evdevKeyKPAsterisk uint16 = 55
	evdevKeyLeftAlt    uint16 = 56
	evdevKeySpace      uint16 = 57
	evdevKeyF1         uint16 = 59
	evdevKeyF2         uint16 = 60
	evdevKeyF3         uint16 = 61
	evdevKeyF4         uint16 = 62
	evdevKeyF5         uint16 = 63
	evdevKeyF6         uint16 = 64
	evdevKeyF7         uint16 = 65
	evdevKeyF8         uint16 = 66
	evdevKeyF9         uint16 = 67
	evdevKeyF10        uint16 = 68
	evdevKeyKP7        uint16 = 71
	evdevKeyKP8        uint16 = 72
	evdevKeyKP9        uint16 = 73
	evdevKeyKPMinus    uint16 = 74
	evdevKeyKP4        uint16 = 75
	evdevKeyKP5        uint16 = 76
	evdevKeyKP6        uint16 = 77
	evdevKeyKPPlus     uint16 = 78
	evdevKeyKP1        uint16 = 79
	evdevKeyKP2        uint16 = 80
	evdevKeyKP3        uint16 = 81
	evdevKeyKP0        uint16 = 82
	evdevKeyKPDot      uint16 = 83
	evdevKeyF11        uint16 = 87
	evdevKeyF12        uint16 = 88
	evdevKeyKPEnter    uint16 = 96
	evdevKeyRightCtrl  uint16 = 97
	evdevKeyKPSlash    uint16 = 98
	evdevKeyRightAlt   uint16 = 100
	evdevKeyHome       uint16 = 102
	evdevKeyUp         uint16 = 103
	evdevKeyPageUp     uint16 = 104
	evdevKeyLeft       uint16 = 105
	evdevKeyRight      uint16 = 106
	evdevKeyEnd        uint16 = 107
	evdevKeyDown       uint16 = 108
	evdevKeyPageDown   uint16 = 109
	evdevKeyInsert     uint16 = 110
	evdevKeyDelete     uint16 = 111
	evdevKeyLeftMeta   uint16 = 125
	evdevKeyRightMeta  uint16 = 126
	evdevKeyF13        uint16 = 183
	evdevKeyF14        uint16 = 184
	evdevKeyF15        uint16 = 185
	evdevKeyF16        uint16 = 186
	evdevKeyF17        uint16 = 187
	evdevKeyF18        uint16 = 188
	evdevKeyF19        uint16 = 189
	evdevKeyF20        uint16 = 190
	evdevKeyF21        uint16 = 191
	evdevKeyF22        uint16 = 192
	evdevKeyF23        uint16 = 193
	evdevKeyF24        uint16 = 194

	evdevModifierShift = "shift"
	evdevModifierCtrl  = "ctrl"
	evdevModifierAlt   = "alt"
	evdevModifierCmd   = "cmd"

	// Alias spellings accepted in configs for the canonical modifier tokens.
	evdevModifierAliasControl = "control"
	evdevModifierAliasOption  = "option"
	evdevModifierAliasSuper   = "super"

	evdevPrefixShift = "Shift+"
	evdevPrefixCtrl  = "Ctrl+"
	evdevPrefixAlt   = "Alt+"
	evdevPrefixCmd   = "Cmd+"

	evdevKeyNameReturn    = "Return"
	evdevKeyNameSpace     = "Space"
	evdevKeyNameTab       = "Tab"
	evdevKeyNameEscape    = "Escape"
	evdevKeyNameBackspace = "Backspace"
	evdevKeyNameDelete    = "Delete"
	evdevKeyNameLeft      = "Left"
	evdevKeyNameRight     = "Right"
	evdevKeyNameUp        = "Up"
	evdevKeyNameDown      = "Down"
	evdevKeyNameHome      = "Home"
	evdevKeyNameEnd       = "End"
	evdevKeyNamePageUp    = "PageUp"
	evdevKeyNamePageDown  = "PageDown"
	evdevKeyNameInsert    = "Insert"
	evdevKeyNameF1        = "F1"
	evdevKeyNameF2        = "F2"
	evdevKeyNameF3        = "F3"
	evdevKeyNameF4        = "F4"
	evdevKeyNameF5        = "F5"
	evdevKeyNameF6        = "F6"
	evdevKeyNameF7        = "F7"
	evdevKeyNameF8        = "F8"
	evdevKeyNameF9        = "F9"
	evdevKeyNameF10       = "F10"
	evdevKeyNameF11       = "F11"
	evdevKeyNameF12       = "F12"
	evdevKeyNameF13       = "F13"
	evdevKeyNameF14       = "F14"
	evdevKeyNameF15       = "F15"
	evdevKeyNameF16       = "F16"
	evdevKeyNameF17       = "F17"
	evdevKeyNameF18       = "F18"
	evdevKeyNameF19       = "F19"
	evdevKeyNameF20       = "F20"
	evdevKeyNameF21       = "F21"
	evdevKeyNameF22       = "F22"
	evdevKeyNameF23       = "F23"
	evdevKeyNameF24       = "F24"
)

var evdevModifierNames = map[uint16]string{
	evdevKeyLeftShift:  evdevModifierShift,
	evdevKeyRightShift: evdevModifierShift,
	evdevKeyLeftCtrl:   evdevModifierCtrl,
	evdevKeyRightCtrl:  evdevModifierCtrl,
	evdevKeyLeftAlt:    evdevModifierAlt,
	evdevKeyRightAlt:   evdevModifierAlt,
	evdevKeyLeftMeta:   evdevModifierCmd,
	evdevKeyRightMeta:  evdevModifierCmd,
}

// evdevKeyNames names a raw evdev key code without consulting a keyboard
// layout. It is the fallback the evdev tap drops to when xkb state creation
// fails, and the only table the global hotkey reader has; a code with no entry
// here reaches nothing.
var evdevKeyNames = map[uint16]string{
	evdevKeyA:          "a",
	evdevKeyB:          "b",
	evdevKeyC:          "c",
	evdevKeyD:          "d",
	evdevKeyE:          "e",
	evdevKeyF:          "f",
	evdevKeyG:          "g",
	evdevKeyH:          "h",
	evdevKeyI:          "i",
	evdevKeyJ:          "j",
	evdevKeyK:          "k",
	evdevKeyL:          "l",
	evdevKeyM:          "m",
	evdevKeyN:          "n",
	evdevKeyO:          "o",
	evdevKeyP:          "p",
	evdevKeyQ:          "q",
	evdevKeyR:          "r",
	evdevKeyS:          "s",
	evdevKeyT:          "t",
	evdevKeyF1:         evdevKeyNameF1,
	evdevKeyF2:         evdevKeyNameF2,
	evdevKeyF3:         evdevKeyNameF3,
	evdevKeyF4:         evdevKeyNameF4,
	evdevKeyF5:         evdevKeyNameF5,
	evdevKeyF6:         evdevKeyNameF6,
	evdevKeyF7:         evdevKeyNameF7,
	evdevKeyF8:         evdevKeyNameF8,
	evdevKeyF9:         evdevKeyNameF9,
	evdevKeyF10:        evdevKeyNameF10,
	evdevKeyF11:        evdevKeyNameF11,
	evdevKeyF12:        evdevKeyNameF12,
	evdevKeyF13:        evdevKeyNameF13,
	evdevKeyF14:        evdevKeyNameF14,
	evdevKeyF15:        evdevKeyNameF15,
	evdevKeyF16:        evdevKeyNameF16,
	evdevKeyF17:        evdevKeyNameF17,
	evdevKeyF18:        evdevKeyNameF18,
	evdevKeyF19:        evdevKeyNameF19,
	evdevKeyF20:        evdevKeyNameF20,
	evdevKeyF21:        evdevKeyNameF21,
	evdevKeyF22:        evdevKeyNameF22,
	evdevKeyF23:        evdevKeyNameF23,
	evdevKeyF24:        evdevKeyNameF24,
	evdevKeyU:          "u",
	evdevKeyV:          "v",
	evdevKeyW:          "w",
	evdevKeyX:          "x",
	evdevKeyY:          "y",
	evdevKeyZ:          "z",
	evdevKey1:          "1",
	evdevKey2:          "2",
	evdevKey3:          "3",
	evdevKey4:          "4",
	evdevKey5:          "5",
	evdevKey6:          "6",
	evdevKey7:          "7",
	evdevKey8:          "8",
	evdevKey9:          "9",
	evdevKey0:          "0",
	evdevKeyMinus:      "-",
	evdevKeyEqual:      "=",
	evdevKeyLeftBrace:  "[",
	evdevKeyRightBrace: "]",
	evdevKeyBackslash:  "\\",
	evdevKeySemicolon:  ";",
	evdevKeyApostrophe: "'",
	evdevKeyGrave:      "`",
	evdevKeyComma:      ",",
	evdevKeyDot:        ".",
	evdevKeySlash:      "/",
	evdevKeyEnter:      evdevKeyNameReturn,
	evdevKeySpace:      evdevKeyNameSpace,
	evdevKeyTab:        evdevKeyNameTab,
	evdevKeyEsc:        evdevKeyNameEscape,
	evdevKeyBackspace:  evdevKeyNameBackspace,
	evdevKeyDelete:     evdevKeyNameDelete,
	evdevKeyLeft:       evdevKeyNameLeft,
	evdevKeyRight:      evdevKeyNameRight,
	evdevKeyUp:         evdevKeyNameUp,
	evdevKeyDown:       evdevKeyNameDown,
	evdevKeyHome:       evdevKeyNameHome,
	evdevKeyEnd:        evdevKeyNameEnd,
	evdevKeyPageUp:     evdevKeyNamePageUp,
	evdevKeyPageDown:   evdevKeyNamePageDown,
	evdevKeyInsert:     evdevKeyNameInsert,

	// The keypad. Every name below is the one neru_normalize_xkb_name gives
	// the keysym this key reports with NumLock off
	// (internal/adapter/platform/linux/wayland_keymap.c) — the table the
	// Wayland tap reads and the X11 keysym lookup was copied from — so one
	// physical key reaches one binding whichever Linux tap is running.
	//
	// NumLock off is the only state this table can answer for: the kernel
	// reports the same code either way, and resolving the digit is exactly the
	// layout work this table has no keymap to do. The keys whose keysym does
	// not change with NumLock — the operators, the keypad Enter, and the center
	// key, whose KP_Begin fold is the digit it carries — are therefore right in
	// both states. The ten dual-function keys are right only with NumLock off,
	// which for the tap is the degraded half of an already-degraded path but
	// for the global hotkey reader is the only answer it has: a Home hotkey is
	// reachable from the keypad and a keypad 7 typed with NumLock on can reach
	// it. Naming them a third way rather than Wayland's would trade that for a
	// keypad no Linux backend agrees about.
	evdevKeyKPAsterisk: "*",
	evdevKeyKPMinus:    "-",
	evdevKeyKPPlus:     "+",
	evdevKeyKPSlash:    "/",
	evdevKeyKPEnter:    evdevKeyNameReturn,
	evdevKeyKP5:        "5",
	evdevKeyKP7:        evdevKeyNameHome,
	evdevKeyKP8:        evdevKeyNameUp,
	evdevKeyKP9:        evdevKeyNamePageUp,
	evdevKeyKP4:        evdevKeyNameLeft,
	evdevKeyKP6:        evdevKeyNameRight,
	evdevKeyKP1:        evdevKeyNameEnd,
	evdevKeyKP2:        evdevKeyNameDown,
	evdevKeyKP3:        evdevKeyNamePageDown,
	evdevKeyKP0:        evdevKeyNameInsert,
	evdevKeyKPDot:      evdevKeyNameDelete,
}

type evdevModifierState struct {
	linuxModifierState
}

// prefix is the modifier part of a chord name, in the fixed shift+ctrl+alt+cmd
// order every reader and every blacklist entry is canonicalized to.
//
// It sits on the shared state rather than the evdev one because both Linux
// backends spell a chord with it: the evdev tap and hotkey listener through their
// modifier refcounts, and the X11 tap through the counts it keeps from KeyPress
// and KeyRelease. One spelling is what lets a binding written once match on
// either (docs/CROSS_PLATFORM.md, "One key, one name").
func (s *linuxModifierState) prefix() string {
	if s == nil {
		return ""
	}

	var prefix strings.Builder

	if s.shift > 0 {
		prefix.WriteString(evdevPrefixShift)
	}

	if s.ctrl > 0 {
		prefix.WriteString(evdevPrefixCtrl)
	}

	if s.alt > 0 {
		prefix.WriteString(evdevPrefixAlt)
	}

	if s.cmd > 0 {
		prefix.WriteString(evdevPrefixCmd)
	}

	return prefix.String()
}

func evdevModifierName(code uint16) string {
	return evdevModifierNames[code]
}

func evdevKeyName(code uint16) string {
	return evdevKeyNames[code]
}
