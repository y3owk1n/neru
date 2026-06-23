//go:build windows

// internal/core/infra/platform/windows/keys.go
// Virtual-key parsing and key-name normalization for Windows input hooks.
// Does not install hooks or register hotkeys.

package windows

import (
	"fmt"
	"strings"
	"unicode"
)

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
	vkLeft     = 0x25
	vkUp       = 0x26
	vkRight    = 0x27
	vkDown     = 0x28
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

	// mapvkVkToChar is MapVirtualKey's MAPVK_VK_TO_CHAR mode: translate a
	// virtual-key code to its unshifted character for the active keyboard
	// layout. The high bit of the result flags a dead key.
	mapvkVkToChar = 2
)

var (
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
	procMapVirtualKeyW   = user32.NewProc("MapVirtualKeyW")
	procVkKeyScanW       = user32.NewProc("VkKeyScanW")
)

// ParseHotkeyString parses a Neru hotkey string into RegisterHotKey modifiers and VK.
func ParseHotkeyString(keyString string) (modifiers uint32, virtualKey uint32, err error) {
	parts := strings.Split(keyString, "+")
	if len(parts) == 0 {
		return 0, 0, fmt.Errorf("empty hotkey string")
	}

	base := strings.TrimSpace(parts[len(parts)-1])
	if base == "" {
		return 0, 0, fmt.Errorf("empty hotkey key")
	}

	vk, ok := nameToVirtualKey(base)
	if !ok {
		return 0, 0, fmt.Errorf("unsupported hotkey key: %s", base)
	}

	var mods uint32
	for _, part := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "ctrl", "control":
			mods |= modControl
		case "alt", "option":
			mods |= modAlt
		case "shift":
			mods |= modShift
		case "cmd", "command", "win", "super", "meta":
			mods |= modWin
		default:
			return 0, 0, fmt.Errorf("unsupported hotkey modifier: %s", part)
		}
	}

	return mods, vk, nil
}

// KeyNameFromVirtualKey maps a virtual-key code to Neru key strings.
func KeyNameFromVirtualKey(vk uint32) string {
	switch vk {
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
	case vkDelete:
		return "Delete"
	case vkLShift, vkRShift:
		return "shift"
	case vkLControl, vkRControl:
		return "ctrl"
	case vkLMenu, vkRMenu:
		return "alt"
	case vkLWin, vkRWin:
		return "cmd"
	default:
		if vk >= 0x30 && vk <= 0x39 {
			return string(rune(vk))
		}
		if vk >= 0x41 && vk <= 0x5A {
			return strings.ToLower(string(rune(vk)))
		}
		// OEM/punctuation keys (e.g. "`", "/", "-") are layout-dependent: the
		// same character lives on different VK codes across keyboard layouts.
		// Translate via the active layout so hotkeys like "`" match regardless
		// of whether the user is on a US, UK, or other layout.
		if name := charNameFromVirtualKey(vk); name != "" {
			return name
		}
	}

	return ""
}

// charNameFromVirtualKey maps a virtual-key code to its unshifted printable
// character for the active keyboard layout, or "" if it has none (or is a dead
// key). Letters are lowercased for consistency with the explicit letter path.
func charNameFromVirtualKey(vk uint32) string {
	ret, _, _ := procMapVirtualKeyW.Call(uintptr(vk), mapvkVkToChar)
	if ret == 0 || ret&0x80000000 != 0 {
		return ""
	}

	ch := rune(ret & 0xFFFF)
	if ch < 0x20 || ch > 0x7E {
		return ""
	}

	if unicode.IsLetter(ch) {
		return strings.ToLower(string(ch))
	}

	return string(ch)
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

	vk := uint32(scan) & 0xFF
	if vk == 0 {
		return 0, false
	}

	return vk, true
}

// KeyComboFromVirtualKey maps a virtual-key code to a Neru combo string (e.g. shift+l).
// Modifier-only keys return the modifier name alone.
func KeyComboFromVirtualKey(vk uint32) string {
	base := KeyNameFromVirtualKey(vk)
	if base == "" {
		return ""
	}

	if ModifierNameFromVirtualKey(vk) != "" {
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
		mods = append(mods, "ctrl")
	}

	if isVirtualKeyDown(vkMenu) {
		mods = append(mods, "alt")
	}

	if isVirtualKeyDown(vkShift) {
		mods = append(mods, "shift")
	}

	if isVirtualKeyDown(vkLWin) || isVirtualKeyDown(vkRWin) {
		mods = append(mods, "cmd")
	}

	return mods
}

func isVirtualKeyDown(vk uint32) bool {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vk))

	return ret&0x8000 != 0
}

// ModifierNameFromVirtualKey returns modifier names for dedicated modifier VK codes.
func ModifierNameFromVirtualKey(vk uint32) string {
	name := KeyNameFromVirtualKey(vk)
	switch name {
	case "shift", "ctrl", "alt", "cmd":
		return name
	default:
		return ""
	}
}

func nameToVirtualKey(name string) (uint32, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
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
	default:
		if len(name) == 1 {
			r := rune(name[0])
			if unicode.IsLetter(r) {
				return uint32(unicode.ToUpper(r)), true
			}
			if unicode.IsDigit(r) {
				return uint32(r), true
			}
			if vk, ok := virtualKeyFromChar(r); ok {
				return vk, true
			}
		}
	}

	return 0, false
}
