//go:build !darwin

package vision

import (
	"context"
	"io"

	"github.com/y3owk1n/neru/internal/derrors"
)

// ServeTextLines is the text lines helper's side, which only macOS runs.
func ServeTextLines(context.Context, io.Reader, io.Writer) error {
	return derrors.New(derrors.CodeNotSupported, "the text lines helper runs on macOS only")
}
