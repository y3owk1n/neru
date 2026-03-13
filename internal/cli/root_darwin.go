//go:build darwin

package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// isRunningFromAppBundle returns true when the binary is inside a macOS .app bundle.
// This is used to auto-launch the daemon when double-clicked from Finder.
func isRunningFromAppBundle() bool {
	execPath, err := os.Executable()
	if err != nil {
		return false
	}

	realPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		realPath = execPath
	}

	return strings.Contains(realPath, ".app/Contents/MacOS")
}
