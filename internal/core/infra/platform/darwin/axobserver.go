//go:build darwin

package darwin

/*
#include "axobserver.h"
*/
import "C"

import "unsafe"

// AX notification mask bits, mirrored from axobserver.h. Callers OR these
// together to select which notifications an observer registers on an
// application element.
const (
	AXNotifLayoutChanged           uint32 = C.NeruAXNotifLayoutChanged
	AXNotifCreated                 uint32 = C.NeruAXNotifCreated
	AXNotifUIElementDestroyed      uint32 = C.NeruAXNotifUIElementDestroyed
	AXNotifWindowCreated           uint32 = C.NeruAXNotifWindowCreated
	AXNotifWindowMoved             uint32 = C.NeruAXNotifWindowMoved
	AXNotifWindowResized           uint32 = C.NeruAXNotifWindowResized
	AXNotifFocusedUIElementChanged uint32 = C.NeruAXNotifFocusedUIElementChanged
	AXNotifMenuOpened              uint32 = C.NeruAXNotifMenuOpened
	AXNotifMenuClosed              uint32 = C.NeruAXNotifMenuClosed
	AXNotifValueChanged            uint32 = C.NeruAXNotifValueChanged
	AXNotifLoadComplete            uint32 = C.NeruAXNotifLoadComplete
)

// ObserverSink receives AX notification signals from the observer run-loop
// thread. Implementations must do O(1), non-blocking work (a channel send);
// the callback fires on the run-loop thread. notif is the notification name,
// for debug logging.
type ObserverSink interface {
	HandleAXNotification(pid int, epoch uint64, notif string)
}

// observerSlot holds the process-global sink for C-exported AX callbacks, using
// the same generation-checked slot as the other darwin bridges so a cleared or
// replaced sink ignores in-flight callbacks.
var observerSlot cgoSlot[ObserverSink]

// SetObserverSink registers the process-global AX notification sink. Passing nil
// clears it.
func SetObserverSink(sink ObserverSink) {
	observerSlot.Set(sink)
}

//export handleAXNotification
func handleAXNotification(pid C.int, epoch C.ulonglong, notif *C.char) {
	p := int(pid)
	e := uint64(epoch)
	name := C.GoString(notif)

	observerSlot.withValid(func(sink ObserverSink) {
		sink.HandleAXNotification(p, e, name)
	})
}

// StartObserverThread starts the dedicated observer run-loop thread if it is not
// already running, blocking until it is live. Idempotent.
func StartObserverThread() {
	C.NeruStartObserverThread()
}

// StopObserverThread stops and joins the observer run-loop thread. Idempotent.
func StopObserverThread() {
	C.NeruStopObserverThread()
}

// ObserverThreadRunning reports whether the observer run-loop thread is running.
func ObserverThreadRunning() bool {
	return C.NeruObserverThreadRunning() != 0
}

// SetObserverMessagingTimeout sets the accessibility messaging timeout on the
// system-wide element (best effort; bounds synchronous AX calls process-wide).
func SetObserverMessagingTimeout(seconds float64) {
	C.NeruSetObserverMessagingTimeout(C.float(seconds))
}

// ArmObserver creates an AXObserver for pid registering the notifications
// selected by mask, and returns an opaque handle (nil on failure). epoch is
// packed into the callback refcon so stale callbacks can be rejected.
func ArmObserver(pid int, epoch uint64, mask uint32) unsafe.Pointer {
	return unsafe.Pointer(
		C.NeruArmObserver(C.int(pid), C.ulonglong(epoch), C.uint32_t(mask)),
	)
}

// DisarmObserver tears down an observer handle from ArmObserver. When live is
// true it unregisters notifications (skip for a dead process). Safe with a nil
// handle.
func DisarmObserver(handle unsafe.Pointer, live bool) {
	liveFlag := C.int(0)
	if live {
		liveFlag = C.int(1)
	}

	C.NeruDisarmObserver(handle, liveFlag)
}

// ObserverLiveCounts returns the created-minus-released counts of AXObserver and
// application-element refs, for leak-balance assertions in tests.
func ObserverLiveCounts() (observers, appElements int64) {
	return int64(C.NeruObserverLiveObserverCount()),
		int64(C.NeruObserverLiveAppElementCount())
}
