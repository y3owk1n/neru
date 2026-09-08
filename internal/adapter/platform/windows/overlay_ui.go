//go:build windows

package windows

import (
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

// Dedicated Win32 UI thread with a message pump for HWND creation and painting.
// Does not implement overlay drawing; overlay.go marshals HWND work here.
var (
	overlayUIOnce                   sync.Once
	overlayUIOps                    chan func()
	overlayUIGID                    atomic.Uint64
	overlayUIThreadID               uintptr
	procMsgWaitForMultipleObjectsEx = user32.NewProc("MsgWaitForMultipleObjectsEx")
)

const (
	overlayUIOpsBuffer = 256
	goroutinePrefixLen = len("goroutine ")
	decimalBase        = 10
	qsAllInput         = 0x04FF
	mwmoInputAvailable = 0x0004
)

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct {
		x int32
		y int32
	}
}

func startOverlayUIThread() {
	overlayUIOnce.Do(func() {
		overlayUIOps = make(chan func(), overlayUIOpsBuffer)
		ready := make(chan struct{})

		go func() {
			runtime.LockOSThread()
			overlayUIGID.Store(curGoroutineID())

			overlayUIThreadID, _, _ = procGetCurrentThreadID.Call()
			// Create the native queue before producers can post its wakeup.
			pumpOverlayMessages()
			close(ready)

			for {
				pumpOverlayMessages()

				select {
				case callback := <-overlayUIOps:
					callback()
				default:
					// A Go channel wait cannot service HWND messages. Wake for
					// native input as well as the WM_NULL posted with callbacks.
					// INPUTAVAILABLE also wakes for a bounded pump's leftovers.
					discardCall(procMsgWaitForMultipleObjectsEx.Call(
						0, 0, uintptr(^uint32(0)), qsAllInput, mwmoInputAvailable,
					))
				}
			}
		}()

		<-ready
	})
}

func runOnOverlayUI(callback func()) {
	startOverlayUIThread()

	if overlayUIGID.Load() == curGoroutineID() {
		callback()
		pumpOverlayMessages()

		return
	}

	done := make(chan struct{})
	queueOverlayUI(func() {
		callback()
		close(done)
	})

	<-done
}

// postOnOverlayUI queues callback on the overlay UI thread and returns without
// waiting for it. On the UI thread itself it runs inline, as runOnOverlayUI
// does. It is how a Flush leaves the keyboard hook's thread before a pixel is
// painted.
func postOnOverlayUI(callback func()) {
	startOverlayUIThread()

	if overlayUIGID.Load() == curGoroutineID() {
		callback()

		return
	}

	queueOverlayUI(callback)
}

func queueOverlayUI(callback func()) {
	overlayUIOps <- callback
	// Publish the callback first so a wakeup cannot run ahead of its work.
	// WM_NULL needs no window and is harmless if another pump consumes it.
	discardCall(procPostThreadMessageW.Call(overlayUIThreadID, 0, 0, 0))
}

func curGoroutineID() uint64 {
	var buf [64]byte

	n := runtime.Stack(buf[:], false)
	idx := goroutinePrefixLen

	for idx < n && buf[idx] >= '0' && buf[idx] <= '9' {
		idx++
	}

	var id uint64
	for j := goroutinePrefixLen; j < idx; j++ {
		id = id*decimalBase + uint64(buf[j]-'0')
	}

	return id
}

// maxMessagesPerPump bounds a single drain so a pathological message source
// (e.g. WM_PAINT regenerating because an update region never validated) can
// never spin this loop forever and wedge the overlay UI thread. Any remaining
// messages are drained on the next pump, so a real backlog is not lost.
const maxMessagesPerPump = 512

func pumpOverlayMessages() {
	var msg winMsg

	for range maxMessagesPerPump {
		ret, _, _ := procPeekMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0,
			0,
			0,
			pmRemove,
		)
		if ret == 0 {
			return
		}

		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
