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
)

type evdevModifierState struct {
	shift bool
	ctrl  bool
	alt   bool
	cmd   bool
}

func (s *evdevModifierState) update(modifier string, isDown bool) {
	switch modifier {
	case "shift":
		s.shift = isDown
	case "ctrl":
		s.ctrl = isDown
	case "alt":
		s.alt = isDown
	case "cmd":
		s.cmd = isDown
	}
}

func (s evdevModifierState) prefix() string {
	var prefix strings.Builder

	if s.shift {
		prefix.WriteString("Shift+")
	}
	if s.ctrl {
		prefix.WriteString("Ctrl+")
	}
	if s.alt {
		prefix.WriteString("Alt+")
	}
	if s.cmd {
		prefix.WriteString("Cmd+")
	}

	return prefix.String()
}

func evdevModifierName(code uint16) string {
	switch code {
	case evdevKeyLeftShift, evdevKeyRightShift:
		return "shift"
	case evdevKeyLeftCtrl, evdevKeyRightCtrl:
		return "ctrl"
	case evdevKeyLeftAlt, evdevKeyRightAlt:
		return "alt"
	case evdevKeyLeftMeta, evdevKeyRightMeta:
		return "cmd"
	default:
		return ""
	}
}

func evdevKeyName(code uint16) string {
	switch code {
	case evdevKeyA:
		return "a"
	case evdevKeyB:
		return "b"
	case evdevKeyC:
		return "c"
	case evdevKeyD:
		return "d"
	case evdevKeyE:
		return "e"
	case evdevKeyF:
		return "f"
	case evdevKeyG:
		return "g"
	case evdevKeyH:
		return "h"
	case evdevKeyI:
		return "i"
	case evdevKeyJ:
		return "j"
	case evdevKeyK:
		return "k"
	case evdevKeyL:
		return "l"
	case evdevKeyM:
		return "m"
	case evdevKeyN:
		return "n"
	case evdevKeyO:
		return "o"
	case evdevKeyP:
		return "p"
	case evdevKeyQ:
		return "q"
	case evdevKeyR:
		return "r"
	case evdevKeyS:
		return "s"
	case evdevKeyT:
		return "t"
	case evdevKeyU:
		return "u"
	case evdevKeyV:
		return "v"
	case evdevKeyW:
		return "w"
	case evdevKeyX:
		return "x"
	case evdevKeyY:
		return "y"
	case evdevKeyZ:
		return "z"
	case evdevKey1:
		return "1"
	case evdevKey2:
		return "2"
	case evdevKey3:
		return "3"
	case evdevKey4:
		return "4"
	case evdevKey5:
		return "5"
	case evdevKey6:
		return "6"
	case evdevKey7:
		return "7"
	case evdevKey8:
		return "8"
	case evdevKey9:
		return "9"
	case evdevKey0:
		return "0"
	case evdevKeyMinus:
		return "-"
	case evdevKeyEqual:
		return "="
	case evdevKeyLeftBrace:
		return "["
	case evdevKeyRightBrace:
		return "]"
	case evdevKeyBackslash:
		return "\\"
	case evdevKeySemicolon:
		return ";"
	case evdevKeyApostrophe:
		return "'"
	case evdevKeyGrave:
		return "`"
	case evdevKeyComma:
		return ","
	case evdevKeyDot:
		return "."
	case evdevKeySlash:
		return "/"
	case evdevKeyEnter:
		return "Return"
	case evdevKeySpace:
		return "Space"
	case evdevKeyTab:
		return "Tab"
	case evdevKeyEsc:
		return "Escape"
	case evdevKeyBackspace:
		return "Backspace"
	case evdevKeyDelete:
		return "Delete"
	case evdevKeyLeft:
		return "Left"
	case evdevKeyRight:
		return "Right"
	case evdevKeyUp:
		return "Up"
	case evdevKeyDown:
		return "Down"
	case evdevKeyHome:
		return "Home"
	case evdevKeyEnd:
		return "End"
	case evdevKeyPageUp:
		return "PageUp"
	case evdevKeyPageDown:
		return "PageDown"
	case evdevKeyInsert:
		return "Insert"
	default:
		return ""
	}
}
