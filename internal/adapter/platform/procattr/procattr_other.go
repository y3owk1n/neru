//go:build !windows

package procattr

import "os/exec"

// HideConsole is a no-op outside Windows. Consoles are a Windows concept. On
// macOS and Linux a subprocess inherits its parent's terminal or has no
// terminal at all, and neither case creates a window to suppress.
func HideConsole(command *exec.Cmd) {}
