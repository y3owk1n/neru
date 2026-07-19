//go:build !darwin

package modes

// CursorVisibilitySupported returns false on non-macOS platforms.
func (h *Handler) CursorVisibilitySupported() bool { return false }

func (h *Handler) hideSystemCursorNative() {}

func (h *Handler) showSystemCursorNative() {}

// RehideSystemCursor is a no-op on non-macOS platforms.
func (h *Handler) RehideSystemCursor() {}
