//go:build darwin

package darwin

/*
#include <stdlib.h>
#include "axobserver.h"
*/
import "C"

import "unsafe"

// AXObserverNotificationHandler receives the pid whose UI changed and the name
// of the notification that fired (for debug logging).
type AXObserverNotificationHandler func(pid int, notif string)

var axObserverSlot cgoSlot[AXObserverNotificationHandler]

// SetAXObserverHandler registers the process-global observer callback. Passing
// nil clears it and drops any in-flight callback via the slot's generation.
func SetAXObserverHandler(handler AXObserverNotificationHandler) {
	axObserverSlot.Set(handler)
}

// StartObserverThread starts the observer run-loop thread if it is not already
// running, blocking until it is live. Idempotent.
func StartObserverThread() {
	C.NeruObserverStartThread()
}

// StopObserverThread stops and joins the observer run-loop thread. Idempotent.
func StopObserverThread() {
	C.NeruObserverStopThread()
}

// ObserverThreadRunning reports whether the observer run-loop thread is running.
func ObserverThreadRunning() bool {
	return C.NeruObserverThreadRunning() != 0
}

// ArmObserver arms an AXObserver on pid for the fixed accessibility
// notification set and returns an opaque handle, or nil on failure. The
// run-loop thread must be running. messagingTimeout (seconds, ignored when
// <= 0) bounds this observer's synchronous AX calls to the target app; it is
// scoped to that app's element, not process-wide.
func ArmObserver(pid int, messagingTimeout float64) unsafe.Pointer {
	return unsafe.Pointer(C.NeruObserverArm(C.int(pid), C.float(messagingTimeout)))
}

// DisarmObserver tears down a handle from ArmObserver. When live is true it
// unregisters notifications; skip that for a process that has exited. Safe with a
// nil handle.
func DisarmObserver(handle unsafe.Pointer, live bool) {
	liveFlag := C.int(0)
	if live {
		liveFlag = C.int(1)
	}

	C.NeruObserverDisarm(handle, liveFlag)
}

// ObserverLiveCounts returns the created-minus-released counts of AXObserver and
// application-element refs, for leak-balance assertions in tests. Both are zero
// at idle.
func ObserverLiveCounts() (observers, appElements int64) {
	return int64(C.NeruObserverLiveObserverCount()),
		int64(C.NeruObserverLiveAppElementCount())
}

//export handleAXObserverNotification
func handleAXObserverNotification(pid C.int, notif *C.char) {
	firingPID := int(pid)
	name := C.GoString(notif)

	axObserverSlot.withValid(func(handler AXObserverNotificationHandler) {
		handler(firingPID, name)
	})
}

// HandleAXObserverNotification synthesizes an observer notification, dispatching
// it through the registered handler. It lets the callback wiring be tested
// without a live accessibility notification.
func HandleAXObserverNotification(pid int, notif string) {
	cNotif := C.CString(notif)
	defer C.free(unsafe.Pointer(cNotif))

	handleAXObserverNotification(C.int(pid), cNotif)
}
