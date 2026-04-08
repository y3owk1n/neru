//go:build windows

package main

import "github.com/y3owk1n/neru/internal/app"

type windowsDaemonHost struct{}

func newDaemonHost() daemonHost {
	return windowsDaemonHost{}
}

func (windowsDaemonHost) Run(application *app.App) error {
	defer application.Cleanup()

	return application.Run()
}
