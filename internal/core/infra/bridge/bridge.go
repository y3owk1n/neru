package bridge

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework ApplicationServices -framework Cocoa -framework Carbon -framework CoreGraphics
#include "accessibility.h"
#include "overlay.h"
#include "hotkeys.h"
#include "eventtap.h"
#include "appwatcher.h"
#include "alert.h"
#include "secureinput.h"
#include "keymap.h"
#include <stdlib.h>

CGRect getActiveScreenBounds();
*/
import "C"

import (
	"image"
	"sync"
	"unsafe"

	"go.uber.org/zap"
)

// SetApplicationAttribute toggles an accessibility attribute for the application identified by PID.
func SetApplicationAttribute(pid int, attribute string, value bool) bool {
	log := getLogger()
	log.Debug("Bridge: Setting application attribute",
		zap.Int("pid", pid),
		zap.String("attribute", attribute),
		zap.Bool("value", value))

	cAttr := C.CString(attribute)
	defer C.free(unsafe.Pointer(cAttr)) //nolint:nlreturn

	var cValue C.int
	if value {
		cValue = 1
	}

	result := C.setApplicationAttribute(C.int(pid), cAttr, cValue)

	if result == 1 {
		log.Debug("Bridge: Application attribute set successfully",
			zap.Int("pid", pid),
			zap.String("attribute", attribute))
	} else {
		log.Warn("Bridge: Failed to set application attribute",
			zap.Int("pid", pid),
			zap.String("attribute", attribute))
	}

	return result == 1
}

// HasClickAction determines if an accessibility element has the AXPress action available.
func HasClickAction(element unsafe.Pointer) bool {
	log := getLogger()

	if element == nil {
		log.Debug("Bridge: HasClickAction called with nil element")

		return false
	}

	result := C.hasClickAction(element) //nolint:nlreturn

	log.Debug("Bridge: HasClickAction result",
		zap.Bool("has_action", result == 1))

	return result == 1
}

// appWatcher is the global application watcher instance.
var (
	appWatcher   AppWatcherInterface
	appWatcherMu sync.RWMutex
)

// AppWatcherInterface interface defines callbacks for application lifecycle events.
type AppWatcherInterface interface {
	HandleLaunch(appName, bundleID string)
	HandleTerminate(appName, bundleID string)
	HandleActivate(appName, bundleID string)
	HandleDeactivate(appName, bundleID string)
	HandleScreenParametersChanged()
}

// AppWatcher returns the global application watcher instance.
func AppWatcher() AppWatcherInterface {
	appWatcherMu.RLock()
	defer appWatcherMu.RUnlock()

	return appWatcher
}

// SetAppWatcher configures the application watcher implementation.
func SetAppWatcher(watcher AppWatcherInterface) {
	log := getLogger()
	log.Debug("Bridge: Setting app watcher")

	appWatcherMu.Lock()
	defer appWatcherMu.Unlock()

	appWatcher = watcher
}

// StartAppWatcher begins monitoring application lifecycle events.
func StartAppWatcher() {
	log := getLogger()
	log.Debug("Bridge: Starting app watcher")
	C.startAppWatcher()
}

// StopAppWatcher ceases monitoring application lifecycle events.
func StopAppWatcher() {
	log := getLogger()
	log.Debug("Bridge: Stopping app watcher")
	C.stopAppWatcher()
}

// HandleAppLaunch processes an app launch event.
func HandleAppLaunch(appName, bundleID string) {
	log := getLogger()
	log.Debug("Bridge: handleAppLaunch called")

	appWatcherMu.RLock()
	watcher := appWatcher
	appWatcherMu.RUnlock()

	if watcher != nil {
		log.Debug("Bridge: Forwarding app launch event",
			zap.String("app_name", appName),
			zap.String("bundle_id", bundleID))

		watcher.HandleLaunch(appName, bundleID)
	}
}

//export handleAppLaunch
func handleAppLaunch(cAppName *C.char, cBundleID *C.char) {
	HandleAppLaunch(C.GoString(cAppName), C.GoString(cBundleID))
}

// HandleAppTerminate processes an app termination event.
func HandleAppTerminate(appName, bundleID string) {
	log := getLogger()
	log.Debug("Bridge: handleAppTerminate called")

	appWatcherMu.RLock()
	watcher := appWatcher
	appWatcherMu.RUnlock()

	if watcher != nil {
		log.Debug("Bridge: Forwarding app terminate event",
			zap.String("app_name", appName),
			zap.String("bundle_id", bundleID))

		watcher.HandleTerminate(appName, bundleID)
	}
}

//export handleAppTerminate
func handleAppTerminate(cAppName *C.char, cBundleID *C.char) {
	HandleAppTerminate(C.GoString(cAppName), C.GoString(cBundleID))
}

// HandleAppActivate processes an app activation event.
func HandleAppActivate(appName, bundleID string) {
	log := getLogger()
	log.Debug("Bridge: handleAppActivate called")

	appWatcherMu.RLock()
	watcher := appWatcher
	appWatcherMu.RUnlock()

	if watcher != nil {
		log.Debug("Bridge: Forwarding app activate event",
			zap.String("app_name", appName),
			zap.String("bundle_id", bundleID))

		watcher.HandleActivate(appName, bundleID)
	}
}

//export handleAppActivate
func handleAppActivate(cAppName *C.char, cBundleID *C.char) {
	HandleAppActivate(C.GoString(cAppName), C.GoString(cBundleID))
}

// HandleScreenParametersChanged processes a screen parameters change event.
func HandleScreenParametersChanged() {
	log := getLogger()
	log.Debug("Bridge: handleScreenParametersChanged called")

	appWatcherMu.RLock()
	watcher := appWatcher
	appWatcherMu.RUnlock()

	if watcher != nil {
		log.Debug("Bridge: Forwarding screen parameters changed event")

		watcher.HandleScreenParametersChanged()
	}
}

//export handleScreenParametersChanged
func handleScreenParametersChanged() {
	HandleScreenParametersChanged()
}

// HandleAppDeactivate processes an app deactivation event.
func HandleAppDeactivate(appName, bundleID string) {
	log := getLogger()
	log.Debug("Bridge: handleAppDeactivate called")

	appWatcherMu.RLock()
	watcher := appWatcher
	appWatcherMu.RUnlock()

	if watcher != nil {
		log.Debug("Bridge: Forwarding app deactivate event",
			zap.String("app_name", appName),
			zap.String("bundle_id", bundleID))

		watcher.HandleDeactivate(appName, bundleID)
	}
}

//export handleAppDeactivate
func handleAppDeactivate(cAppName *C.char, cBundleID *C.char) {
	HandleAppDeactivate(C.GoString(cAppName), C.GoString(cBundleID))
}

// ActiveScreenBounds retrieves the screen bounds containing the current mouse cursor position.
func ActiveScreenBounds() image.Rectangle {
	log := getLogger()
	log.Debug("Bridge: ActiveScreenBounds called")

	rect := C.getActiveScreenBounds()
	result := image.Rect(
		int(rect.origin.x),
		int(rect.origin.y),
		int(rect.origin.x+rect.size.width),
		int(rect.origin.y+rect.size.height),
	)

	log.Debug("Bridge: Active screen bounds",
		zap.Int("x", result.Min.X),
		zap.Int("y", result.Min.Y),
		zap.Int("width", result.Dx()),
		zap.Int("height", result.Dy()))

	return result
}

// ShowConfigValidationError displays a native macOS alert for config validation errors.
// Returns true if the user clicked the "Copy Config Path" button.
func ShowConfigValidationError(errorMessage, configPath string) bool {
	log := getLogger()
	log.Debug("Bridge: Showing config validation error alert",
		zap.String("error", errorMessage),
		zap.String("config_path", configPath))

	cError := C.CString(errorMessage)
	defer C.free(unsafe.Pointer(cError)) //nolint:nlreturn

	cPath := C.CString(configPath)
	defer C.free(unsafe.Pointer(cPath)) //nolint:nlreturn

	result := C.showConfigValidationErrorAlert(cError, cPath)

	log.Debug("Bridge: Alert result",
		zap.Int("result", int(result)))

	return result == 2 //nolint:mnd
}

// IsSecureInputEnabled checks if macOS secure input mode is currently enabled.
// Secure input mode is active when a password field or other secure text input is focused.
// When enabled, keyboard events are blocked for security purposes.
func IsSecureInputEnabled() bool {
	log := getLogger()
	result := C.isSecureInputEnabled()

	log.Debug("Bridge: IsSecureInputEnabled check",
		zap.Bool("enabled", result == 1))

	return result == 1
}

// ShowSecureInputNotification displays a macOS notification informing the user
// that mode activation was blocked because secure input is active.
func ShowSecureInputNotification() {
	log := getLogger()
	log.Debug("Bridge: Showing secure input notification")

	C.showSecureInputNotification()
}

// ShowNotification displays a macOS notification with the given title and message.
func ShowNotification(title, message string) {
	log := getLogger()
	log.Debug("Bridge: Showing notification",
		zap.String("title", title),
		zap.String("message", message))

	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle)) //nolint: nlreturn

	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cMessage)) //nolint: nlreturn

	C.showNotification(cTitle, cMessage)
}
