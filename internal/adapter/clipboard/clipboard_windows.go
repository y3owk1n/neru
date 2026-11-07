//go:build windows

package clipboard

import (
	"context"

	"github.com/atotto/clipboard"

	"github.com/y3owk1n/neru/internal/derrors"
)

// writeText sets the clipboard through the Win32 clipboard API, as Unicode
// text. The library retries for a second while another application holds the
// clipboard open, which keeps the write within the adapter's deadline.
func writeText(_ context.Context, text string) error {
	err := clipboard.WriteAll(text)
	if err != nil {
		return derrors.Wrap(err, derrors.CodeActionFailed, "failed to write the clipboard")
	}

	return nil
}
