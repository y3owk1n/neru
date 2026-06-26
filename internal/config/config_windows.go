//go:build windows

package config

import (
	"os"
	"path/filepath"
)

func applyPlatformDefaults(cfg *Config) {
	// Windows-specific exec shell defaults (absolute path required for validation)
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = "C:\\Windows"
	}
	cfg.General.ExecShell = filepath.Join(systemRoot, "System32", "cmd.exe")
	cfg.General.ExecShellArgs = []string{"/c"}

	// UIA control type roles for clickable elements
	cfg.Hints.ClickableRoles = append(cfg.Hints.ClickableRoles,
		"Button",
		"CheckBox",
		"RadioButton",
		"Hyperlink",
		"ComboBox",
		"Edit",
		"Slider",
		"TabItem",
		"MenuItem",
		"DataItem",
	)
}
