//go:build linux

package eventtap

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
	evdevKeyLeftAlt    uint16 = 56
	evdevKeySpace      uint16 = 57
	evdevKeyCapsLock   uint16 = 58
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
	evdevKeyF11        uint16 = 87
	evdevKeyF12        uint16 = 88
	evdevKeyRightCtrl  uint16 = 97
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

	evdevModifierShift = "shift"
	evdevModifierCtrl  = "ctrl"
	evdevModifierAlt   = "alt"
	evdevModifierCmd   = "cmd"

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
}

type evdevModifierState struct {
	shift int
	ctrl  int
	alt   int
	cmd   int
}

func (s *evdevModifierState) update(modifier string, isDown bool) {
	delta := 1
	if !isDown {
		delta = -1
	}

	switch modifier {
	case evdevModifierShift:
		s.shift += delta
	case evdevModifierCtrl:
		s.ctrl += delta
	case evdevModifierAlt:
		s.alt += delta
	case evdevModifierCmd:
		s.cmd += delta
	}
}

func (s *evdevModifierState) prefix() string {
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
