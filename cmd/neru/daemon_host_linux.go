//go:build linux

package main

import (
	"go.uber.org/zap"

	eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/adapter/systray"
	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/derrors"
)

type linuxDaemonHost struct{}

func newDaemonHost() daemonHost {
	return linuxDaemonHost{}
}

func (linuxDaemonHost) Run(application *app.App) error {
	// Cleanup is idempotent (sync.Once), so this backstop for a skipped systray
	// OnExit is harmless. Mirrors the darwin/windows daemon hosts.
	defer application.Cleanup()

	// Hold the keyboards now, while nothing is pressed, rather than under the
	// binding that launches the first mode (WarmEvdevProxy). Without cgo,
	// /dev/input or /dev/uinput access the first mode says what it fell back to.
	if backend := platform.DetectLinuxBackend(); backend.IsWayland() {
		logger := application.Logger()

		go func() {
			err := eventtaplinux.WarmEvdevProxy(logger)
			if err != nil && !derrors.IsNotSupported(err) {
				logger.Warn(
					"Wayland keyboard proxy unavailable at launch; modes will use the overlay's keyboard focus",
					zap.Error(err),
				)
			}
		}()

		// Create the scroll wheel now, so its settle wait happens at launch and not
		// during the first scroll. A wheel that cannot be created is reported by
		// the first scroll that falls back. The scroll path never uses the wheel
		// on an unrecognized Wayland compositor, so none is created there.
		if backend != platform.BackendWaylandOther {
			go func() {
				err := eventtaplinux.WarmScrollDevice()
				if err != nil {
					logger.Debug("Scroll wheel not created at launch", zap.Error(err))
				}
			}()
		}
	}

	// On KDE, libei input goes through the RemoteDesktop portal's one-time
	// consent prompt. Warm it up in the background so the prompt appears now
	// rather than blocking the first action past the IPC timeout. No-op on
	// wlroots / X11.
	go func() {
		err := linux.WarmWaylandInput()
		if err != nil {
			// The session did not establish (consent not approved in time, or no
			// portal), so the first action falls back to a slow lazy connect that
			// can exceed the IPC client timeout.
			application.Logger().Warn(
				"Wayland input warm-up failed; first action may be slow until the RemoteDesktop consent prompt is approved",
				zap.Error(err),
			)

			return
		}

		if !linux.HasKeyboard() {
			application.Logger().Warn(
				"Wayland input warm-up complete but RemoteDesktop portal did not grant a " +
					"keyboard device; \"action feed\" and sticky modifiers will be unavailable. " +
					"Restart neru and check \"Enable keyboard\" in the consent prompt; if the " +
					"prompt does not appear, run `flatpak permission-remove kde-authorized " +
					"remote-desktop \"\"` first to clear the saved grant",
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

	// The systray loop is the daemon's main loop, exporting the tray over D-Bus
	// (SNI) or, without a systray component, running headless. It blocks until
	// systray.Quit() (Quit menu item, app.Run error, or signal).
	systrayComponent := application.GetSystrayComponent()
	if systrayComponent != nil {
		systray.Run(systrayComponent.OnReady, systrayComponent.OnExit)
	} else {
		systray.RunHeadless(func() {}, func() {})
	}

	// Unblock the app goroutine. Stop is idempotent (sync.Once).
	application.Stop()

	return <-runDone
}
