// Package appwatcher provides application lifecycle monitoring functionality.
package appwatcher

import (
	"sync"

	"github.com/y3owk1n/neru/internal/bridge"
	"go.uber.org/zap"
)

// Package appwatcher provides functionality for monitoring application lifecycle events
// such as launches, terminations, activations, and deactivations on macOS.

// AppCallback is a callback function type for application events.
type AppCallback func(appName string, bundleID string)

// Watcher represents an application watcher.
type Watcher struct {
	mu sync.RWMutex
	// Callbacks for different events
	launchCallbacks       []AppCallback
	terminateCallbacks    []AppCallback
	activateCallbacks     []AppCallback
	deactivateCallbacks   []AppCallback
	screenChangeCallbacks []func()
	logger                *zap.Logger
}

// NewWatcher creates a new application watcher.
func NewWatcher(logger *zap.Logger) *Watcher {
	watcher := &Watcher{
		logger: logger,
	}

	bridge.SetAppWatcher(bridge.AppWatcher(watcher))

	return watcher
}

// Start starts the application watcher.
func (w *Watcher) Start() {
	w.logger.Debug("App watcher: Starting")
	bridge.StartAppWatcher()
}

// Stop stops the application watcher.
func (w *Watcher) Stop() {
	w.logger.Debug("App watcher: Stopping")
	bridge.StopAppWatcher()
}

// OnScreenParametersChanged registers a callback for screen parameter change events.
func (w *Watcher) OnScreenParametersChanged(callback func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.screenChangeCallbacks = append(w.screenChangeCallbacks, callback)
}

// OnTerminate registers a callback for application termination events.
func (w *Watcher) OnTerminate(callback AppCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.terminateCallbacks = append(w.terminateCallbacks, callback)
}

// OnActivate registers a callback for application activation events.
func (w *Watcher) OnActivate(callback AppCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.activateCallbacks = append(w.activateCallbacks, callback)
}

// OnDeactivate registers a callback for application deactivation events.
func (w *Watcher) OnDeactivate(callback AppCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.deactivateCallbacks = append(w.deactivateCallbacks, callback)
}

// HandleLaunch is called from the bridge when an application launches.
func (w *Watcher) HandleLaunch(appName, bundleID string) {
	w.logger.Debug("App watcher: Application launched",
		zap.String("app_name", appName),
		zap.String("bundle_id", bundleID))
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, callback := range w.launchCallbacks {
		callback(appName, bundleID)
	}
}

// HandleTerminate is called from the bridge when an application terminates.
func (w *Watcher) HandleTerminate(appName, bundleID string) {
	w.logger.Debug("App watcher: Application terminated",
		zap.String("app_name", appName),
		zap.String("bundle_id", bundleID))
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, callback := range w.terminateCallbacks {
		callback(appName, bundleID)
	}
}

// HandleActivate is called from the bridge when an application is activated.
func (w *Watcher) HandleActivate(appName, bundleID string) {
	w.logger.Debug("App watcher: Application activated",
		zap.String("app_name", appName),
		zap.String("bundle_id", bundleID))
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, callback := range w.activateCallbacks {
		callback(appName, bundleID)
	}
}

// HandleDeactivate is called from the bridge when an application is deactivated.
func (w *Watcher) HandleDeactivate(appName, bundleID string) {
	w.logger.Debug("App watcher: Application deactivated",
		zap.String("app_name", appName),
		zap.String("bundle_id", bundleID))
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, callback := range w.deactivateCallbacks {
		callback(appName, bundleID)
	}
}

// HandleScreenParametersChanged is called from the bridge when display parameters change.
func (w *Watcher) HandleScreenParametersChanged() {
	w.logger.Debug("App watcher: Screen parameters changed")
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, callback := range w.screenChangeCallbacks {
		callback()
	}
}
