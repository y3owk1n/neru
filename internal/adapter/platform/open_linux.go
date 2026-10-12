//go:build linux

package platform

import (
	"context"
	"os/exec"

	"github.com/y3owk1n/neru/internal/derrors"
)

// OpenExternal opens a URL or file path with the Linux desktop's default
// handler. It does not validate the target or wait for the launched app.
func OpenExternal(ctx context.Context, target string) error {
	command := exec.CommandContext(ctx, "xdg-open", target)

	err := command.Start()
	if err != nil {
		return derrors.Wrap(err, derrors.CodeExecFailed, "failed to launch the system open handler")
	}

	// Reap it, or every call leaves a zombie process for the daemon's lifetime.
	go func() { _ = command.Wait() }()

	return nil
}
