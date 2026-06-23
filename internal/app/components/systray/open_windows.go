//go:build windows

// internal/app/components/systray/open_windows.go
// Opens a URL or file path with the Windows default handler via rundll32.
// Does not validate the target or wait for the launched app.

package systray

import (
	"context"
	"os/exec"
)

func openExternal(ctx context.Context, target string) error {
	// rundll32 url.dll,FileProtocolHandler handles both URLs and file paths
	// using the registered default application, with no console window flash.
	return exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", target).Run()
}
