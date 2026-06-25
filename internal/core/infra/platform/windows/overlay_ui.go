//go:build windows

// internal/core/infra/platform/windows/overlay_ui.go
// Dedicated Win32 UI thread with a message pump for HWND creation and painting.
// Does not implement overlay drawing; overlay.go marshals HWND work here.

package windows

import (
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

var (
	overlayUIOnce sync.Once
	overlayUIOps  chan func()
	overlayUIGID  atomic.Uint64
)

const (
	overlayUIOpsBuffer = 256
	goroutinePrefixLen = len("goroutine ")
	decimalBase        = 10
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
			close(ready)

			for fn := range overlayUIOps {
				fn()
				pumpOverlayMessages()
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
	overlayUIOps <- func() {
		callback()
		close(done)
	}

	<-done
}

func curGoroutineID() uint64 {
	var buf [64]byte

	n := runtime.Stack(buf[:], false)
	// "goroutine 123 ["
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
