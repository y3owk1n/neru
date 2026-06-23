//go:build darwin

// internal/app/components/systray/open_darwin.go
// Opens a URL or file path with the macOS default handler.
// Does not validate the target or wait for the launched app.

package systray

import (
	"context"
	"os/exec"
)

func openExternal(ctx context.Context, target string) error {
	return exec.CommandContext(ctx, "/usr/bin/open", target).Run()
}
