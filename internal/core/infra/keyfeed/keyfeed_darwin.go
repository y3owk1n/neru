//go:build darwin

// Package keyfeed posts keyboard input directly to the host operating system.
package keyfeed

/*
#cgo CFLAGS: -x objective-c
#include "../platform/darwin/keyfeed.h"
#include <stdlib.h>
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
// When a single uppercase letter (A-Z) is provided without an explicit Shift modifier,
// Shift is automatically added to produce the correct uppercase character.
func Feed(key string) error {
	normalized, err := NormalizeKeyForFeed(key)
	if err != nil {
		return err
	}

	cKey := C.CString(normalized)
	defer C.free(unsafe.Pointer(cKey)) //nolint:nlreturn

	ret := C.NeruPostKeyFeed(cKey)
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

// NormalizeKeyForFeed normalizes a key string for feeding to the OS.
// It handles uppercase letter detection and Shift injection.
func NormalizeKeyForFeed(key string) (string, error) {
	trimmed := strings.TrimSpace(key)

	isSingleUppercase := len(trimmed) == 1 && trimmed[0] >= 'A' && trimmed[0] <= 'Z'

	normalized := config.CanonicalHotkeyForPlatform(trimmed)
	if normalized == "" {
		return "", derrors.New(derrors.CodeInvalidInput, "key is required")
	}

	if isSingleUppercase && !strings.Contains(normalized, "+") {
		normalized = "Shift+" + normalized
	}

	return normalized, nil
}
