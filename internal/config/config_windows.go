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

	// AX-style roles produced by the Windows UI Automation enumerator
	// (see mapControlType in uia_windows.go). These must match the roles
	// assigned during element discovery, not the raw UIA control type names.
	cfg.Hints.ClickableRoles = append(cfg.Hints.ClickableRoles,
		"AXButton",
		"AXCheckBox",
		"AXRadioButton",
		"AXLink",
		"AXComboBox",
		"AXTextField",
		"AXSlider",
		"AXTabButton",
		"AXMenuItem",
		"AXCell",
		"AXRow",
		"AXIncrementor",
	)
}
