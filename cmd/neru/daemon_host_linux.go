//go:build linux

package main

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
	"github.com/y3owk1n/neru/internal/core/infra/systray"
)

type linuxDaemonHost struct{}

func newDaemonHost() daemonHost {
	return linuxDaemonHost{}
}

func (linuxDaemonHost) Run(application *app.App) error {
	// Ensure cleanup runs even if the systray OnExit callback does not fire.
	// Cleanup is idempotent (sync.Once), so the duplicate call from OnExit is
	// harmless. Mirrors the darwin/windows daemon hosts.
	defer application.Cleanup()

	// On KDE/KWin, input is injected via libei through the RemoteDesktop
	// portal, which shows a one-time consent prompt. Warm it up at startup (in
	// the background) so the prompt appears now rather than blocking the first
	// action past the IPC timeout. No-op on wlroots / X11.
	go func() {
		err := linux.WarmWaylandInput()
		if err != nil {
			// On KDE this means the libei/RemoteDesktop session did not
			// establish (consent not approved in time, or portal unavailable),
			// so the first action falls back to a slow lazy connect that can
			// exceed the IPC client timeout.
			application.Logger().Warn(
				"Wayland input warm-up failed; first action may be slow until the RemoteDesktop consent prompt is approved",
				zap.Error(err),
			)

			return
		}

		application.Logger().Info("Wayland input warm-up complete")
	}()

	runDone := make(chan error, 1)

	go func() {
		err := application.Run()
		if err != nil {
			systray.Quit()
		}

		runDone <- err
	}()

	// The systray event loop is the daemon's main loop. With a systray
	// component the tray icon + menu are exported over D-Bus (SNI); without one
	// RunHeadless keeps the process alive until Quit. Either way this blocks
	// until systray.Quit() fires (Quit menu item, app.Run error, or signal).
	systrayComponent := application.GetSystrayComponent()
	if systrayComponent != nil {
		systray.Run(systrayComponent.OnReady, systrayComponent.OnExit)
	} else {
		systray.RunHeadless(func() {}, func() {})
	}

	// Unblock the app goroutine so it can return. Stop is idempotent
	// (protected by sync.Once), so this is safe even if the app already stopped.
	application.Stop()

	return <-runDone
}
