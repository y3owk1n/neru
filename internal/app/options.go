package app

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
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

// WithWrittenConfig records what the configuration passed to WithConfig was
// written as, before the loader settled its derived values — see
// [config.LoadResult.Written]. It is what `neru config set` applies a change
// to, so that a change to a source option can be derived from again.
//
// Optional: without it the app derives from the resolved configuration
// instead, which normalizes what was typed but cannot re-infer.
func WithWrittenConfig(cfg *config.Config) Option {
	return func(a *App) error {
		a.writtenConfig = cfg

		return nil
	}
}

// WithConfigWarnings carries the warnings the launch-time load found, for the
// app to log once its logger exists. The load runs before there is a logger to
// hand it, and a warning nobody prints is a warning nobody can act on (ADR
// 0002); a hot reload logs its own through the config service.
func WithConfigWarnings(warnings []string) Option {
	return func(a *App) error {
		a.configWarnings = warnings

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

// WithOverlayPort sets the overlay implementation the app draws through.
//
// It is the one seam between the app and the overlay: a caller that brings its
// own port brings its own screen, and no native backend is built behind it.
// Left unset, the composition root builds the platform backend and the adapter
// over it during startup.
func WithOverlayPort(overlayPort ports.OverlayPort) Option {
	return func(a *App) error {
		a.overlayPort = overlayPort

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

// WithAccessibility sets the accessibility port implementation.
// Note: accessibility can be nil, will be initialized during app startup if not provided.
func WithAccessibility(accessibility ports.AccessibilityPort) Option {
	return func(a *App) error {
		a.accessibility = accessibility

		return nil
	}
}

// WithTextInput sets the text input port implementation.
// Note: textInput can be nil, will be initialized during app startup if not provided.
func WithTextInput(textInput ports.TextInputPort) Option {
	return func(a *App) error {
		a.textInput = textInput

		return nil
	}
}

// WithSystemPort sets the system port implementation.
// Note: systemPort can be nil, will be initialized during app startup if not provided.
func WithSystemPort(systemPort ports.SystemPort) Option {
	return func(a *App) error {
		a.systemPort = systemPort

		return nil
	}
}
