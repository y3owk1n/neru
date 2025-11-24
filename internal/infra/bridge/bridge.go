package bridge

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ApplicationServices -framework Cocoa -framework Carbon -framework CoreGraphics
#include "accessibility.h"
#include "overlay.h"
#include "hotkeys.h"
#include "eventtap.h"
#include "appwatcher.h"
#include "alert.h"
#include <stdlib.h>

CGRect getActiveScreenBounds();
*/
import "C"

import (
	"image"
	"unsafe"

	"go.uber.org/zap"
)

// Global logger instance used for bridge package logging.
var bridgeLogger *zap.Logger

// InitializeLogger sets the global logger instance for the bridge package.
func InitializeLogger(logger *zap.Logger) {
	bridgeLogger = logger
}

// SetApplicationAttribute toggles an accessibility attribute for the application identified by PID.
func SetApplicationAttribute(pid int, attribute string, value bool) bool {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: Setting application attribute",
			zap.Int("pid", pid),
			zap.String("attribute", attribute),
			zap.Bool("value", value))
	}

	cAttr := C.CString(attribute)
	defer C.free(unsafe.Pointer(cAttr)) //nolint:nlreturn

	var cValue C.int
	if value {
		cValue = 1
	}

	result := C.setApplicationAttribute(C.int(pid), cAttr, cValue)

	if bridgeLogger != nil {
		if result == 1 {
			bridgeLogger.Debug("Bridge: Application attribute set successfully",
				zap.Int("pid", pid),
				zap.String("attribute", attribute))
		} else {
			bridgeLogger.Warn("Bridge: Failed to set application attribute",
				zap.Int("pid", pid),
				zap.String("attribute", attribute))
		}
	}

	return result == 1
}

// HasClickAction determines if an accessibility element has the AXPress action available.
func HasClickAction(element unsafe.Pointer) bool {
	if element == nil {
		if bridgeLogger != nil {
			bridgeLogger.Debug("Bridge: HasClickAction called with nil element")
		}

		return false
	}

	result := C.hasClickAction(element) //nolint:nlreturn

	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: HasClickAction result",
			zap.Bool("has_action", result == 1))
	}

	return result == 1
}

var appWatcher AppWatcher

// AppWatcher interface defines callbacks for application lifecycle events.
type AppWatcher interface {
	HandleLaunch(appName, bundleID string)
	HandleTerminate(appName, bundleID string)
	HandleActivate(appName, bundleID string)
	HandleDeactivate(appName, bundleID string)
	HandleScreenParametersChanged()
}

// SetAppWatcher configures the application watcher implementation.
func SetAppWatcher(w AppWatcher) {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: Setting app watcher")
	}
	appWatcher = w
}

// StartAppWatcher begins monitoring application lifecycle events.
func StartAppWatcher() {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: Starting app watcher")
	}
	C.startAppWatcher()
}

// StopAppWatcher ceases monitoring application lifecycle events.
func StopAppWatcher() {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: Stopping app watcher")
	}
	C.stopAppWatcher()
}

// HandleAppLaunch processes an app launch event.
func HandleAppLaunch(appName, bundleID string) {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: handleAppLaunch called")
	}

	if appWatcher != nil {
		if bridgeLogger != nil {
			bridgeLogger.Debug("Bridge: Forwarding app launch event",
				zap.String("app_name", appName),
				zap.String("bundle_id", bundleID))
		}

		appWatcher.HandleLaunch(appName, bundleID)
	}
}

//export handleAppLaunch
func handleAppLaunch(cAppName *C.char, cBundleID *C.char) {
	HandleAppLaunch(C.GoString(cAppName), C.GoString(cBundleID))
}

// HandleAppTerminate processes an app termination event.
func HandleAppTerminate(appName, bundleID string) {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: handleAppTerminate called")
	}

	if appWatcher != nil {
		if bridgeLogger != nil {
			bridgeLogger.Debug("Bridge: Forwarding app terminate event",
				zap.String("app_name", appName),
				zap.String("bundle_id", bundleID))
		}

		appWatcher.HandleTerminate(appName, bundleID)
	}
}

//export handleAppTerminate
func handleAppTerminate(cAppName *C.char, cBundleID *C.char) {
	HandleAppTerminate(C.GoString(cAppName), C.GoString(cBundleID))
}

// HandleAppActivate processes an app activation event.
func HandleAppActivate(appName, bundleID string) {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: handleAppActivate called")
	}

	if appWatcher != nil {
		if bridgeLogger != nil {
			bridgeLogger.Debug("Bridge: Forwarding app activate event",
				zap.String("app_name", appName),
				zap.String("bundle_id", bundleID))
		}

		appWatcher.HandleActivate(appName, bundleID)
	}
}

//export handleAppActivate
func handleAppActivate(cAppName *C.char, cBundleID *C.char) {
	HandleAppActivate(C.GoString(cAppName), C.GoString(cBundleID))
}

// HandleScreenParametersChanged processes a screen parameters change event.
func HandleScreenParametersChanged() {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: handleScreenParametersChanged called")
	}

	if appWatcher != nil {
		if bridgeLogger != nil {
			bridgeLogger.Debug("Bridge: Forwarding screen parameters changed event")
		}

		go appWatcher.HandleScreenParametersChanged()
	}
}

//export handleScreenParametersChanged
func handleScreenParametersChanged() {
	HandleScreenParametersChanged()
}

// HandleAppDeactivate processes an app deactivation event.
func HandleAppDeactivate(appName, bundleID string) {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: handleAppDeactivate called")
	}

	if appWatcher != nil {
		if bridgeLogger != nil {
			bridgeLogger.Debug("Bridge: Forwarding app deactivate event",
				zap.String("app_name", appName),
				zap.String("bundle_id", bundleID))
		}

		appWatcher.HandleDeactivate(appName, bundleID)
	}
}

//export handleAppDeactivate
func handleAppDeactivate(cAppName *C.char, cBundleID *C.char) {
	HandleAppDeactivate(C.GoString(cAppName), C.GoString(cBundleID))
}

// GetActiveScreenBounds retrieves the screen bounds containing the current mouse cursor position.
func GetActiveScreenBounds() image.Rectangle {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: GetActiveScreenBounds called")
	}

	rect := C.getActiveScreenBounds()
	result := image.Rect(
		int(rect.origin.x),
		int(rect.origin.y),
		int(rect.origin.x+rect.size.width),
		int(rect.origin.y+rect.size.height),
	)

	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: Active screen bounds",
			zap.Int("x", result.Min.X),
			zap.Int("y", result.Min.Y),
			zap.Int("width", result.Dx()),
			zap.Int("height", result.Dy()))
	}

	return result
}

// ShowConfigValidationError displays a native macOS alert for config validation errors.
// Returns true if the user clicked the "Copy Config Path" button.
func ShowConfigValidationError(errorMessage, configPath string) bool {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: Showing config validation error alert",
			zap.String("error", errorMessage),
			zap.String("config_path", configPath))
	}

	cError := C.CString(errorMessage)
	defer C.free(unsafe.Pointer(cError)) //nolint:nlreturn

	cPath := C.CString(configPath)
	defer C.free(unsafe.Pointer(cPath)) //nolint:nlreturn

	result := C.showConfigValidationErrorAlert(cError, cPath)

	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: Alert result",
			zap.Int("result", int(result)))
	}

	// Return true if user clicked "Copy" button (result == 2)
	return result == 2
}
