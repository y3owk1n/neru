//go:build darwin

package main

import (
	"runtime"

	visionAdapter "github.com/y3owk1n/neru/internal/adapter/vision"
	"github.com/y3owk1n/neru/internal/cli"
)

// main starts the CLI, pinning the OS thread first.
//
// It lives in a per-platform file so the pinning happens before any goroutine
// starts. main.go holds only the shared daemon logic.
func main() {
	// Cocoa requires every UI call on the thread that called LockOSThread, so
	// it has to run before anything else spawns a goroutine.
	runtime.LockOSThread()

	cli.LaunchFunc = LaunchDaemon
	cli.ReadTextLinesFunc = visionAdapter.ServeTextLines

	cli.Execute()
}
