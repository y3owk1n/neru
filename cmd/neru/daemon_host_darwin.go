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
	// Ensure cleanup runs even if the systray OnExit callback does not fire
	// (e.g. Cocoa event-loop crash). Cleanup is idempotent (sync.Once), so
	// the duplicate call from OnExit is harmless.
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
		systray.RunHeadless(func() {}, func() {
			application.Cleanup()
		})
	}

	// Unblock waitForShutdown so the goroutine can return.
	// Stop is idempotent (protected by sync.Once), so this is safe even if
	// the app already stopped itself.
	application.Stop()

	return <-runDone
}
