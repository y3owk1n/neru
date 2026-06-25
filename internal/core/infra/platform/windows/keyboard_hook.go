//go:build windows

// internal/core/infra/platform/windows/keyboard_hook.go
// Low-level keyboard hook (WH_KEYBOARD_LL) for global key capture.
// Does not dispatch Neru mode logic; callers receive normalized key events.

package windows

import (
	"errors"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// keyboardHookStopJoinTimeout bounds how long Stop waits for the hook goroutine
// to exit before reaping it in the background. The normal teardown completes in
// well under a millisecond (WM_QUIT wakes GetMessage), so this only ever trips
// in the lock-inversion race described in Stop.
const keyboardHookStopJoinTimeout = 250 * time.Millisecond

const (
	whKeyboardLL = 13
	wmKeyDown    = 0x0100
	wmKeyUp      = 0x0101
	wmSysKeyDown = 0x0104
	wmSysKeyUp   = 0x0105
	wmQuit       = 0x0012
	llkhfUp      = 0x0080
	pmRemove     = 0x0001
)

type kbdLLHookStruct struct {
	vkCode      uint32
	scanCode    uint32
	flags       uint32
	time        uint32
	dwExtraInfo uintptr
}

type msg struct {
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

// KeyboardHook captures global key events via WH_KEYBOARD_LL.
type KeyboardHook struct {
	mu       sync.Mutex
	hook     uintptr
	threadID uint32
	callback func(key string, isUp bool)
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
}

var (
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
	procGetCurrentThreadID  = kernel32.NewProc("GetCurrentThreadId")

	activeKeyboardHook *KeyboardHook
)

var errKeyboardHookCallbackNil = errors.New("keyboard hook callback is nil")

// StartKeyboardHook installs a WH_KEYBOARD_LL hook and begins dispatching events.
func StartKeyboardHook(callback func(key string, isUp bool)) (*KeyboardHook, error) {
	if callback == nil {
		return nil, errKeyboardHookCallbackNil
	}

	hook := &KeyboardHook{
		callback: callback,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}

	go hook.run()

	return hook, nil
}

func hookKeyName(vk uint32, isUp bool) string {
	if isUp {
		return KeyNameFromVirtualKey(vk)
	}

	return KeyComboFromVirtualKey(vk)
}

// Stop removes the keyboard hook and waits for the hook thread to exit.
func (h *KeyboardHook) Stop() {
	if h == nil {
		return
	}

	h.stopOnce.Do(func() {
		close(h.stopCh)

		// Wake the hook thread's GetMessage pump. GetMessage blocks until a
		// message arrives, so once the run goroutine is parked inside it,
		// closing stopCh alone never returns control to the loop. Posting
		// WM_QUIT to the hook thread makes GetMessage return 0 and end the
		// loop. Without this, Stop deadlocks on doneCh (the loop's own
		// stopCh check only runs between GetMessage calls, which it never
		// reaches while blocked). The in-loop check still covers the race
		// where Stop fires before the first GetMessage.
		h.mu.Lock()
		threadID := h.threadID
		h.mu.Unlock()

		if threadID != 0 {
			_, _, _ = procPostThreadMessageW.Call(
				uintptr(threadID),
				wmQuit,
				0,
				0,
			)
		}
	})

	// Wait for the hook goroutine to exit, but never block the caller forever.
	// The hook's key callback acquires the handler mutex (HandleKeyPress), and
	// mode-exit calls Stop while holding that same mutex. If a key event is
	// in-flight (e.g. a modifier key-up after a Shift+click), the callback is
	// parked on the mutex and the goroutine cannot finish until the caller
	// returns and releases it. Joining synchronously here would deadlock, so
	// fall back to reaping in the background: the caller returns, releases the
	// mutex, the callback drains, and the goroutine then observes stopCh/WM_QUIT
	// and exits. Human-scale latency before the next mode re-enable makes a
	// double-hook overlap during that window effectively impossible.
	select {
	case <-h.doneCh:
	case <-time.After(keyboardHookStopJoinTimeout):
		go func() { <-h.doneCh }()
	}
}

func (h *KeyboardHook) run() {
	defer close(h.doneCh)

	// lParam is typed unsafe.Pointer (not uintptr) so the KBDLLHOOKSTRUCT
	// dereference is a Pointer->*T conversion, which keeps go vet's unsafeptr
	// check happy. syscall.NewCallback accepts pointer-kind parameters.
	hookProc := syscall.NewCallback(func(code int, wParam uintptr, lParam unsafe.Pointer) uintptr {
		if code < 0 {
			ret, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, uintptr(lParam))

			return ret
		}

		current := activeKeyboardHook
		if current == nil || current.callback == nil {
			ret, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, uintptr(lParam))

			return ret
		}

		kbd := (*kbdLLHookStruct)(lParam)
		isUp := wParam == wmKeyUp || wParam == wmSysKeyUp || kbd.flags&llkhfUp != 0

		key := hookKeyName(kbd.vkCode, isUp)
		if key != "" {
			current.callback(key, isUp)
		}

		ret, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, uintptr(lParam))

		return ret
	})

	handle, _, _ := procSetWindowsHookExW.Call(
		whKeyboardLL,
		hookProc,
		moduleHandle(),
		0,
	)
	if handle == 0 {
		return
	}

	h.mu.Lock()
	h.hook = handle
	threadID, _, _ := procGetCurrentThreadID.Call()
	h.threadID = uint32(threadID)
	activeKeyboardHook = h
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		if h.hook != 0 {
			_, _, _ = procUnhookWindowsHookEx.Call(h.hook)
			h.hook = 0
		}

		if activeKeyboardHook == h {
			activeKeyboardHook = nil
		}
		h.mu.Unlock()
	}()

	var message msg
	for {
		select {
		case <-h.stopCh:
			if h.threadID != 0 {
				_, _, _ = procPostThreadMessageW.Call(
					uintptr(h.threadID),
					wmQuit,
					0,
					0,
				)
			}

			return
		default:
		}

		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&message)),
			0,
			0,
			0,
		)
		if ret == 0 || int32(ret) == -1 {
			return
		}

		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}
