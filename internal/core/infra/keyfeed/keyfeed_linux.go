//go:build linux

// Package keyfeed posts keyboard input directly to the host operating system.
package keyfeed

import (
	"strings"

	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
)

// Feed posts a key or key chord directly to the OS via the Wayland virtual
// keyboard protocol (zwp_virtual_keyboard_v1). Only wlroots-based compositors
// (niri, Sway, Hyprland, River) are supported.
//
// Key strings follow the canonical form used by CanonicalHotkeyForPlatform:
//   - single character: "a", "B", "1"
//   - named key: "Return", "F1", "Space"
//   - modifier+key: "Ctrl+c", "Shift+F1", "Ctrl+Shift+Space"
//
// Uppercase letters (A-Z) without an explicit Shift modifier automatically
// inject Shift, producing the correct uppercase character.
func Feed(key string) error {
	normalized, err := NormalizeKeyForFeed(key)
	if err != nil {
		return err
	}

	err = linux.FeedKey(normalized)
	if err != nil {
		return err
	}

	return nil
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
