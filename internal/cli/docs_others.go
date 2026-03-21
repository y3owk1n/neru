//go:build !darwin

package cli

import (
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

func openDocsPage(path string) error {
	return derrors.New(derrors.CodeNotSupported, "open documentation is only implemented for macOS")
}
