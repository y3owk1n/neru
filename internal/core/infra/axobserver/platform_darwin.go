//go:build darwin

package axobserver

import (
	"syscall"
	"unsafe"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

// observerMessagingTimeoutSeconds bounds each observer's synchronous AX calls to
// the app it watches, so a wedged app cannot hang the observer thread. It is set
// on that app's element only, never process-wide, so hint scanning keeps the
// default timeout.
const observerMessagingTimeoutSeconds = 0.25

// darwinPlatform drives real AXObservers through the darwin bridge. It owns the
// run-loop thread lifecycle and the live observer handles: the thread starts on
// the first armed observer and stops once the last one is gone, so an idle neru
// has no observer thread and no background cost. The Manager serializes every
// call (it holds its lock), so the handle map and the threadUp flag need no lock
// of their own.
type darwinPlatform struct {
	handles  map[int]unsafe.Pointer
	threadUp bool
}

func newPlatform() Platform {
	return &darwinPlatform{handles: make(map[int]unsafe.Pointer)}
}

func (p *darwinPlatform) Arm(pid int, mask Mask) bool {
	native := toDarwinMask(mask)
	if native == 0 {
		return false
	}

	// Arm replaces: the Manager re-arms a mask change by calling Arm again with
	// no prior Disarm, so tear down any existing handle for this pid first.
	if old, ok := p.handles[pid]; ok {
		delete(p.handles, pid)
		darwin.DisarmObserver(old, processAlive(pid))
	}

	p.ensureThreadUp()

	handle := darwin.ArmObserver(pid, native, observerMessagingTimeoutSeconds)
	if handle == nil {
		p.stopThreadIfIdle()

		return false
	}

	p.handles[pid] = handle

	return true
}

func (p *darwinPlatform) Disarm(pid int) {
	handle, ok := p.handles[pid]
	if !ok {
		return
	}

	delete(p.handles, pid)
	darwin.DisarmObserver(handle, processAlive(pid))
	p.stopThreadIfIdle()
}

func (p *darwinPlatform) DisarmAll() {
	for pid, handle := range p.handles {
		darwin.DisarmObserver(handle, processAlive(pid))
	}

	p.handles = make(map[int]unsafe.Pointer)
	p.stopThreadIfIdle()
}

func (p *darwinPlatform) SetSink(sink func(pid int, notif string)) {
	if sink == nil {
		darwin.SetAXObserverHandler(nil)

		return
	}

	darwin.SetAXObserverHandler(func(pid int, notif string) {
		sink(pid, notif)
	})
}

func (p *darwinPlatform) Close() {
	darwin.SetAXObserverHandler(nil)
	p.DisarmAll()
}

func (p *darwinPlatform) ensureThreadUp() {
	if p.threadUp {
		return
	}

	darwin.StartObserverThread()
	p.threadUp = true
}

func (p *darwinPlatform) stopThreadIfIdle() {
	if !p.threadUp || len(p.handles) > 0 {
		return
	}

	darwin.StopObserverThread()
	p.threadUp = false
}

// processAlive reports whether pid still exists, so a disarm can skip the
// notification-unregister IPC to a process that has already exited. Signal 0
// performs the existence and permission check without delivering a signal.
func processAlive(pid int) bool {
	return syscall.Kill(pid, syscall.Signal(0)) != syscall.ESRCH
}

// toDarwinMask maps the package's notification bits to the native notification
// bits. The two bit layouts are kept independent on purpose, so a change to one
// does not silently reinterpret the other.
func toDarwinMask(mask Mask) darwin.AXObserverMask {
	pairs := []struct {
		bit    Mask
		native darwin.AXObserverMask
	}{
		{notifCreated, darwin.AXNotifCreated},
		{notifUIDestroyed, darwin.AXNotifUIDestroyed},
		{notifLayoutChanged, darwin.AXNotifLayoutChanged},
		{notifWindowCreated, darwin.AXNotifWindowCreated},
		{notifWindowMoved, darwin.AXNotifWindowMoved},
		{notifWindowResized, darwin.AXNotifWindowResized},
		{notifLoadComplete, darwin.AXNotifLoadComplete},
		{notifMenuOpened, darwin.AXNotifMenuOpened},
		{notifMenuClosed, darwin.AXNotifMenuClosed},
		{notifFocusedUIChanged, darwin.AXNotifFocusedUIChanged},
		{notifValueChanged, darwin.AXNotifValueChanged},
		{notifLiveRegionChanged, darwin.AXNotifLiveRegionChanged},
		{notifLiveRegionCreated, darwin.AXNotifLiveRegionCreated},
		{notifExpandedChanged, darwin.AXNotifExpandedChanged},
		{notifRowExpanded, darwin.AXNotifRowExpanded},
		{notifRowCollapsed, darwin.AXNotifRowCollapsed},
		{notifElementBusyChanged, darwin.AXNotifElementBusyChanged},
	}

	var native darwin.AXObserverMask
	for _, pair := range pairs {
		if mask&pair.bit != 0 {
			native |= pair.native
		}
	}

	return native
}
