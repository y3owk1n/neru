//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc

#include "hotkeys.h"
#include "theme.h"
#include "appwatcher.h"
#include <stdlib.h>

extern void hotkeyCallbackBridge(int hotkeyId, void* userData);

void startAppWatcher();
void stopAppWatcher();
*/
import "C"

import (
	"sync"
	"unsafe"

	"go.uber.org/zap"
)

var (
	appWatcher   AppWatcherInterface
	appWatcherMu sync.RWMutex
)

// AppWatcher returns the global application watcher instance.
func AppWatcher() AppWatcherInterface {
	appWatcherMu.RLock()
	defer appWatcherMu.RUnlock()

	return appWatcher
}

// RegisterHotkey registers a global hotkey on macOS.
func RegisterHotkey(
	keyCode, modifiers, hotkeyID int,
	callback unsafe.Pointer,
	userData unsafe.Pointer,
) bool {
	log := getLogger()
	log.Debug("Darwin: Registering hotkey",
		zap.Int("key_code", keyCode),
		zap.Int("modifiers", modifiers),
		zap.Int("hotkey_id", hotkeyID))

	result := C.registerHotkey(
		C.int(keyCode),
		C.int(modifiers),
		C.int(hotkeyID),
		(C.HotkeyCallback)(callback),
		userData, //nolint:nlreturn
	)

	log.Debug("Darwin: Register hotkey result",
		zap.Bool("success", result == 1))

	success := result == 1

	return success
}

// UnregisterHotkey unregisters a global hotkey on macOS.
func UnregisterHotkey(hotkeyID int) {
	log := getLogger()
	log.Debug("Darwin: Unregistering hotkey", zap.Int("hotkey_id", hotkeyID))

	C.unregisterHotkey(C.int(hotkeyID))
}

// UnregisterAllHotkeys unregisters all global hotkeys on macOS.
func UnregisterAllHotkeys() {
	C.unregisterAllHotkeys()
}

// ParseKeyString parses a key string into a key code and modifiers on macOS.
func ParseKeyString(keyString string) (int, int, bool) {
	log := getLogger()
	log.Debug("Darwin: Parsing key string", zap.String("key_string", keyString))

	cKeyString := C.CString(keyString)
	defer C.free(unsafe.Pointer(cKeyString)) //nolint:nlreturn

	var keyCode C.int
	var modifiers C.int
	result := C.parseKeyString(cKeyString, &keyCode, &modifiers)

	log.Debug("Darwin: Parse key string result",
		zap.Int("key_code", int(keyCode)),
		zap.Int("modifiers", int(modifiers)),
		zap.Bool("success", result == 1))

	success := result == 1

	return int(keyCode), int(modifiers), success
}

// HotkeyHandler defines the signature for hotkey event handlers.
type HotkeyHandler func(hotkeyID int)

var (
	hotkeyHandler   HotkeyHandler
	hotkeyHandlerMu sync.RWMutex
)

// SetHotkeyHandler sets the global hotkey event handler.
func SetHotkeyHandler(handler HotkeyHandler) {
	hotkeyHandlerMu.Lock()
	defer hotkeyHandlerMu.Unlock()
	hotkeyHandler = handler
}

// GetHotkeyCallbackBridge returns a pointer to the C hotkey callback bridge function.
func GetHotkeyCallbackBridge() unsafe.Pointer {
	ptr := (unsafe.Pointer)(C.hotkeyCallbackBridge) //nolint:unconvert

	return ptr
}

//export hotkeyCallbackBridge
func hotkeyCallbackBridge(hotkeyID C.int, _ unsafe.Pointer) {
	hotkeyHandlerMu.RLock()
	handler := hotkeyHandler
	hotkeyHandlerMu.RUnlock()

	if handler != nil {
		handler(int(hotkeyID))
	}
}

// SetAppWatcher configures the application watcher implementation.
func SetAppWatcher(watcher AppWatcherInterface) {
	log := getLogger()
	log.Debug("Darwin: Setting app watcher")

	appWatcherMu.Lock()
	defer appWatcherMu.Unlock()

	appWatcher = watcher
}

// StartAppWatcher begins monitoring application lifecycle events.
func StartAppWatcher() {
	log := getLogger()
	log.Debug("Darwin: Starting app watcher")
	C.startAppWatcher()
}

// StopAppWatcher ceases monitoring application lifecycle events.
func StopAppWatcher() {
	log := getLogger()
	log.Debug("Darwin: Stopping app watcher")
	C.stopAppWatcher()
}

//export handleAppLaunch
func handleAppLaunch(appName *C.char, bundleID *C.char) {
	log := getLogger()
	log.Debug("Darwin: handleAppLaunch called")

	appWatcherMu.RLock()
	watcher := appWatcher
	appWatcherMu.RUnlock()

	if watcher != nil {
		log.Debug("Darwin: Forwarding app launch event",
			zap.String("app_name", C.GoString(appName)),
			zap.String("bundle_id", C.GoString(bundleID)))

		watcher.HandleLaunch(C.GoString(appName), C.GoString(bundleID))
	}
}

//export handleAppTerminate
func handleAppTerminate(appName *C.char, bundleID *C.char) {
	log := getLogger()
	log.Debug("Darwin: handleAppTerminate called")

	appWatcherMu.RLock()
	watcher := appWatcher
	appWatcherMu.RUnlock()

	if watcher != nil {
		log.Debug("Darwin: Forwarding app termination event",
			zap.String("app_name", C.GoString(appName)),
			zap.String("bundle_id", C.GoString(bundleID)))

		watcher.HandleTerminate(C.GoString(appName), C.GoString(bundleID))
	}
}

//export handleAppActivate
func handleAppActivate(appName *C.char, bundleID *C.char) {
	log := getLogger()
	log.Debug("Darwin: handleAppActivate called")

	appWatcherMu.RLock()
	watcher := appWatcher
	appWatcherMu.RUnlock()

	if watcher != nil {
		log.Debug("Darwin: Forwarding app activation event",
			zap.String("app_name", C.GoString(appName)),
			zap.String("bundle_id", C.GoString(bundleID)))

		watcher.HandleActivate(C.GoString(appName), C.GoString(bundleID))
	}
}

//export handleAppDeactivate
func handleAppDeactivate(appName *C.char, bundleID *C.char) {
	log := getLogger()
	log.Debug("Darwin: handleAppDeactivate called")

	appWatcherMu.RLock()
	watcher := appWatcher
	appWatcherMu.RUnlock()

	if watcher != nil {
		log.Debug("Darwin: Forwarding app deactivation event",
			zap.String("app_name", C.GoString(appName)),
			zap.String("bundle_id", C.GoString(bundleID)))

		watcher.HandleDeactivate(C.GoString(appName), C.GoString(bundleID))
	}
}

//export handleScreenParametersChanged
func handleScreenParametersChanged() {
	log := getLogger()
	log.Debug("Darwin: handleScreenParametersChanged called")

	appWatcherMu.RLock()
	watcher := appWatcher
	appWatcherMu.RUnlock()

	if watcher != nil {
		log.Debug("Darwin: Forwarding screen parameters changed event")
		go watcher.HandleScreenParametersChanged()
	}
}

// HandleAppLaunch simulates an app launch event for testing.
func HandleAppLaunch(appName, bundleID string) {
	handleAppLaunch(C.CString(appName), C.CString(bundleID))
}

// HandleAppTerminate simulates an app terminate event for testing.
func HandleAppTerminate(appName, bundleID string) {
	handleAppTerminate(C.CString(appName), C.CString(bundleID))
}

// HandleAppActivate simulates an app activate event for testing.
func HandleAppActivate(appName, bundleID string) {
	handleAppActivate(C.CString(appName), C.CString(bundleID))
}

// HandleAppDeactivate simulates an app deactivate event for testing.
func HandleAppDeactivate(appName, bundleID string) {
	handleAppDeactivate(C.CString(appName), C.CString(bundleID))
}

// HandleScreenParametersChanged simulates a screen parameters changed event for testing.
func HandleScreenParametersChanged() {
	handleScreenParametersChanged()
}
