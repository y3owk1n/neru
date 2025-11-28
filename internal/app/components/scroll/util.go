package scroll

import (
	"go.uber.org/zap"
)

const (
	// CtrlD is the byte value for Ctrl+D.
	CtrlD = 4

	// CtrlU is the byte value for Ctrl+U.
	CtrlU = 21
)

// ParseKey parses a key press and returns the operation, the new last key state, and whether the key sequence is valid.
func ParseKey(
	key string,
	lastKey string,
	_ *zap.Logger,
) (string, string, bool) {
	// Handle multi-key sequences (g -> g = top)
	if lastKey == "g" {
		if key == "g" {
			return "top", "", true
		}
		// If we had 'g' but next key isn't 'g', reset state and process current key normally
		// This allows 'g' followed by 'j' to just scroll down instead of being ignored
		// But for now, let's just reset and return false to indicate the sequence failed
		return "", "", false
	}

	switch key {
	case "j":
		return "down", "", true
	case "k":
		return "up", "", true
	case "h":
		return "left", "", true
	case "l":
		return "right", "", true
	case "g":
		// Start of 'gg' sequence
		return "start_g", "g", true
	case "G":
		return "bottom", "", true
	}

	// Check for control keys
	if len(key) == 1 {
		byteVal := key[0]
		if byteVal == CtrlD { // Ctrl+D
			return "half_down", "", true
		}

		if byteVal == CtrlU { // Ctrl+U
			return "half_up", "", true
		}
	}

	return "", "", false
}
