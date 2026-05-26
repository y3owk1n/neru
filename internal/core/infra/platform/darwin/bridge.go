//go:build darwin

package darwin

/*
#include "hotkeys.h"
#include "theme.h"
#include "appwatcher.h"
#include "accessibility.h"
#include <stdlib.h>

extern void hotkeyCallbackBridge(int hotkeyId, int eventKind, void* userData);
*/
import "C"

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"go.uber.org/zap"
)

var (
	appWatcher         AppWatcherInterface
	appWatcherMu       sync.RWMutex
	mcDetectionEnabled atomic.Bool
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

	result := C.NeruRegisterHotkey(
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

	C.NeruUnregisterHotkey(C.int(hotkeyID))
}

// UnregisterAllHotkeys unregisters all global hotkeys on macOS.
func UnregisterAllHotkeys() {
	C.NeruUnregisterAllHotkeys()
}

// ParseKeyString parses a key string into a key code and modifiers on macOS.
func ParseKeyString(keyString string) (int, int, bool) {
	log := getLogger()
	log.Debug("Darwin: Parsing key string", zap.String("key_string", keyString))

	cKeyString := C.CString(keyString)
	defer C.free(unsafe.Pointer(cKeyString)) //nolint:nlreturn

	var keyCode C.int
	var modifiers C.int
	result := C.NeruParseKeyString(cKeyString, &keyCode, &modifiers)

	log.Debug("Darwin: Parse key string result",
		zap.Int("key_code", int(keyCode)),
		zap.Int("modifiers", int(modifiers)),
		zap.Bool("success", result == 1))

	success := result == 1

	return int(keyCode), int(modifiers), success
}

// HotkeyEventKind describes whether a global hotkey was pressed or released.
type HotkeyEventKind int

const (
	// HotkeyEventPressed is emitted when a registered hotkey is pressed.
	HotkeyEventPressed HotkeyEventKind = 1
	// HotkeyEventReleased is emitted when a registered hotkey is released.
	HotkeyEventReleased HotkeyEventKind = 2
)

// HotkeyHandler defines the signature for hotkey event handlers.
type HotkeyHandler func(hotkeyID int, eventKind HotkeyEventKind)

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
func hotkeyCallbackBridge(hotkeyID C.int, eventKind C.int, _ unsafe.Pointer) {
	hotkeyHandlerMu.RLock()
	handler := hotkeyHandler
	hotkeyHandlerMu.RUnlock()

	if handler != nil {
		handler(int(hotkeyID), HotkeyEventKind(eventKind))
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

// SetDetectMissionControlEnabled enables or disables Mission Control detection.
// When disabled, the entire detection pipeline is gated at the ObjC level —
// no timer, no window scans, and no callbacks to the Go bridge.
func SetDetectMissionControlEnabled(enabled bool) {
	mcDetectionEnabled.Store(enabled)
	C.NeruSetDetectMissionControlEnabled(C.bool(enabled))
}

// StartAppWatcher begins monitoring application lifecycle events.
func StartAppWatcher() {
	log := getLogger()
	log.Debug("Darwin: Starting app watcher")
	C.NeruStartAppWatcher()
}

// StopAppWatcher ceases monitoring application lifecycle events.
func StopAppWatcher() {
	log := getLogger()
	log.Debug("Darwin: Stopping app watcher")
	C.NeruStopAppWatcher()
}

func currentAppWatcher() AppWatcherInterface {
	appWatcherMu.RLock()
	watcher := appWatcher
	appWatcherMu.RUnlock()

	return watcher
}

func dispatchAppWatcherAppEvent(
	handlerName, forwardMsg string,
	appName, bundleID *C.char,
	forward func(AppWatcherInterface, string, string),
) {
	log := getLogger()
	log.Debug("Darwin: " + handlerName + " called")

	watcher := currentAppWatcher()
	if watcher == nil {
		return
	}

	name := C.GoString(appName)
	id := C.GoString(bundleID)
	log.Debug("Darwin: "+forwardMsg,
		zap.String("app_name", name),
		zap.String("bundle_id", id))
	forward(watcher, name, id)
}

func dispatchAppWatcherVoidEvent(
	handlerName, forwardMsg string,
	async bool,
	forward func(AppWatcherInterface),
) {
	log := getLogger()
	log.Debug("Darwin: " + handlerName + " called")

	watcher := currentAppWatcher()
	if watcher == nil {
		return
	}

	log.Debug("Darwin: " + forwardMsg)
	if async {
		go forward(watcher)

		return
	}

	forward(watcher)
}

//export handleAppLaunch
func handleAppLaunch(appName *C.char, bundleID *C.char) {
	dispatchAppWatcherAppEvent("handleAppLaunch", "Forwarding app launch event", appName, bundleID,
		func(w AppWatcherInterface, name, id string) { w.HandleLaunch(name, id) })
}

//export handleAppTerminate
func handleAppTerminate(appName *C.char, bundleID *C.char) {
	dispatchAppWatcherAppEvent(
		"handleAppTerminate",
		"Forwarding app termination event",
		appName,
		bundleID,
		func(w AppWatcherInterface, name, id string) { w.HandleTerminate(name, id) },
	)
}

//export handleAppActivate
func handleAppActivate(appName *C.char, bundleID *C.char) {
	dispatchAppWatcherAppEvent(
		"handleAppActivate",
		"Forwarding app activation event",
		appName,
		bundleID,
		func(w AppWatcherInterface, name, id string) { w.HandleActivate(name, id) },
	)
}

//export handleAppDeactivate
func handleAppDeactivate(appName *C.char, bundleID *C.char) {
	dispatchAppWatcherAppEvent(
		"handleAppDeactivate",
		"Forwarding app deactivation event",
		appName,
		bundleID,
		func(w AppWatcherInterface, name, id string) { w.HandleDeactivate(name, id) },
	)
}

//export handleScreenParametersChanged
func handleScreenParametersChanged() {
	dispatchAppWatcherVoidEvent(
		"handleScreenParametersChanged",
		"Forwarding screen parameters changed event",
		true,
		func(w AppWatcherInterface) { w.HandleScreenParametersChanged() },
	)
}

//export handleMissionControlActivated
func handleMissionControlActivated() {
	if !mcDetectionEnabled.Load() {
		return
	}

	dispatchAppWatcherVoidEvent(
		"handleMissionControlActivated",
		"Forwarding Mission Control activated event",
		true,
		func(w AppWatcherInterface) { w.HandleMissionControlActivated() },
	)
}

//export handleMissionControlDeactivated
func handleMissionControlDeactivated() {
	if !mcDetectionEnabled.Load() {
		return
	}

	dispatchAppWatcherVoidEvent(
		"handleMissionControlDeactivated",
		"Forwarding Mission Control deactivated event",
		true,
		func(w AppWatcherInterface) { w.HandleMissionControlDeactivated() },
	)
}

// HandleAppLaunch simulates an app launch event for testing.
func HandleAppLaunch(appName, bundleID string) {
	cName := C.CString(appName)
	cBundle := C.CString(bundleID)
	defer C.free(unsafe.Pointer(cName))   //nolint:nlreturn
	defer C.free(unsafe.Pointer(cBundle)) //nolint:nlreturn

	handleAppLaunch(cName, cBundle)
}

// HandleAppTerminate simulates an app terminate event for testing.
func HandleAppTerminate(appName, bundleID string) {
	cName := C.CString(appName)
	cBundle := C.CString(bundleID)
	defer C.free(unsafe.Pointer(cName))   //nolint:nlreturn
	defer C.free(unsafe.Pointer(cBundle)) //nolint:nlreturn

	handleAppTerminate(cName, cBundle)
}

// HandleAppActivate simulates an app activate event for testing.
func HandleAppActivate(appName, bundleID string) {
	cName := C.CString(appName)
	cBundle := C.CString(bundleID)
	defer C.free(unsafe.Pointer(cName))   //nolint:nlreturn
	defer C.free(unsafe.Pointer(cBundle)) //nolint:nlreturn

	handleAppActivate(cName, cBundle)
}

// HandleAppDeactivate simulates an app deactivate event for testing.
func HandleAppDeactivate(appName, bundleID string) {
	cName := C.CString(appName)
	cBundle := C.CString(bundleID)
	defer C.free(unsafe.Pointer(cName))   //nolint:nlreturn
	defer C.free(unsafe.Pointer(cBundle)) //nolint:nlreturn

	handleAppDeactivate(cName, cBundle)
}

// HandleScreenParametersChanged simulates a screen parameters changed event for testing.
func HandleScreenParametersChanged() {
	handleScreenParametersChanged()
}

// HandleMissionControlActivated simulates a Mission Control activation event for testing.
func HandleMissionControlActivated() {
	handleMissionControlActivated()
}

// HandleMissionControlDeactivated simulates a Mission Control deactivation event for testing.
func HandleMissionControlDeactivated() {
	handleMissionControlDeactivated()
}
