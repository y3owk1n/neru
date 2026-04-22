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
	shift bool
	ctrl  bool
	alt   bool
	cmd   bool
}

func (s *evdevModifierState) update(modifier string, isDown bool) {
	switch modifier {
	case evdevModifierShift:
		s.shift = isDown
	case evdevModifierCtrl:
		s.ctrl = isDown
	case evdevModifierAlt:
		s.alt = isDown
	case evdevModifierCmd:
		s.cmd = isDown
	}
}

func (s *evdevModifierState) prefix() string {
	if s == nil {
		return ""
	}

	var prefix strings.Builder

	if s.shift {
		prefix.WriteString(evdevPrefixShift)
	}

	if s.ctrl {
		prefix.WriteString(evdevPrefixCtrl)
	}

	if s.alt {
		prefix.WriteString(evdevPrefixAlt)
	}

	if s.cmd {
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
