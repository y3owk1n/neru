//go:build windows

package windows

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Virtual-key parsing and key-name normalization for Windows input hooks.
// Does not install hooks or register hotkeys.
const (
	modAlt     = 0x0001
	modControl = 0x0002
	modShift   = 0x0004
	modWin     = 0x0008

	vkBack     = 0x08
	vkTab      = 0x09
	vkReturn   = 0x0D
	vkEscape   = 0x1B
	vkSpace    = 0x20
	vkPrior    = 0x21
	vkNext     = 0x22
	vkEnd      = 0x23
	vkHome     = 0x24
	vkLeft     = 0x25
	vkUp       = 0x26
	vkRight    = 0x27
	vkDown     = 0x28
	vkInsert   = 0x2D
	vkDelete   = 0x2E
	vkLShift   = 0xA0
	vkRShift   = 0xA1
	vkLControl = 0xA2
	vkRControl = 0xA3
	vkLMenu    = 0xA4
	vkRMenu    = 0xA5
	vkLWin     = 0x5B
	vkRWin     = 0x5C
	vkControl  = 0x11
	vkMenu     = 0x12
	vkShift    = 0x10

	// Function keys occupy a contiguous VK range: VK_F1 (0x70) through
	// VK_F24 (0x87). They are handled arithmetically rather than as 24
	// separate constants.
	vkF1              = 0x70
	vkF24             = 0x87
	functionKeyCount  = vkF24 - vkF1 + 1
	functionKeyPrefix = "f"

	// mapvkVkToChar is MapVirtualKey's MAPVK_VK_TO_CHAR mode: translate a
	// virtual-key code to its unshifted character for the active keyboard
	// layout. The high bit of the result flags a dead key.
	mapvkVkToChar = 2

	// loWordMask isolates the low 16 bits; byteMask isolates the low 8 bits.
	loWordMask = 0xFFFF
	byteMask   = 0xFF

	// Neru modifier names, shared across parsing and name lookup.
	modNameCtrl  = "ctrl"
	modNameAlt   = "alt"
	modNameShift = "shift"
	modNameCmd   = "cmd"
)

var (
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
	procMapVirtualKeyW   = user32.NewProc("MapVirtualKeyW")
	procVkKeyScanW       = user32.NewProc("VkKeyScanW")
)

var (
	errEmptyHotkeyString    = errors.New("empty hotkey string")
	errEmptyHotkeyKey       = errors.New("empty hotkey key")
	errUnsupportedHotkeyKey = errors.New("unsupported hotkey key")
	errUnsupportedModifier  = errors.New("unsupported hotkey modifier")
)

// ParseHotkeyString parses a Neru hotkey string into RegisterHotKey modifiers and VK.
func ParseHotkeyString(keyString string) (uint32, uint32, error) {
	parts := strings.Split(keyString, "+")
	if len(parts) == 0 {
		return 0, 0, errEmptyHotkeyString
	}

	base := strings.TrimSpace(parts[len(parts)-1])
	if base == "" {
		return 0, 0, errEmptyHotkeyKey
	}

	virtualKey, ok := nameToVirtualKey(base)
	if !ok {
		return 0, 0, fmt.Errorf("%w: %s", errUnsupportedHotkeyKey, base)
	}

	var mods uint32
	for _, part := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case modNameCtrl, "control":
			mods |= modControl
		case modNameAlt, "option":
			mods |= modAlt
		case modNameShift:
			mods |= modShift
		case modNameCmd, "command", "win", "super", "meta":
			mods |= modWin
		default:
			return 0, 0, fmt.Errorf("%w: %s", errUnsupportedModifier, part)
		}
	}

	return mods, virtualKey, nil
}

// KeyNameFromVirtualKey maps a virtual-key code to Neru key strings.
func KeyNameFromVirtualKey(virtualKey uint32) string {
	switch virtualKey {
	case vkBack:
		return "Delete"
	case vkTab:
		return "Tab"
	case vkReturn:
		return "Return"
	case vkEscape:
		return "Escape"
	case vkSpace:
		return "Space"
	case vkLeft:
		return "Left"
	case vkRight:
		return "Right"
	case vkUp:
		return "Up"
	case vkDown:
		return "Down"
	// MapVirtualKey yields no character for the navigation keys, so this table
	// is the only path that names them. The names match the Linux backends.
	case vkPrior:
		return "PageUp"
	case vkNext:
		return "PageDown"
	case vkHome:
		return "Home"
	case vkEnd:
		return "End"
	case vkInsert:
		return "Insert"
	case vkDelete:
		return "Delete"
	case vkLShift, vkRShift:
		return modNameShift
	case vkLControl, vkRControl:
		return modNameCtrl
	case vkLMenu, vkRMenu:
		return modNameAlt
	case vkLWin, vkRWin:
		return modNameCmd
	default:
		if virtualKey >= 0x30 && virtualKey <= 0x39 {
			return string(rune(virtualKey))
		}

		if virtualKey >= 0x41 && virtualKey <= 0x5A {
			return strings.ToLower(string(rune(virtualKey)))
		}

		if name := functionKeyName(virtualKey); name != "" {
			return name
		}
		// OEM/punctuation keys (e.g. "`", "/", "-") are layout-dependent: the
		// same character lives on different VK codes across keyboard layouts.
		// Translate via the active layout so hotkeys like "`" match regardless
		// of whether the user is on a US, UK, or other layout.
		if name := charNameFromVirtualKey(virtualKey); name != "" {
			return name
		}
	}

	return ""
}

// functionKeyName returns the Neru display name ("F1" - "F24") for a function
// key virtual-key code, or "" when the code is not in the function key range.
func functionKeyName(vk uint32) string {
	if vk < vkF1 || vk > vkF24 {
		return ""
	}

	return "F" + strconv.Itoa(int(vk-vkF1)+1)
}

// functionKeyVirtualKey resolves a lowercase function key name ("f1" - "f24")
// to its virtual-key code. The second result is false for any other name.
func functionKeyVirtualKey(name string) (uint32, bool) {
	digits, ok := strings.CutPrefix(name, functionKeyPrefix)
	if !ok || digits == "" {
		return 0, false
	}

	index, err := strconv.Atoi(digits)
	if err != nil || index < 1 || index > functionKeyCount {
		return 0, false
	}

	return uint32(vkF1 + index - 1), true
}

// charNameFromVirtualKey maps a virtual-key code to its unshifted printable
// character for the active keyboard layout, or "" if it has none (or is a dead
// key). Letters are lowercased for consistency with the explicit letter path.
func charNameFromVirtualKey(vk uint32) string {
	ret, _, _ := procMapVirtualKeyW.Call(uintptr(vk), mapvkVkToChar)
	if ret == 0 || ret&0x80000000 != 0 {
		return ""
	}

	keyChar := rune(ret & loWordMask)
	if keyChar < 0x20 || keyChar > 0x7E {
		return ""
	}

	if unicode.IsLetter(keyChar) {
		return strings.ToLower(string(keyChar))
	}

	return string(keyChar)
}

// virtualKeyFromChar resolves a single character to its virtual-key code on the
// active keyboard layout, ignoring the required shift state. Returns false when
// the character is not reachable on the current layout.
func virtualKeyFromChar(r rune) (uint32, bool) {
	ret, _, _ := procVkKeyScanW.Call(uintptr(r))

	scan := int16(ret)
	if scan == -1 {
		return 0, false
	}

	vk := uint32(scan) & byteMask
	if vk == 0 {
		return 0, false
	}

	return vk, true
}

// KeyComboFromVirtualKey maps a virtual-key code to a Neru combo string (e.g. shift+l).
// Modifier-only keys return the modifier name alone.
func KeyComboFromVirtualKey(virtualKey uint32) string {
	base := KeyNameFromVirtualKey(virtualKey)
	if base == "" {
		return ""
	}

	if ModifierNameFromVirtualKey(virtualKey) != "" {
		return base
	}

	return KeyComboFromBaseAndModifiers(base, pressedModifierNames())
}

// KeyComboFromBaseAndModifiers builds a Neru key combo from a base key and modifiers.
func KeyComboFromBaseAndModifiers(base string, modifiers []string) string {
	if base == "" {
		return ""
	}

	if len(modifiers) == 0 {
		return base
	}

	parts := append(append([]string(nil), modifiers...), base)

	return strings.Join(parts, "+")
}

func pressedModifierNames() []string {
	var mods []string

	if isVirtualKeyDown(vkControl) {
		mods = append(mods, modNameCtrl)
	}

	if isVirtualKeyDown(vkMenu) {
		mods = append(mods, modNameAlt)
	}

	if isVirtualKeyDown(vkShift) {
		mods = append(mods, modNameShift)
	}

	if isVirtualKeyDown(vkLWin) || isVirtualKeyDown(vkRWin) {
		mods = append(mods, modNameCmd)
	}

	return mods
}

func isVirtualKeyDown(vk uint32) bool {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vk))

	return ret&0x8000 != 0
}

// ModifierNameFromVirtualKey returns modifier names for dedicated modifier VK codes.
func ModifierNameFromVirtualKey(virtualKey uint32) string {
	name := KeyNameFromVirtualKey(virtualKey)
	switch name {
	case modNameShift, modNameCtrl, modNameAlt, modNameCmd:
		return name
	default:
		return ""
	}
}

func nameToVirtualKey(name string) (uint32, bool) {
	lowered := strings.ToLower(strings.TrimSpace(name))

	switch lowered {
	case "return", "enter":
		return vkReturn, true
	case "space":
		return vkSpace, true
	case "tab":
		return vkTab, true
	case "escape", "esc":
		return vkEscape, true
	case "backspace", "delete":
		return vkBack, true
	case "left":
		return vkLeft, true
	case "right":
		return vkRight, true
	case "up":
		return vkUp, true
	case "down":
		return vkDown, true
	case "pageup":
		return vkPrior, true
	case "pagedown":
		return vkNext, true
	case "home":
		return vkHome, true
	case "end":
		return vkEnd, true
	case "insert":
		return vkInsert, true
	default:
		if vk, ok := functionKeyVirtualKey(lowered); ok {
			return vk, true
		}

		if len(name) == 1 {
			keyRune := rune(name[0])
			if unicode.IsLetter(keyRune) {
				return uint32(unicode.ToUpper(keyRune)), true
			}

			if unicode.IsDigit(keyRune) {
				return uint32(keyRune), true
			}

			if vk, ok := virtualKeyFromChar(keyRune); ok {
				return vk, true
			}
		}
	}

	return 0, false
}
