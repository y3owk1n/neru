//go:build darwin

// Package keyfeed posts keyboard input directly to the host operating system.
package keyfeed

/*
#cgo CFLAGS: -x objective-c
#include "../platform/darwin/hotkeys.h"
#include <ApplicationServices/ApplicationServices.h>
#include <Carbon/Carbon.h>
#include <stdbool.h>
#include <stdlib.h>

static const int neruSyntheticEventMarker = 0x1337;

static CGEventFlags flagsForModifiers(int modifiers) {
	CGEventFlags flags = 0;
	if (modifiers & ModifierCmd) {
		flags |= kCGEventFlagMaskCommand;
	}
	if (modifiers & ModifierShift) {
		flags |= kCGEventFlagMaskShift;
	}
	if (modifiers & ModifierAlt) {
		flags |= kCGEventFlagMaskAlternate;
	}
	if (modifiers & ModifierCtrl) {
		flags |= kCGEventFlagMaskControl;
	}

	return flags;
}

static CGKeyCode modifierKeyCode(int modifier) {
	switch (modifier) {
	case ModifierCmd:
		return kVK_Command;
	case ModifierShift:
		return kVK_Shift;
	case ModifierAlt:
		return kVK_Option;
	case ModifierCtrl:
		return kVK_Control;
	default:
		return 0xFFFF;
	}
}

static int postKeyboardEvent(CGEventSourceRef source, CGKeyCode keyCode, bool isDown, CGEventFlags flags) {
	CGEventRef event = CGEventCreateKeyboardEvent(source, keyCode, isDown);
	if (!event) {
		return 0;
	}

	CGEventSetFlags(event, flags);
	CGEventSetIntegerValueField(event, kCGEventSourceUserData, neruSyntheticEventMarker);
	CGEventPost(kCGHIDEventTap, event);
	CFRelease(event);

	return 1;
}

static int postModifierEvent(CGEventSourceRef source, int modifier, bool isDown, CGEventFlags flags) {
	CGKeyCode keyCode = modifierKeyCode(modifier);
	if (keyCode == 0xFFFF) {
		return 0;
	}

	CGEventRef event = CGEventCreateKeyboardEvent(source, keyCode, isDown);
	if (!event) {
		return 0;
	}

	CGEventSetType(event, kCGEventFlagsChanged);
	CGEventSetFlags(event, flags);
	CGEventSetIntegerValueField(event, kCGEventSourceUserData, neruSyntheticEventMarker);
	CGEventPost(kCGHIDEventTap, event);
	CFRelease(event);

	return 1;
}

static int postKeyFeed(const char *keyString) {
	int keyCode = 0;
	int modifiers = 0;
	if (!parseKeyString(keyString, &keyCode, &modifiers)) {
		return 0;
	}

	CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
	if (!source) {
		return -1;
	}

	int orderedModifiers[] = {ModifierCtrl, ModifierAlt, ModifierShift, ModifierCmd};
	CGEventFlags activeFlags = 0;
	int pressedModifiers = 0;
	int result = 1;

	for (int i = 0; i < 4; i++) {
		int modifier = orderedModifiers[i];
		if (!(modifiers & modifier)) {
			continue;
		}

		CGEventFlags nextFlags = activeFlags | flagsForModifiers(modifier);
		if (!postModifierEvent(source, modifier, true, nextFlags)) {
			result = -1;
			goto cleanup;
		}

		activeFlags = nextFlags;
		pressedModifiers |= modifier;
	}

	CGEventFlags keyFlags = flagsForModifiers(modifiers);
	if (!postKeyboardEvent(source, (CGKeyCode)keyCode, true, keyFlags) ||
	    !postKeyboardEvent(source, (CGKeyCode)keyCode, false, keyFlags)) {
		result = -1;
		goto cleanup;
	}

cleanup:
	for (int i = 3; i >= 0; i--) {
		int modifier = orderedModifiers[i];
		if (!(pressedModifiers & modifier)) {
			continue;
		}

		activeFlags &= ~flagsForModifiers(modifier);
		if (!postModifierEvent(source, modifier, false, activeFlags)) {
			result = -1;
		}
	}

	CFRelease(source);
	return result;
}
*/
import "C"

import (
	"strings"
	"unsafe"

	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	_ "github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

// Feed posts a key or key chord directly to macOS. Synthetic events are marked
// so Neru's own event tap ignores them when the daemon is running.
func Feed(key string) error {
	normalized := config.CanonicalHotkeyForPlatform(strings.TrimSpace(key))
	if normalized == "" {
		return derrors.New(derrors.CodeInvalidInput, "key is required")
	}

	cKey := C.CString(normalized)
	defer C.free(unsafe.Pointer(cKey)) //nolint:nlreturn

	ret := C.postKeyFeed(cKey)
	switch ret {
	case 1:
		return nil
	case 0:
		return derrors.Newf(derrors.CodeInvalidInput, "unsupported key %q", key)
	default:
		return derrors.New(
			derrors.CodeAccessibilityFailed,
			"failed to post key event: check accessibility permissions",
		)
	}
}
