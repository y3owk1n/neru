//go:build windows

package main

import (
	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/core/infra/systray"
)

type windowsDaemonHost struct{}

func newDaemonHost() daemonHost {
	return windowsDaemonHost{}
}

func (windowsDaemonHost) Run(application *app.App) error {
	// Cleanup is idempotent (sync.Once); the duplicate call from the systray
	// OnExit callback is harmless.
	defer application.Cleanup()

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
		systray.RunHeadless(func() {}, func() {})
	}

	// Unblock waitForShutdown so the run goroutine can return. Stop is
	// idempotent, so this is safe even if the app already stopped itself.
	application.Stop()

	return <-runDone
}
