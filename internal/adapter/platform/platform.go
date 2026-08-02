package platform

import (
	"runtime"
)

// OS represents the operating system.
type OS string

const (
	// Darwin represents macOS.
	Darwin OS = "darwin"
	// Linux represents Linux.
	Linux OS = "linux"
	// Windows represents Windows.
	Windows OS = "windows"
	// Unknown represents an unknown operating system.
	Unknown OS = "unknown"
)

// CurrentOS returns the current operating system.
func CurrentOS() OS {
	switch runtime.GOOS {
	case "darwin":
		return Darwin
	case "linux":
		return Linux
	case "windows":
		return Windows
	default:
		return Unknown
	}
}

// IsDarwin returns true if the current OS is macOS.
func IsDarwin() bool {
	return CurrentOS() == Darwin
}

// IsLinux returns true if the current OS is Linux.
func IsLinux() bool {
	return CurrentOS() == Linux
}

// IsWindows returns true if the current OS is Windows.
func IsWindows() bool {
	return CurrentOS() == Windows
}
