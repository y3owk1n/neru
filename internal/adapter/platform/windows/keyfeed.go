//go:build windows

package windows

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Key feed: typing a key into the focused application through SendInput.
const (
	// mapvkVkToVsc is MapVirtualKey's MAPVK_VK_TO_VSC mode: virtual key to
	// scan code. Applications that read the scan code rather than the virtual
	// key (games, some terminals) see nothing without it.
	mapvkVkToVsc = 0

	// VkKeyScan reports the modifiers a character needs on the active layout
	// in its high byte.
	vkKeyScanShift   = 0x01
	vkKeyScanControl = 0x02
	vkKeyScanAlt     = 0x04
	// vkKeyScanStateShift moves that high byte down to the low one.
	vkKeyScanStateShift = 8
)

// extendedVirtualKeys are the keys whose scan code carries the 0xE0 prefix.
// Without KEYEVENTF_EXTENDEDKEY the navigation cluster arrives as its numpad
// twin, so Home reads as numpad 7.
var extendedVirtualKeys = map[uint16]bool{
	vkPrior: true, vkNext: true, vkEnd: true, vkHome: true,
	vkLeft: true, vkUp: true, vkRight: true, vkDown: true,
	vkInsert: true, vkDelete: true,
	vkLWin: true, vkRWin: true, vkRControl: true, vkRMenu: true,
}

// FeedKey presses and releases one key or chord in the focused application.
//
// key is the canonical form config.CanonicalHotkeyForPlatform produces on
// Windows ("a", "Return", "Ctrl+Shift+Space"). Modifiers go down first, the
// key is tapped, and the modifiers come up in reverse. A character that needs
// Shift on the active layout ("!", "?") gets it added.
func FeedKey(key string) error {
	modifiers, virtualKey, err := parseFeedKey(key)
	if err != nil {
		return err
	}

	pressed := make([]uint16, 0, len(modifiers))

	for _, modifier := range modifiers {
		err = sendKeyboardInput(modifier, false)
		if err != nil {
			return wrapSendInput(errors.Join(err, releaseKeys(pressed)))
		}

		pressed = append(pressed, modifier)
	}

	return wrapSendInput(errors.Join(tapKey(virtualKey), releaseKeys(pressed)))
}

// tapKey presses and releases one key with its layout scan code.
func tapKey(virtualKey uint16) error {
	scan, _, _ := procMapVirtualKeyW.Call(uintptr(virtualKey), mapvkVkToVsc)

	var flags uint32
	if extendedVirtualKeys[virtualKey] {
		flags = keyeventfExtendedKey
	}

	err := sendKeyEvent(virtualKey, uint16(scan), flags)
	if err != nil {
		return err
	}

	return sendKeyEvent(virtualKey, uint16(scan), flags|keyeventfKeyUp)
}

// releaseKeys lifts modifiers in reverse press order. Every release is
// attempted even after one fails, so a modifier is never left held because
// an earlier one could not be lifted; the failures are joined.
func releaseKeys(keys []uint16) error {
	var errs []error

	for _, key := range slices.Backward(keys) {
		errs = append(errs, sendKeyboardInput(key, true))
	}

	return errors.Join(errs...)
}

func wrapSendInput(err error) error {
	if err == nil {
		return nil
	}

	return derrors.Wrap(err, derrors.CodeActionFailed, "failed to post key event")
}

// parseFeedKey splits a canonical key string into the modifier keys to hold
// and the virtual key to tap. Modifiers implied by the character on the
// active layout are appended after the explicit ones, without duplicates.
func parseFeedKey(key string) ([]uint16, uint16, error) {
	parts := strings.Split(key, "+")

	base := strings.TrimSpace(parts[len(parts)-1])
	if base == "" {
		return nil, 0, derrors.New(derrors.CodeInvalidInput, "key is required")
	}

	modifiers := make([]uint16, 0, len(parts))

	for _, part := range parts[:len(parts)-1] {
		modifier, err := feedModifierKey(part)
		if err != nil {
			return nil, 0, derrors.Wrapf(err, derrors.CodeInvalidInput, "key %q", key)
		}

		modifiers = appendModifier(modifiers, modifier)
	}

	virtualKey, implied, ok := feedVirtualKey(base)
	if !ok {
		return nil, 0, derrors.Newf(derrors.CodeInvalidInput, "unsupported key %q", base)
	}

	for _, modifier := range implied {
		modifiers = appendModifier(modifiers, modifier)
	}

	return modifiers, virtualKey, nil
}

func appendModifier(modifiers []uint16, modifier uint16) []uint16 {
	if slices.Contains(modifiers, modifier) {
		return modifiers
	}

	return append(modifiers, modifier)
}

// feedModifierKey maps a modifier token to the canonical (left) key that
// presents it, the same key PostModifierKey holds.
func feedModifierKey(token string) (uint16, error) {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case modNameCtrl, "control":
		return vkLControl, nil
	case modNameAlt, "option":
		return vkLMenu, nil
	case modNameShift:
		return vkLShift, nil
	case modNameCmd, "command", "win", "super", "meta":
		return vkLWin, nil
	default:
		return 0, fmt.Errorf("%w: %s", errUnsupportedModifier, token)
	}
}

// feedVirtualKey resolves a base key name to its virtual key plus the
// modifiers the active layout needs to produce it. Named keys and letters
// carry none; any other single character, digits included, asks VkKeyScan,
// which is the only source that knows "1" is Shift+& on French and "!" is
// Shift+1 on US.
func feedVirtualKey(name string) (uint16, []uint16, bool) {
	keyRune, size := utf8.DecodeRuneInString(name)
	if size == len(name) && !unicode.IsLetter(keyRune) {
		return virtualKeyWithLayoutModifiers(keyRune)
	}

	virtualKey, ok := nameToVirtualKey(name)
	if !ok {
		return 0, nil, false
	}

	return uint16(virtualKey), nil, true
}

func virtualKeyWithLayoutModifiers(keyRune rune) (uint16, []uint16, bool) {
	ret, _, _ := procVkKeyScanW.Call(uintptr(keyRune))

	scan := int16(ret)
	if scan == -1 {
		return 0, nil, false
	}

	virtualKey := uint16(scan) & byteMask
	if virtualKey == 0 {
		return 0, nil, false
	}

	state := (uint16(scan) >> vkKeyScanStateShift) & byteMask

	var modifiers []uint16

	if state&vkKeyScanShift != 0 {
		modifiers = append(modifiers, vkLShift)
	}

	if state&vkKeyScanControl != 0 {
		modifiers = append(modifiers, vkLControl)
	}

	if state&vkKeyScanAlt != 0 {
		modifiers = append(modifiers, vkLMenu)
	}

	return virtualKey, modifiers, true
}
