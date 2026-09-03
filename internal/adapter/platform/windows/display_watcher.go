//go:build windows

package windows

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Display-configuration change notifications via a hidden top-level window.
//
// Windows has no hook for display changes the way it has one for the
// foreground window: WM_DISPLAYCHANGE is broadcast to every top-level window
// when a resolution, arrangement or monitor set changes, and WM_DPICHANGED is
// sent to a window whose monitor's DPI changes. Both need an HWND to land on,
// so the watcher owns a hidden, never-shown popup window on a
// runtime.LockOSThread pump of its own (doc.go). The keyboard hook's loop only
// exists while a mode is active, and a display change with nothing open still
// has to move the overlay for the next activation.
//
// The window procedure hands the change to the caller's callback and returns.
// Everything the callback does is on the pump thread, so it must never block
// or take a lock something on this thread could be waiting for.
const (
	wmDisplayChange = 0x007E
	wmDPIChanged    = 0x02E0

	displayWatcherClassName = "NeruDisplayWatcher"
)

var (
	// activeDisplayWatcher is the watcher displayWndProc dispatches to, atomic
	// for the reason activeForegroundWatcher is: the pump thread stores it and
	// Windows calls the procedure on that same thread, but Stop reads it from
	// the caller's goroutine.
	activeDisplayWatcher atomic.Pointer[DisplayWatcher]

	// displayWndProcPtr is allocated once for the process, never per watcher: a
	// syscall.NewCallback slot is never freed and the process has a fixed 2000
	// of them (see keyboardHookProcPtr).
	displayWndProcPtr = sync.OnceValue(func() uintptr {
		return syscall.NewCallback(displayWndProc)
	})

	displayClassOnce sync.Once
	errDisplayClass  error
)

var (
	errDisplayWatcherCallbackNil = errors.New("display watcher callback is nil")
	errDisplayWindowCreateFailed = errors.New(
		"CreateWindowExW failed: display watcher window not created",
	)
)

// DisplayWatcher delivers display-configuration changes to a callback.
type DisplayWatcher struct {
	callback func()
	hwnd     uintptr
	threadID uint32
	stopOnce sync.Once
	doneCh   chan struct{}
	startErr chan error
}

// StartDisplayWatcher creates a hidden window on a dedicated message-loop
// thread and calls callback, on that thread, each time the window receives
// WM_DISPLAYCHANGE or WM_DPICHANGED. It returns once the window exists, or the
// creation error.
func StartDisplayWatcher(callback func()) (*DisplayWatcher, error) {
	if callback == nil {
		return nil, errDisplayWatcherCallbackNil
	}

	watcher := &DisplayWatcher{
		callback: callback,
		doneCh:   make(chan struct{}),
		startErr: make(chan error, 1),
	}

	go watcher.run()

	// Every exit from run either sends the install error or closes startErr,
	// so this receive is the whole handshake.
	err := <-watcher.startErr
	if err != nil {
		return nil, err
	}

	return watcher, nil
}

// WindowHandle is the hidden window the watcher listens on. It exists so a
// test can post the messages Windows would broadcast; nothing else needs it.
func (w *DisplayWatcher) WindowHandle() uintptr {
	return w.hwnd
}

// Stop destroys the window and waits for the pump thread to exit. No callback
// fires after it returns. The join is unbounded for the reason
// ForegroundWatcher.Stop's is: the callback contract forbids parking on a lock
// the caller holds.
func (w *DisplayWatcher) Stop() {
	if w == nil {
		return
	}

	w.stopOnce.Do(func() {
		// threadID is written before startErr closes, and Start does not
		// return the watcher before that, so it is always set here.
		_, _, _ = procPostThreadMessageW.Call(uintptr(w.threadID), wmQuit, 0, 0)
	})

	<-w.doneCh
}

// displayWndProc is the WNDPROC for the watcher's window class. Only the two
// display messages reach the callback; everything else is DefWindowProc's.
func displayWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	if message == wmDisplayChange || message == wmDPIChanged {
		if current := activeDisplayWatcher.Load(); current != nil && current.hwnd == hwnd {
			current.callback()
		}

		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)

	return ret
}

func registerDisplayWatcherClass() error {
	displayClassOnce.Do(func() {
		className, err := windows.UTF16PtrFromString(displayWatcherClassName)
		if err != nil {
			errDisplayClass = err

			return
		}

		class := wndClassEx{
			cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
			lpfnWndProc:   displayWndProcPtr(),
			hInstance:     windows.Handle(moduleHandle()),
			lpszClassName: className,
		}

		atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
		if atom == 0 {
			errDisplayClass = fmt.Errorf("RegisterClassExW: %w", err)
		}
	})

	return errDisplayClass
}

func (w *DisplayWatcher) run() {
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()
	defer close(w.doneCh)

	err := registerDisplayWatcherClass()
	if err != nil {
		w.startErr <- err

		return
	}

	className, err := windows.UTF16PtrFromString(displayWatcherClassName)
	if err != nil {
		w.startErr <- err

		return
	}

	// A popup with no WS_VISIBLE is a top-level window that is never shown,
	// which is exactly the set WM_DISPLAYCHANGE is broadcast to. A
	// message-only window (HWND_MESSAGE parent) would not qualify.
	hwnd, _, callErr := procCreateWindowExW.Call(
		wsExToolWindow|wsExNoActivate,
		uintptr(unsafe.Pointer(className)),
		0,
		wsPopup,
		0,
		0,
		0,
		0,
		0,
		0,
		moduleHandle(),
		0,
	)
	if hwnd == 0 {
		w.startErr <- fmt.Errorf("%w: %w", errDisplayWindowCreateFailed, callErr)

		return
	}

	// CreateWindowExW is a user32 call, so this thread has a message queue
	// from here on and Stop's PostThreadMessage cannot be lost.
	threadID, _, _ := procGetCurrentThreadID.Call()
	w.threadID = uint32(threadID)
	w.hwnd = hwnd
	activeDisplayWatcher.Store(w)

	close(w.startErr)

	defer func() {
		activeDisplayWatcher.CompareAndSwap(w, nil)

		discardCall(procDestroyWindow.Call(hwnd))
	}()

	var message msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if ret == 0 || int32(ret) == -1 {
			return
		}

		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}
