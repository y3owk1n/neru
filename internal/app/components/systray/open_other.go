//go:build !darwin && !windows

package systray

import (
	"context"
	"os/exec"
)

// Opens a URL or file path with xdg-open (Linux and other XDG desktops).
// Does not validate the target or wait for the launched app.
func openExternal(ctx context.Context, target string) error {
	return exec.CommandContext(ctx, "xdg-open", target).Run()
}
