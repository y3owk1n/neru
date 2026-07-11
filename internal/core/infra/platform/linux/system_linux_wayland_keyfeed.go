//go:build linux

package linux

import (
	"slices"
	"strings"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// evdevKeycode maps canonical key names (lowercase) to evdev keycodes.
// Based on linux/input-event-codes.h KEY_* constants.
//
//nolint:mnd // evdev keycodes are hardware constants, not magic numbers.
var evdevKeycode = map[string]uint32{
	// Letters
	"a": 30, "b": 48, "c": 46, "d": 32, "e": 18, "f": 33, "g": 34, "h": 35,
	"i": 23, "j": 36, "k": 37, "l": 38, "m": 50, "n": 49, "o": 24, "p": 25,
	"q": 16, "r": 19, "s": 31, "t": 20, "u": 22, "v": 47, "w": 17, "x": 45,
	"y": 21, "z": 44,
	// Digits
	"0": 11, "1": 2, "2": 3, "3": 4, "4": 5, "5": 6, "6": 7, "7": 8, "8": 9, "9": 10,
	// Function keys
	"f1": 59, "f2": 60, "f3": 61, "f4": 62, "f5": 63, "f6": 64,
	"f7": 65, "f8": 66, "f9": 67, "f10": 68, "f11": 87, "f12": 88,
	"f13": 183, "f14": 184, "f15": 185, "f16": 186, "f17": 187, "f18": 188,
	"f19": 189, "f20": 190, "f21": 191, "f22": 192, "f23": 193, "f24": 194,
	// Named keys
	"space":     57,
	"return":    28,
	"enter":     28,
	"tab":       15,
	"escape":    1,
	"backspace": 14,
	"delete":    111,
	"home":      102,
	"end":       107,
	"pageup":    104,
	"pagedown":  109,
	"up":        103,
	"down":      108,
	"left":      105,
	"right":     106,
	// Punctuation and symbols
	"comma":        51,
	"period":       52,
	"semicolon":    39,
	"quote":        40,
	"apostrophe":   40,
	"slash":        53,
	"backslash":    43,
	"bracketleft":  26,
	"bracketright": 27,
	"minus":        12,
	"equal":        13,
	"grave":        41,
	// Character aliases for punctuation
	",":  51,
	".":  52,
	";":  39,
	"'":  40,
	"/":  53,
	"\\": 43,
	"[":  26,
	"]":  27,
	"-":  12,
	"=":  13,
	"`":  41,
	// Numpad
	"kp0": 82, "kp1": 79, "kp2": 80, "kp3": 81, "kp4": 75,
	"kp5": 76, "kp6": 77, "kp7": 71, "kp8": 72, "kp9": 73,
	"kpdot":      83,
	"kpdivide":   98,
	"kpmultiply": 55,
	"kpminus":    74,
	"kpplus":     78,
	"kpenter":    96,
}

// modifierNameMap maps display-form modifier names (from
// CanonicalHotkeyForPlatform) to the lowercase names expected by
// neru_wlr_modifier_event / waylandModifierEvent.
var modifierNameMap = map[string]string{
	modNameCtrl:  modNameCtrl,
	modNameShift: modNameShift,
	modNameAlt:   modNameAlt,
	"super":      modNameCmd,
	modNameCmd:   modNameCmd,
}

// FeedKey injects a key or key chord into the Wayland compositor.
// The key string must already be in canonical form (as returned by
// config.CanonicalHotkeyForPlatform).
//
// Supported formats:
//   - single key: "a", "Return", "F1"
//   - modifier+key: "Ctrl+c", "Shift+F1", "Ctrl+Shift+Space"
//
// Backend selection:
//   - wlroots compositors (niri, Sway, Hyprland, River) use
//     zwp_virtual_keyboard_v1.
//   - KDE/KWin uses libei / RemoteDesktop portal when a keyboard device was
//     granted by the user (the portal defaults to pointer-only, so keyboard
//     may be unavailable).
func FeedKey(key string) error {
	hasVKB, err := wlrootsHasVirtualKeyboard()
	if err != nil {
		return err
	}

	if hasVKB {
		return feedKeyWlroots(key)
	}

	return feedKeyLibei(key)
}

func feedKeyWlroots(key string) error {
	modifiers, keycode, err := parseKeyString(key)
	if err != nil {
		return err
	}

	// 1. Press modifiers via protocol-level modifier state.
	for _, modifier := range modifiers {
		err := wlrootsModifierEvent(modifier, true)
		if err != nil {
			for j := range modifiers {
				if j >= len(modifiers) {
					break
				}

				_ = wlrootsModifierEvent(modifiers[j], false)
			}

			return err
		}
	}

	// 2. Press and release the main key.
	err = wlrootsKey(keycode, true)
	if err == nil {
		err = wlrootsKey(keycode, false)
	}

	// 3. Release modifiers in reverse order.
	for _, v := range slices.Backward(modifiers) {
		_ = wlrootsModifierEvent(v, false)
	}

	return err
}

func feedKeyLibei(key string) error {
	if !libeiHasKeyboard() {
		return derrors.New(
			derrors.CodeNotSupported,
			"key feeding unavailable on KDE: the RemoteDesktop portal session "+
				"did not grant a keyboard device; only a pointer was granted",
		)
	}

	modifiers, keycode, err := parseKeyString(key)
	if err != nil {
		return err
	}

	// Modifier names to evdev keycodes for the libei path (actual key presses).
	modCode := func(name string) (int, bool) {
		code, ok := libeiModifierKeycodes[name]

		return code, ok
	}

	// 1. Press modifier keys.
	for _, mod := range modifiers {
		code, ok := modCode(mod)
		if !ok {
			return derrors.Newf(
				derrors.CodeInvalidInput,
				"unsupported modifier %q for libei keyboard",
				mod,
			)
		}

		err := libeiKey(code, true)
		if err != nil {
			for j := range modifiers {
				if j >= len(modifiers) {
					break
				}

				if releaseCode, ok2 := modCode(modifiers[j]); ok2 {
					_ = libeiKey(releaseCode, false)
				}
			}

			return err
		}
	}

	// 2. Press and release the main key.
	err = libeiKey(int(keycode), true)
	if err == nil {
		err = libeiKey(int(keycode), false)
	}

	// 3. Release modifier keys in reverse order.
	for _, v := range slices.Backward(modifiers) {
		if releaseCode, ok := modCode(v); ok {
			_ = libeiKey(releaseCode, false)
		}
	}

	return err
}

// parseKeyString splits a canonical key string into modifier names and
// an evdev keycode. Returns the modifiers (lowercase internal names,
// e.g. "ctrl", "shift") and the main key's evdev keycode.
func parseKeyString(key string) ([]string, uint32, error) {
	parts := strings.Split(key, "+")

	var modifiers []string

	// Identify modifiers (all parts except the last).
	for i := range len(parts) - 1 {
		lower := strings.ToLower(strings.TrimSpace(parts[i]))

		internal, ok := modifierNameMap[lower]
		if !ok {
			return nil, 0, derrors.Newf(derrors.CodeInvalidInput, "unknown modifier %q", parts[i])
		}

		modifiers = append(modifiers, internal)
	}

	// Identify the main key (last part).
	mainKey := strings.TrimSpace(parts[len(parts)-1])
	if mainKey == "" {
		return nil, 0, derrors.New(derrors.CodeInvalidInput, "key is required")
	}

	code, ok := evdevKeycode[strings.ToLower(mainKey)]
	if !ok {
		return nil, 0, derrors.Newf(derrors.CodeInvalidInput, "unsupported key %q", mainKey)
	}

	return modifiers, code, nil
}
