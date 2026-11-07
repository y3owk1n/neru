//go:build !darwin && !linux && !windows

package clipboard

import (
	"context"

	"github.com/y3owk1n/neru/internal/derrors"
)

// writeText reports that this platform has no clipboard path. No shipped
// target lands here.
func writeText(_ context.Context, _ string) error {
	return derrors.New(
		derrors.CodeNotSupported,
		"copying to the clipboard is only supported on macOS, Linux and Windows",
	)
}
