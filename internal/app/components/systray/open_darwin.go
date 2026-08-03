//go:build darwin

package systray

import (
	"context"
	"os/exec"
)

// Opens a URL or file path with the macOS default handler.
// Does not validate the target or wait for the launched app.
func openExternal(ctx context.Context, target string) error {
	return exec.CommandContext(ctx, "/usr/bin/open", target).Run()
}
