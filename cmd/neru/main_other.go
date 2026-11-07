//go:build !darwin

package main

import (
	visionAdapter "github.com/y3owk1n/neru/internal/adapter/vision"
	"github.com/y3owk1n/neru/internal/cli"
)

// main starts the CLI.
//
// It lives in a per-platform file because macOS must pin the OS thread before
// any goroutine starts, and nothing here needs to. main.go holds the shared
// daemon logic.
func main() {
	cli.LaunchFunc = LaunchDaemon
	cli.ReadTextLinesFunc = visionAdapter.ServeTextLines

	cli.Execute()
}
