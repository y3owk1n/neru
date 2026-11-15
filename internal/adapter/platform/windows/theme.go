//go:build windows

package windows

import (
	"golang.org/x/sys/windows/registry"
)

// This file reads the Windows app theme. It does not implement
// ports.SystemPort, and system.go delegates dark mode here.

// AppsUseDarkTheme reports whether Windows app dark mode is active.
func AppsUseDarkTheme() bool {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return false
	}

	defer func() { _ = key.Close() }()

	value, _, err := key.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return false
	}

	return value == 0
}
