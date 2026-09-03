//go:build !darwin && !linux && !windows

package systray

import (
	"context"

	"github.com/y3owk1n/neru/internal/derrors"
)

func openExternal(_ context.Context, _ string) error {
	return derrors.New(derrors.CodeNotSupported, "opening external targets is not supported")
}
