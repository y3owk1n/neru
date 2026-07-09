package axobserver

import "unsafe"

// platform abstracts the native observer operations so the Manager can be unit
// tested with a fake on any OS. The real implementation forwards to the
// build-tagged platform* shims (the darwin AXObserver bridge, or no-ops).
type platform interface {
	startThread()
	stopThread()
	threadRunning() bool
	setMessagingTimeout(seconds float64)
	arm(pid int, epoch uint64, mask uint32) unsafe.Pointer
	disarm(handle unsafe.Pointer, live bool)
	setSink(sink func(pid int, epoch uint64, notif string))
}

// realPlatform forwards to the platform* package functions.
type realPlatform struct{}

func (realPlatform) startThread() { platformStartObserverThread() }

func (realPlatform) stopThread() { platformStopObserverThread() }

func (realPlatform) threadRunning() bool { return platformObserverThreadRunning() }

func (realPlatform) setMessagingTimeout(seconds float64) {
	platformSetObserverMessagingTimeout(seconds)
}

func (realPlatform) arm(pid int, epoch uint64, mask uint32) unsafe.Pointer {
	return platformArmObserver(pid, epoch, mask)
}

func (realPlatform) disarm(handle unsafe.Pointer, live bool) {
	platformDisarmObserver(handle, live)
}

func (realPlatform) setSink(sink func(pid int, epoch uint64, notif string)) {
	platformSetObserverSink(sink)
}
