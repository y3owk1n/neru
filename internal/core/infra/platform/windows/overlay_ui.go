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
		overlayUIOps = make(chan func(), 256)
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

func runOnOverlayUI(fn func()) {
	startOverlayUIThread()

	if overlayUIGID.Load() == curGoroutineID() {
		fn()
		pumpOverlayMessages()

		return
	}

	done := make(chan struct{})
	overlayUIOps <- func() {
		fn()
		close(done)
	}
	<-done
}

func curGoroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// "goroutine 123 ["
	i := 10
	for i < n && buf[i] >= '0' && buf[i] <= '9' {
		i++
	}

	var id uint64
	for j := 10; j < i; j++ {
		id = id*10 + uint64(buf[j]-'0')
	}

	return id
}

func pumpOverlayMessages() {
	var msg winMsg

	for {
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

		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
