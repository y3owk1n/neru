//go:build !darwin && !linux && !windows

package vision

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// DetectElements reports not-supported on the platforms with no vision
// implementation at all: the CGO-less `other` slot, which no supported OS
// builds into.
func (a *Adapter) DetectElements(
	_ context.Context,
	_ image.Rectangle,
	_ config.HintsVisionConfig,
	_ bool,
) ([]*element.Element, error) {
	return nil, derrors.New(
		derrors.CodeNotSupported,
		"vision element detection is not implemented on this platform",
	)
}

// DetectContours reports not-supported: the detector is platform-neutral, but
// this slot has no capture backend to feed it a frame.
func (a *Adapter) DetectContours(
	_ context.Context,
	_ image.Rectangle,
) ([]*element.Element, error) {
	return nil, derrors.New(
		derrors.CodeNotSupported,
		"contour element detection needs a screen capture backend, which this platform lacks",
	)
}

// CaptureScreen reports not-supported: this slot has no capture backend.
// Linux has one in adapter_linux.go and Windows in adapter_windows.go.
func (a *Adapter) CaptureScreen(_ context.Context) (*image.RGBA, error) {
	return nil, derrors.New(
		derrors.CodeNotSupported,
		"screen capture is not implemented on this platform",
	)
}

// Health reports not-supported, which is how the hint pipeline learns the
// vision strategy is unavailable here.
func (a *Adapter) Health(_ context.Context) error {
	return derrors.New(
		derrors.CodeNotSupported,
		"the vision strategy is not implemented on this platform",
	)
}
