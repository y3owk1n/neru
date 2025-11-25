package appwatcher

import (
	"sync"

	"github.com/y3owk1n/neru/internal/infra/bridge"
	"go.uber.org/zap"
)

// AppCallback defines the function signature for application event handlers.
// It receives the application name and bundle identifier as parameters.
type AppCallback func(appName string, bundleID string)

// Watcher monitors application lifecycle events on macOS and dispatches callbacks.
// It tracks application launches, terminations, activations, deactivations, and screen changes.
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

// NewWatcher creates and initializes a new application watcher instance.
// The watcher is ready to register callbacks and start monitoring immediately.
func NewWatcher(logger *zap.Logger) *Watcher {
	watcher := &Watcher{
		logger: logger,
	}

	bridge.SetAppWatcher(bridge.AppWatcher(watcher))

	return watcher
}

// Start begins monitoring application lifecycle events.
// Events will be dispatched to registered callbacks once monitoring starts.
func (w *Watcher) Start() {
	w.logger.Debug("App watcher: Starting")
	bridge.StartAppWatcher()
}

// Stop halts application lifecycle event monitoring.
// No further events will be dispatched after stopping.
func (w *Watcher) Stop() {
	w.logger.Debug("App watcher: Stopping")
	bridge.StopAppWatcher()
}

// OnScreenParametersChanged registers a callback for screen parameter change events.
// The callback is executed when display configuration changes (resolution, arrangement, etc.).
func (w *Watcher) OnScreenParametersChanged(callback func()) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.screenChangeCallbacks = append(w.screenChangeCallbacks, callback)
}

// OnTerminate registers a callback for application termination events.
// The callback is executed when a monitored application terminates.
func (w *Watcher) OnTerminate(callback AppCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.terminateCallbacks = append(w.terminateCallbacks, callback)
}

// OnActivate registers a callback for application activation events.
// The callback is executed when a monitored application becomes active.
func (w *Watcher) OnActivate(callback AppCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.activateCallbacks = append(w.activateCallbacks, callback)
}

// OnDeactivate registers a callback for application deactivation events.
// The callback is executed when a monitored application loses focus.
func (w *Watcher) OnDeactivate(callback AppCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.deactivateCallbacks = append(w.deactivateCallbacks, callback)
}

// HandleLaunch processes application launch events from the Objective-C bridge.
// It dispatches the event to all registered launch callbacks.
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

// HandleTerminate processes application termination events from the Objective-C bridge.
// It dispatches the event to all registered termination callbacks.
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

// HandleActivate processes application activation events from the Objective-C bridge.
// It dispatches the event to all registered activation callbacks.
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

// HandleDeactivate processes application deactivation events from the Objective-C bridge.
// It dispatches the event to all registered deactivation callbacks.
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

// HandleScreenParametersChanged processes screen parameter change events from the Objective-C bridge.
// It dispatches the event to all registered screen change callbacks.
func (w *Watcher) HandleScreenParametersChanged() {
	w.logger.Debug("App watcher: Screen parameters changed")

	w.mu.RLock()
	defer w.mu.RUnlock()

	for _, callback := range w.screenChangeCallbacks {
		callback()
	}
}
