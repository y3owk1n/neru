//go:build linux

package cli

import (
	"context"
	"os/exec"

	"github.com/y3owk1n/neru/internal/buildinfo"
	"github.com/y3owk1n/neru/internal/derrors"
)

func openDocsPage(path string) error {
	url := buildinfo.DocsURL(path, buildinfo.Version)

	err := exec.CommandContext(context.Background(), "xdg-open", url).Start()
	if err != nil {
		return derrors.Wrap(err, derrors.CodeExecFailed, "failed to open documentation in browser")
	}

	return nil
}
