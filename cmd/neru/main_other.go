//go:build !darwin

package main

import "github.com/y3owk1n/neru/internal/cli"

func main() {
	cli.LaunchFunc = LaunchDaemon

	cli.Execute()
}
