//go:build windows

package windows

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Foreground-window change notifications via SetWinEventHook.
//
// EVENT_SYSTEM_FOREGROUND fires when the foreground window changes, which is
// the Win32 equivalent of macOS's NSWorkspace activation notification. An
// out-of-context hook delivers it on the message loop of the thread that
// installed it, so the watcher owns a runtime.LockOSThread pump of its own
// (doc.go) rather than borrowing the keyboard hook's, which only exists while
// a mode is active.
//
// The hook procedure hands the HWND to the caller's callback and returns.
// Everything the callback does is on the pump thread, so it must never block
// or take a lock something on this thread could be waiting for.
const (
	eventSystemForeground = 0x0003
	wineventOutOfContext  = 0x0000
)

var (
	procSetWinEventHook = user32.NewProc("SetWinEventHook")
	procUnhookWinEvent  = user32.NewProc("UnhookWinEvent")

	// activeForegroundWatcher is the watcher foregroundEventProc dispatches to,
	// atomic for the reason activeKeyboardHook is: the pump thread stores it
	// and Windows calls the procedure on that same thread, but Stop reads it
	// from the caller's goroutine.
	activeForegroundWatcher atomic.Pointer[ForegroundWatcher]

	// foregroundEventProcPtr is allocated once for the process, never per
	// watcher: a syscall.NewCallback slot is never freed and the process has a
	// fixed 2000 of them (see keyboardHookProcPtr).
	foregroundEventProcPtr = sync.OnceValue(func() uintptr {
		return syscall.NewCallback(foregroundEventProc)
	})
)

var (
	errForegroundWatcherCallbackNil = errors.New("foreground watcher callback is nil")
	errForegroundHookInstallFailed  = errors.New(
		"SetWinEventHook failed: foreground watcher not installed",
	)
)

// ForegroundWatcher delivers foreground-window changes to a callback.
type ForegroundWatcher struct {
	callback func(hwnd uintptr)
	threadID uint32
	stopOnce sync.Once
	doneCh   chan struct{}
	startErr chan error
}

// StartForegroundWatcher installs an EVENT_SYSTEM_FOREGROUND hook on a
// dedicated message-loop thread and calls callback with each new foreground
// HWND, on that thread. It returns once the hook is installed, or the install
// error.
func StartForegroundWatcher(callback func(hwnd uintptr)) (*ForegroundWatcher, error) {
	if callback == nil {
		return nil, errForegroundWatcherCallbackNil
	}

	watcher := &ForegroundWatcher{
		callback: callback,
		doneCh:   make(chan struct{}),
		startErr: make(chan error, 1),
	}

	go watcher.run()

	select {
	case err := <-watcher.startErr:
		if err != nil {
			return nil, err
		}
	case <-watcher.doneCh:
		return nil, errForegroundHookInstallFailed
	}

	return watcher, nil
}

// Stop unhooks and waits for the pump thread to exit. No callback fires after
// it returns. Unlike KeyboardHook.Stop the join is unbounded: the callback
// contract above forbids the one thing that made a bounded join necessary
// there, a callback parked on a lock the caller holds.
func (w *ForegroundWatcher) Stop() {
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

// foregroundEventProc is the WINEVENTPROC Windows calls on the pump thread.
// The hook range is EVENT_SYSTEM_FOREGROUND only, so event is always that; it
// is checked anyway because the procedure is process-wide and the range is a
// property of the installing call, not of the procedure.
func foregroundEventProc(
	_ uintptr,
	event uint32,
	hwnd uintptr,
	_ int32,
	_ int32,
	_ uint32,
	_ uint32,
) uintptr {
	if event != eventSystemForeground {
		return 0
	}

	current := activeForegroundWatcher.Load()
	if current == nil {
		return 0
	}

	current.callback(hwnd)

	return 0
}

func (w *ForegroundWatcher) run() {
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()
	defer close(w.doneCh)

	hook, _, _ := procSetWinEventHook.Call(
		eventSystemForeground,
		eventSystemForeground,
		0,
		foregroundEventProcPtr(),
		0,
		0,
		wineventOutOfContext,
	)
	if hook == 0 {
		w.startErr <- errForegroundHookInstallFailed

		return
	}

	// SetWinEventHook is a user32 call, so this thread has a message queue
	// from here on and Stop's PostThreadMessage cannot be lost.
	threadID, _, _ := procGetCurrentThreadID.Call()
	w.threadID = uint32(threadID)
	activeForegroundWatcher.Store(w)

	close(w.startErr)

	defer func() {
		_, _, _ = procUnhookWinEvent.Call(hook)

		activeForegroundWatcher.CompareAndSwap(w, nil)
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

// WindowApplicationIdentity resolves a foreground HWND to the identity
// FocusedApplicationIdentity produces for the focused app: the executable's
// base name and its full path, which is what per-app configuration keys on.
// The bool is false when the handle stands for no application (null,
// destroyed, or the desktop) or its process cannot be read.
func WindowApplicationIdentity(hwnd uintptr) (string, string, bool) {
	handle, err := usableWindowHandle(windows.HWND(hwnd))
	if err != nil {
		return "", "", false
	}

	pid, err := windowProcessID(handle)
	if err != nil {
		return "", "", false
	}

	path, err := processImagePath(pid)
	if err != nil {
		return "", "", false
	}

	return applicationNameFromPath(path), path, true
}
