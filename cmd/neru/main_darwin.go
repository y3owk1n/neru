//go:build darwin

package main

import (
	"runtime"

	"github.com/y3owk1n/neru/internal/cli"
)

func main() {
	// LockOSThread must be called before any goroutines are started.
	// macOS Cocoa (NSApplication, NSWindow, etc.) requires all UI calls
	// to happen on the thread that called LockOSThread — the OS main thread.
	// This is a macOS-only requirement; Linux/Windows do not need it.
	runtime.LockOSThread()

	cli.LaunchFunc = LaunchDaemon

	cli.Execute()
}
