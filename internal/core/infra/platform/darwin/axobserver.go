//go:build darwin

package darwin

/*
#include "axobserver.h"
*/
import "C"

// AXObserverNotificationHandler receives the name of the accessibility
// notification that fired (for debug logging).
type AXObserverNotificationHandler func(notif string)

var axObserverHandlerSlot cgoSlot[AXObserverNotificationHandler]

// SetAXObserverHandler registers the process-global observer callback. Passing
// nil clears it and drops any in-flight callback via the slot's generation.
func SetAXObserverHandler(handler AXObserverNotificationHandler) {
	axObserverHandlerSlot.Set(handler)
}

// ObserverThreadRunning reports whether the observer run-loop thread is running.
func ObserverThreadRunning() bool {
	return C.NeruObserverThreadRunning() != 0
}

// WatchObserver makes pid the watched process: it arms an AXObserver on pid for
// the fixed accessibility notification set, tearing down whatever was watched
// before, and reports whether the watch succeeded. On failure nothing is
// watched afterward, so a later call retries from a clean state.
// messagingTimeout (seconds, ignored when <= 0) bounds this observer's
// synchronous AX calls to the target app; it is scoped to that app's element,
// not process-wide.
func WatchObserver(pid int, messagingTimeout float64) bool {
	return C.NeruObserverWatch(C.int(pid), C.float(messagingTimeout)) != 0
}

// UnwatchObserver stops watching: it tears down the watched observer, if any,
// and stops the run-loop thread. Safe to call when nothing is watched.
func UnwatchObserver() {
	C.NeruObserverUnwatch()
}

// ObserverLiveCounts returns the created-minus-released counts of AXObserver and
// application-element refs, for leak-balance assertions in tests. Both are zero
// at idle.
func ObserverLiveCounts() (observers, appElements int64) {
	return int64(C.NeruObserverLiveObserverCount()),
		int64(C.NeruObserverLiveAppElementCount())
}

//export handleAXObserverNotification
func handleAXObserverNotification(notif *C.char) {
	dispatchAXObserverNotification(C.GoString(notif))
}

// dispatchAXObserverNotification routes a notification name through the
// registered handler.
func dispatchAXObserverNotification(notif string) {
	axObserverHandlerSlot.withValid(func(handler AXObserverNotificationHandler) {
		handler(notif)
	})
}
