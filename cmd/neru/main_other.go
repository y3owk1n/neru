//go:build !darwin

package main

import "github.com/y3owk1n/neru/internal/cli"

// main starts the CLI.
//
// It lives in a per-platform file because macOS must pin the OS thread before
// any goroutine starts; nothing here needs to. main.go holds the shared daemon
// logic.
func main() {
	cli.LaunchFunc = LaunchDaemon

	cli.Execute()
}
