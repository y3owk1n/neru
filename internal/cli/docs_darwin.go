//go:build darwin

package cli

import (
	"context"
	"os/exec"

	"github.com/y3owk1n/neru/internal/buildinfo"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

func openDocsPage(path string) error {
	url := buildinfo.DocsURL(path, buildinfo.Version)

	err := exec.CommandContext(context.Background(), "/usr/bin/open", url).Run()
	if err != nil {
		return derrors.Wrap(err, derrors.CodeExecFailed, "failed to open documentation in browser")
	}

	return nil
}
