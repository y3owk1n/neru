package app

import (
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// Option is a functional option for configuring an App instance.
type Option func(*App) error

// WithConfig sets the application configuration.
// Note: cfg can be nil, in which case default configuration will be used.
func WithConfig(cfg *config.Config) Option {
	return func(a *App) error {
		// Config can be nil - system will use defaults
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
		// Logger can be nil - system will initialize a default logger if needed
		a.logger = logger

		return nil
	}
}

// WithEventTap sets the event tap implementation.
// Note: eventTap can be nil, will be initialized during app startup if not provided.
func WithEventTap(eventTap ports.EventTapPort) Option {
	return func(a *App) error {
		a.eventTap = eventTap

		return nil
	}
}

// WithIPCServer sets the IPC server implementation.
// Note: ipcServer can be nil, will be initialized during app startup if not provided.
func WithIPCServer(ipcServer ports.IPCPort) Option {
	return func(a *App) error {
		a.ipcServer = ipcServer

		return nil
	}
}

// WithOverlayManager sets the overlay manager implementation.
// Note: overlayManager can be nil, will be initialized during app startup if not provided.
func WithOverlayManager(overlayManager OverlayManager) Option {
	return func(a *App) error {
		a.overlayManager = overlayManager

		return nil
	}
}

// WithWatcher sets the app watcher implementation.
// Note: watcher can be nil, will be initialized during app startup if not provided.
func WithWatcher(watcher Watcher) Option {
	return func(a *App) error {
		a.appWatcher = watcher

		return nil
	}
}

// WithHotkeyService sets the hotkey service implementation.
// Note: hotkeyService can be nil, will be initialized during app startup if not provided.
func WithHotkeyService(hotkeyService HotkeyService) Option {
	return func(a *App) error {
		a.hotkeyManager = hotkeyService

		return nil
	}
}
