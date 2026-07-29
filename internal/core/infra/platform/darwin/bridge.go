//go:build darwin

package darwin

/*
#include "eventtap.h"
#include "theme.h"
#include "appwatcher.h"
#include "accessibility.h"
#include <stdlib.h>

extern void hotkeyCallbackBridge(int hotkeyId, int eventKind, void* userData);
*/
import "C"

import (
	"sync/atomic"
	"unsafe"

	"go.uber.org/zap"
)

var (
	appWatcherSlot     cgoSlot[AppWatcherInterface]
	mcDetectionEnabled atomic.Bool
)

// AppWatcher returns the global application watcher instance.
func AppWatcher() AppWatcherInterface {
	return appWatcherSlot.Get()
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

var hotkeyHandlerSlot cgoSlot[HotkeyHandler]

// SetHotkeyHandler sets the global hotkey event handler.
func SetHotkeyHandler(handler HotkeyHandler) {
	hotkeyHandlerSlot.Set(handler)
}

// CreateHotkeyTap creates a per-hotkey CGEventTap.
// Returns an opaque handle (must be destroyed via DestroyHotkeyTap) or nil on failure.
func CreateHotkeyTap(hotkeyID, keyCode, modifiers int) unsafe.Pointer {
	log := getLogger()
	log.Debug("Darwin: Creating hotkey tap",
		zap.Int("hotkey_id", hotkeyID),
		zap.Int("key_code", keyCode),
		zap.Int("modifiers", modifiers))

	tap := C.NeruCreateHotkeyTap(
		C.int(hotkeyID),
		C.int(keyCode),
		C.int(modifiers),
		C.HotkeyTapCallback(C.hotkeyCallbackBridge),
		nil,
	)

	if tap == nil {
		log.Warn("Darwin: Failed to create hotkey tap — check Accessibility permissions")

		return nil
	}

	log.Debug("Darwin: Hotkey tap created", zap.Int("hotkey_id", hotkeyID))

	return unsafe.Pointer(tap)
}

// DestroyHotkeyTap destroys a per-hotkey CGEventTap previously created by CreateHotkeyTap.
func DestroyHotkeyTap(tap unsafe.Pointer) {
	if tap == nil {
		return
	}
	C.NeruDestroyHotkeyTap(C.HotkeyTapRef(tap))
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

//export hotkeyCallbackBridge
func hotkeyCallbackBridge(hotkeyID C.int, eventKind C.int, _ unsafe.Pointer) {
	id := int(hotkeyID)
	kind := HotkeyEventKind(eventKind)
	hotkeyHandlerSlot.withValid(func(handler HotkeyHandler) {
		handler(id, kind)
	})
}

// SetAppWatcher configures the application watcher implementation.
func SetAppWatcher(watcher AppWatcherInterface) {
	log := getLogger()
	log.Debug("Darwin: Setting app watcher")

	appWatcherSlot.Set(watcher)
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

func dispatchAppWatcherAppEvent(
	handlerName, forwardMsg string,
	appName, bundleID *C.char,
	forward func(AppWatcherInterface, string, string),
) {
	log := getLogger()
	log.Debug("Darwin: " + handlerName + " called")

	appWatcherSlot.withValid(func(watcher AppWatcherInterface) {
		name := C.GoString(appName)
		bundleIDValue := C.GoString(bundleID)
		log.Debug("Darwin: "+forwardMsg,
			zap.String("app_name", name),
			zap.String("bundle_id", bundleIDValue))
		forward(watcher, name, bundleIDValue)
	})
}

func dispatchAppWatcherVoidEvent(
	handlerName, forwardMsg string,
	async bool,
	forward func(AppWatcherInterface),
) {
	log := getLogger()
	log.Debug("Darwin: " + handlerName + " called")

	dispatch := func(watcher AppWatcherInterface) {
		log.Debug("Darwin: " + forwardMsg)
		forward(watcher)
	}

	if async {
		appWatcherSlot.withValidAsync(dispatch)

		return
	}

	appWatcherSlot.withValid(dispatch)
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

//export handleFrontAppSwitched
func handleFrontAppSwitched(appName *C.char, bundleID *C.char) {
	dispatchAppWatcherAppEvent(
		"handleFrontAppSwitched",
		"Forwarding front app switch event",
		appName,
		bundleID,
		func(w AppWatcherInterface, name, id string) { w.HandleFrontAppSwitch(name, id) },
	)
}

//export handleMenuTrackingChanged
func handleMenuTrackingChanged() {
	dispatchAppWatcherVoidEvent(
		"handleMenuTrackingChanged",
		"Forwarding menu tracking change event",
		true,
		func(w AppWatcherInterface) { w.HandleMenuTrackingChanged() },
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
