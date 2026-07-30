//go:build windows

package config

import (
	"os"
	"path/filepath"
)

func applyPlatformDefaults(cfg *Config) {
	// Windows-specific exec shell defaults (absolute path required for validation)
	// %SystemRoot% is always set on Windows (e.g. C:\Windows).
	cfg.General.ExecShell = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	cfg.General.ExecShellArgs = []string{"/c"}
}
