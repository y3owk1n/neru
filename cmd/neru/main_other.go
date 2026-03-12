//go:build !darwin

// Package main is the entry point for the Neru application.
package main

import "github.com/y3owk1n/neru/internal/cli"

func main() {
	cli.LaunchFunc = LaunchDaemon

	cli.Execute()
}
