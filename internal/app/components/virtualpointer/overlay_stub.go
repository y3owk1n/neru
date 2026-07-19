//go:build !darwin

package virtualpointer

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

// Overlay is a no-op virtual pointer overlay on non-macOS platforms.
type Overlay struct{}

// NewOverlay creates a no-op virtual pointer overlay.
func NewOverlay(
	_ config.VirtualPointerConfig,
	_ config.ThemeProvider,
	_ *zap.Logger,
) (*Overlay, error) {
	return &Overlay{}, nil
}

// SetConfig is a no-op.
func (o *Overlay) SetConfig(_ config.VirtualPointerConfig) {}

// Show is a no-op.
func (o *Overlay) Show() {}

// Hide is a no-op.
func (o *Overlay) Hide() {}

// Clear is a no-op.
func (o *Overlay) Clear() {}

// ResizeToActiveScreen is a no-op.
func (o *Overlay) ResizeToActiveScreen() {}

// Draw is a no-op.
func (o *Overlay) Draw(_, _, _ int, _ string) {}

// SetSharingType is a no-op.
func (o *Overlay) SetSharingType(_ bool) {}

// Destroy is a no-op.
func (o *Overlay) Destroy() {}
