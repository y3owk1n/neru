//go:build linux

package main

import "github.com/y3owk1n/neru/internal/app"

type linuxDaemonHost struct{}

func newDaemonHost() daemonHost {
	return linuxDaemonHost{}
}

func (linuxDaemonHost) Run(application *app.App) error {
	defer application.Cleanup()

	return application.Run()
}
