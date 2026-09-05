//go:build linux && !cgo

package linux

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
)

// WarmEvdevProxy reports CodeNotSupported without cgo, which evdev needs: there
// is no proxy to build, and the first mode says what it fell back to.
func WarmEvdevProxy(_ *zap.Logger) error {
	return derrors.New(
		derrors.CodeNotSupported,
		"Wayland keyboard capture requires CGO-enabled Linux builds",
	)
}
