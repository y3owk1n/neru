//go:build !darwin

package vision

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/core/domain/element"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// DetectElements is a no-op stub on non-darwin platforms.
func (a *Adapter) DetectElements(_ context.Context, _ image.Rectangle) ([]*element.Element, error) {
	return nil, derrors.New(derrors.CodeNotSupported, "vision detection is only supported on macOS")
}

// CaptureScreen is a no-op stub on non-darwin platforms.
func (a *Adapter) CaptureScreen(_ context.Context) (*image.RGBA, error) {
	return nil, derrors.New(derrors.CodeNotSupported, "screen capture is only supported on macOS")
}

// Health reports not-supported on non-darwin platforms.
func (a *Adapter) Health(_ context.Context) error {
	return derrors.New(derrors.CodeNotSupported, "vision framework is only available on macOS")
}
