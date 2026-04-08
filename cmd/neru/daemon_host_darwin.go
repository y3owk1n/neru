//go:build darwin

package main

import (
	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/core/infra/systray"
)

type darwinDaemonHost struct{}

func newDaemonHost() daemonHost {
	return darwinDaemonHost{}
}

func (darwinDaemonHost) Run(application *app.App) error {
	runDone := make(chan error, 1)

	go func() {
		err := application.Run()
		if err != nil {
			systray.Quit()
		}

		runDone <- err
	}()

	systrayComponent := application.GetSystrayComponent()
	if systrayComponent != nil {
		systray.Run(systrayComponent.OnReady, systrayComponent.OnExit)
	} else {
		systray.RunHeadless(func() {}, func() {
			application.Cleanup()
		})
	}

	select {
	case err := <-runDone:
		return err
	default:
		return nil
	}
}
