//go:build !darwin && !linux && !windows

package platform

import (
	"context"

	"github.com/y3owk1n/neru/internal/derrors"
)

// OpenExternal is the non-target fallback: no handler is known here.
func OpenExternal(_ context.Context, _ string) error {
	return derrors.New(derrors.CodeNotSupported, "opening external targets is not supported")
}
