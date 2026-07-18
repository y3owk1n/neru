//go:build !darwin

package modes

func (h *Handler) hideSystemCursorNative() {}

func (h *Handler) showSystemCursorNative() {}

// RehideSystemCursor is a no-op on non-macOS platforms.
func (h *Handler) RehideSystemCursor() {}
