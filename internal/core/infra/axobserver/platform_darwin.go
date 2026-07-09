//go:build darwin

package axobserver

import (
	"unsafe"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

func platformStartObserverThread() {
	darwin.StartObserverThread()
}

func platformStopObserverThread() {
	darwin.StopObserverThread()
}

func platformObserverThreadRunning() bool {
	return darwin.ObserverThreadRunning()
}

func platformSetObserverMessagingTimeout(seconds float64) {
	darwin.SetObserverMessagingTimeout(seconds)
}

func platformArmObserver(pid int, epoch uint64, mask uint32) unsafe.Pointer {
	return darwin.ArmObserver(pid, epoch, mask)
}

func platformDisarmObserver(handle unsafe.Pointer, live bool) {
	darwin.DisarmObserver(handle, live)
}

// sinkFunc adapts a plain function to the darwin.ObserverSink interface.
type sinkFunc func(pid int, epoch uint64, notif string)

func (f sinkFunc) HandleAXNotification(pid int, epoch uint64, notif string) {
	f(pid, epoch, notif)
}

func platformSetObserverSink(f func(pid int, epoch uint64, notif string)) {
	if f == nil {
		darwin.SetObserverSink(nil)

		return
	}

	darwin.SetObserverSink(sinkFunc(f))
}
