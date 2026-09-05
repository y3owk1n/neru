//go:build windows

package platform

import (
	"context"
	"os/exec"

	"github.com/y3owk1n/neru/internal/derrors"
)

// OpenExternal opens a URL or file path with the Windows default handler via
// rundll32. It does not validate the target or wait for the launched app.
func OpenExternal(ctx context.Context, target string) error {
	// rundll32 url.dll,FileProtocolHandler handles both URLs and file paths
	// using the registered default application, with no console window flash.
	err := exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", target).Run()
	if err != nil {
		return derrors.Wrap(err, derrors.CodeExecFailed, "failed to launch the system open handler")
	}

	return nil
}
