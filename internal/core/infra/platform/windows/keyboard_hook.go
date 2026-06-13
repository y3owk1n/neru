//go:build windows

// internal/core/infra/platform/windows/keyboard_hook.go
// Low-level keyboard hook (WH_KEYBOARD_LL) for global key capture.
// Does not dispatch Neru mode logic; callers receive normalized key events.

package windows

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

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
	procGetCurrentThreadId  = kernel32.NewProc("GetCurrentThreadId")

	activeKeyboardHook *KeyboardHook
)

// StartKeyboardHook installs a WH_KEYBOARD_LL hook and begins dispatching events.
func StartKeyboardHook(callback func(key string, isUp bool)) (*KeyboardHook, error) {
	if callback == nil {
		return nil, fmt.Errorf("keyboard hook callback is nil")
	}

	hook := &KeyboardHook{
		callback: callback,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}

	go hook.run()

	return hook, nil
}

func (h *KeyboardHook) run() {
	defer close(h.doneCh)

	hookProc := syscall.NewCallback(func(code int, wParam uintptr, lParam uintptr) uintptr {
		if code < 0 {
			ret, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, lParam)

			return ret
		}

		current := activeKeyboardHook
		if current == nil || current.callback == nil {
			ret, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, lParam)

			return ret
		}

		kbd := (*kbdLLHookStruct)(unsafe.Pointer(lParam))
		isUp := wParam == wmKeyUp || wParam == wmSysKeyUp || kbd.flags&llkhfUp != 0
		key := hookKeyName(kbd.vkCode, isUp)
		if key != "" {
			current.callback(key, isUp)
		}

		ret, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, lParam)

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
	threadID, _, _ := procGetCurrentThreadId.Call()
	h.threadID = uint32(threadID)
	activeKeyboardHook = h
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		if h.hook != 0 {
			procUnhookWindowsHookEx.Call(h.hook)
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
				procPostThreadMessageW.Call(
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

		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
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
	})
	<-h.doneCh
}
