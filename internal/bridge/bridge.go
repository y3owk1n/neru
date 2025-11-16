// Package bridge provides C bridging functionality for the Neru application.
package bridge

// Package bridge provides Go bindings for Objective-C APIs used in the Neru application,
// including accessibility, hotkeys, and overlay functionality for macOS integration.

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ApplicationServices -framework Cocoa -framework Carbon -framework CoreGraphics
#include "accessibility.h"
#include "overlay.h"
#include "hotkeys.h"
#include "eventtap.h"
#include "appwatcher.h"
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

// Global logger instance for the bridge package.
var bridgeLogger *zap.Logger

// InitializeLogger initializes the logger for the bridge package.
func InitializeLogger(logger *zap.Logger) {
	bridgeLogger = logger
}

// This file ensures the bridge package is properly initialized
// and the Objective-C files are compiled with CGo
//
// The .m files are compiled separately via CGo's automatic source file detection

// SetApplicationAttribute toggles an accessibility attribute on the application with the provided PID.
func SetApplicationAttribute(pid int, attribute string, value bool) bool {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: Setting application attribute",
			zap.Int("pid", pid),
			zap.String("attribute", attribute),
			zap.Bool("value", value))
	}

	cAttr := C.CString(attribute)
	defer C.free(unsafe.Pointer(cAttr))

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

// HasClickAction checks if an element has the AXPress action available.
func HasClickAction(element unsafe.Pointer) bool {
	if element == nil {
		if bridgeLogger != nil {
			bridgeLogger.Debug("Bridge: HasClickAction called with nil element")
		}
		return false
	}

	result := C.hasClickAction(element)

	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: HasClickAction result",
			zap.Bool("has_action", result == 1))
	}

	return result == 1
}

var (
	appWatcher     AppWatcher
	appWatcherOnce sync.Once
)

// AppWatcher interface defines the methods for application watching.
type AppWatcher interface {
	HandleLaunch(appName, bundleID string)
	HandleTerminate(appName, bundleID string)
	HandleActivate(appName, bundleID string)
	HandleDeactivate(appName, bundleID string)
	HandleScreenParametersChanged()
}

// SetAppWatcher sets the application watcher implementation.
func SetAppWatcher(w AppWatcher) {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: Setting app watcher")
	}
	appWatcher = w
}

// StartAppWatcher starts watching for application events.
func StartAppWatcher() {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: Starting app watcher")
	}
	C.startAppWatcher()
}

// StopAppWatcher stops watching for application events.
func StopAppWatcher() {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: Stopping app watcher")
	}
	C.stopAppWatcher()
}

//export handleAppLaunch
func handleAppLaunch(cAppName *C.char, cBundleID *C.char) {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: handleAppLaunch called")
	}

	if appWatcher != nil {
		appName := C.GoString(cAppName)
		bundleID := C.GoString(cBundleID)

		if bridgeLogger != nil {
			bridgeLogger.Debug("Bridge: Forwarding app launch event",
				zap.String("app_name", appName),
				zap.String("bundle_id", bundleID))
		}

		appWatcher.HandleLaunch(appName, bundleID)
	}
}

//export handleAppTerminate
func handleAppTerminate(cAppName *C.char, cBundleID *C.char) {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: handleAppTerminate called")
	}

	if appWatcher != nil {
		appName := C.GoString(cAppName)
		bundleID := C.GoString(cBundleID)

		if bridgeLogger != nil {
			bridgeLogger.Debug("Bridge: Forwarding app terminate event",
				zap.String("app_name", appName),
				zap.String("bundle_id", bundleID))
		}

		appWatcher.HandleTerminate(appName, bundleID)
	}
}

//export handleAppActivate
func handleAppActivate(cAppName *C.char, cBundleID *C.char) {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: handleAppActivate called")
	}

	if appWatcher != nil {
		appName := C.GoString(cAppName)
		bundleID := C.GoString(cBundleID)

		if bridgeLogger != nil {
			bridgeLogger.Debug("Bridge: Forwarding app activate event",
				zap.String("app_name", appName),
				zap.String("bundle_id", bundleID))
		}

		appWatcher.HandleActivate(appName, bundleID)
	}
}

//export handleScreenParametersChanged
func handleScreenParametersChanged() {
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

//export handleAppDeactivate
func handleAppDeactivate(cAppName *C.char, cBundleID *C.char) {
	if bridgeLogger != nil {
		bridgeLogger.Debug("Bridge: handleAppDeactivate called")
	}

	if appWatcher != nil {
		appName := C.GoString(cAppName)
		bundleID := C.GoString(cBundleID)

		if bridgeLogger != nil {
			bridgeLogger.Debug("Bridge: Forwarding app deactivate event",
				zap.String("app_name", appName),
				zap.String("bundle_id", bundleID))
		}

		appWatcher.HandleDeactivate(appName, bundleID)
	}
}

// GetActiveScreenBounds returns the bounds of the screen containing the mouse cursor.
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
