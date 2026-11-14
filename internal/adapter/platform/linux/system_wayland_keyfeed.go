//go:build linux

package linux

import (
	"slices"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
)

// evdevKeycode maps lowercase canonical key names to linux/input-event-codes.h
// KEY_* codes.
//
//nolint:mnd // evdev keycodes are hardware constants, not magic numbers.
var evdevKeycode = map[string]uint32{
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

// modifierNameMap maps display-form modifier names to the lowercase names
// waylandModifierEvent expects.
var modifierNameMap = map[string]string{
	modNameCtrl:  modNameCtrl,
	modNameShift: modNameShift,
	modNameAlt:   modNameAlt,
	"super":      modNameCmd,
	modNameCmd:   modNameCmd,
}

// FeedKey injects a key or chord such as "Ctrl+Shift+Space", already in
// keyvocab.CanonicalHotkeyForPlatform form. It prefers uinput, otherwise the
// compositor's own path: zwp_virtual_keyboard_v1 on wlroots, libei on KWin
// when the portal granted a keyboard.
func FeedKey(key string) error {
	// Validate first, so an invalid key is CodeInvalidInput on any backend.
	modifiers, keycode, err := parseKeyString(key)
	if err != nil {
		return err
	}

	// uinput works the same everywhere and avoids KDE's portal.
	if uinputKeyboardAvailable() {
		return feedKeyUinput(modifiers, keycode)
	}

	hasVKB, err := wlrootsHasVirtualKeyboard()
	if err != nil {
		return err
	}

	if hasVKB {
		return feedKeyWlroots(modifiers, keycode)
	}

	return feedKeyLibei(modifiers, keycode)
}

// uinputModifierCodes resolves parseKeyString's modifier names to evdev
// keycodes in order. libeiModifierKeycodes is provider-agnostic despite its
// name.
func uinputModifierCodes(modifiers []string) ([]int, error) {
	codes := make([]int, 0, len(modifiers))

	for _, modifier := range modifiers {
		code, ok := libeiModifierKeycodes[modifier]
		if !ok {
			return nil, derrors.Newf(
				derrors.CodeNotSupported,
				"unsupported modifier %q for uinput injection",
				modifier,
			)
		}

		codes = append(codes, code)
	}

	return codes, nil
}

func feedKeyWlroots(modifiers []string, keycode uint32) error {
	var err error

	// The ref-counting dispatcher keeps a modifier another consumer (sticky
	// modifiers, clicks) holds from being released when this feed ends.
	for i, modifier := range modifiers {
		err = WaylandModifierEvent(modifier, true)
		if err != nil {
			for j := range i {
				_ = WaylandModifierEvent(modifiers[j], false)
			}

			return err
		}
	}

	err = wlrootsKey(keycode, true)
	if err == nil {
		err = wlrootsKey(keycode, false)
	}

	for _, v := range slices.Backward(modifiers) {
		_ = WaylandModifierEvent(v, false)
	}

	return err
}

func feedKeyLibei(modifiers []string, keycode uint32) error {
	hasKeyboard, busy := libeiHasKeyboard()
	if busy {
		return derrors.New(
			derrors.CodeActionFailed,
			"libei session busy: RemoteDesktop portal warm-up in progress; "+
				"approve the one-time consent prompt, then retry",
		)
	}

	if !hasKeyboard {
		return derrors.New(
			derrors.CodeNotSupported,
			"key feeding unavailable: the RemoteDesktop portal session "+
				"did not grant a keyboard device. Restart neru and check "+
				"\"Enable keyboard\" in the consent prompt; if the prompt "+
				"does not appear on KDE, run `flatpak permission-remove "+
				"kde-authorized remote-desktop \"\"` first to clear the saved grant",
		)
	}

	var err error

	// Ref-counted like feedKeyWlroots, routed to libeiKey.
	for i, modifier := range modifiers {
		err = WaylandModifierEvent(modifier, true)
		if err != nil {
			for j := range i {
				_ = WaylandModifierEvent(modifiers[j], false)
			}

			return err
		}
	}

	err = libeiKey(int(keycode), true)
	if err == nil {
		err = libeiKey(int(keycode), false)
	}

	for _, v := range slices.Backward(modifiers) {
		_ = WaylandModifierEvent(v, false)
	}

	return err
}

// ParseFeedKey splits a key in the form FeedKey takes into its modifier names
// (ctrl, shift, alt, cmd) and the main key's evdev keycode, for the X11
// backend, which injects keys through XTest instead.
func ParseFeedKey(key string) ([]string, uint32, error) {
	return parseKeyString(key)
}

// parseKeyString splits a canonical key string into lowercase modifier names
// and the main key's evdev keycode.
func parseKeyString(key string) ([]string, uint32, error) {
	parts := strings.Split(key, "+")

	var modifiers []string

	for i := range len(parts) - 1 {
		lower := strings.ToLower(strings.TrimSpace(parts[i]))

		internal, ok := modifierNameMap[lower]
		if !ok {
			return nil, 0, derrors.Newf(derrors.CodeInvalidInput, "unknown modifier %q", parts[i])
		}

		modifiers = append(modifiers, internal)
	}

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
