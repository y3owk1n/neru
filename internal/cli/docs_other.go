//go:build !darwin

package cli

import (
	"github.com/y3owk1n/neru/internal/derrors"
)

func openDocsPage(path string) error {
	return derrors.New(derrors.CodeNotSupported, "open documentation is only implemented for macOS")
}
