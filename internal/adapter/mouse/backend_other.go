//go:build !darwin && !linux && !windows

package mouse

import (
	"image"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// platformBackend reports that this platform has no mouse-injection path. No
// shipped target lands here.
func platformBackend() backend {
	refuse := func(image.Point, action.MouseButton, action.Modifiers) error {
		return errNotSupported()
	}

	return backend{
		isDown:     func(action.MouseButton) bool { return false },
		releaseAll: func() {},
		click:      refuse,
		down:       refuse,
		up:         refuse,
		move:       func(image.Point) error { return errNotSupported() },
		scroll: func(int, int, action.Modifiers) error {
			return errNotSupported()
		},
	}
}

func errNotSupported() error {
	return derrors.New(
		derrors.CodeNotSupported,
		"mouse injection is not supported on this platform",
	)
}
