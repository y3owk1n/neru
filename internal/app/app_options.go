package app

import (
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// Option is a functional option for configuring an App instance.
type Option func(*App) error

// WithConfig sets the application configuration.
func WithConfig(cfg *config.Config) Option {
	return func(a *App) error {
		a.config = cfg

		return nil
	}
}

// WithConfigPath sets the configuration file path.
func WithConfigPath(path string) Option {
	return func(a *App) error {
		a.ConfigPath = path

		return nil
	}
}

// WithLogger sets the application logger.
func WithLogger(logger *zap.Logger) Option {
	return func(a *App) error {
		a.logger = logger

		return nil
	}
}

// WithEventTap sets the event tap implementation.
func WithEventTap(eventTap ports.EventTapPort) Option {
	return func(a *App) error {
		a.eventTap = eventTap

		return nil
	}
}

// WithIPCServer sets the IPC server implementation.
func WithIPCServer(ipcServer ports.IPCPort) Option {
	return func(a *App) error {
		a.ipcServer = ipcServer

		return nil
	}
}

// WithOverlayManager sets the overlay manager implementation.
func WithOverlayManager(overlayManager OverlayManager) Option {
	return func(a *App) error {
		a.overlayManager = overlayManager

		return nil
	}
}

// WithWatcher sets the app watcher implementation.
func WithWatcher(watcher Watcher) Option {
	return func(a *App) error {
		a.appWatcher = watcher

		return nil
	}
}

// WithHotkeyService sets the hotkey service implementation.
func WithHotkeyService(hotkeyService HotkeyService) Option {
	return func(a *App) error {
		a.hotkeyManager = hotkeyService

		return nil
	}
}
