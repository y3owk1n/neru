//go:build windows

package windows

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// Low-level keyboard hook (WH_KEYBOARD_LL) for global key capture.
// Does not dispatch Neru mode logic; callers receive normalized key events.
//
// keyboardHookStopJoinTimeout bounds how long Stop waits for the hook goroutine
// to exit before reaping it in the background. The normal teardown completes in
// well under a millisecond (WM_QUIT wakes GetMessage), so this only ever trips
// when the hook thread is stuck inside a Windows call, as described in Stop.
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
	pmNoRemove   = 0x0000
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
	callback func(key string, isUp bool) bool
	stopCh   chan struct{}
	doneCh   chan struct{}
	startErr chan error
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

	// activeKeyboardHook is the hook keyboardHookProc dispatches to. It is
	// atomic because the two sides never share a lock: run stores it under h.mu
	// on the hook goroutine, and the hook procedure loads it on whichever
	// thread Windows delivers the key event on.
	activeKeyboardHook atomic.Pointer[KeyboardHook]

	// keyboardHookProcPtr is the WH_KEYBOARD_LL procedure pointer
	// SetWindowsHookExW is handed, allocated on first use and never again —
	// for the reason monitorEnumProcPtr in win32.go carries in full: a callback
	// slot is never freed and the process gets a fixed 2000 of them.
	//
	// StartKeyboardHook runs on every EventTap.Enable cycle, so a callback
	// allocated there was spent on every mode activation. One procedure serves
	// every hook because it carries no per-hook state: it reads
	// activeKeyboardHook, which is whichever hook is currently installed.
	keyboardHookProcPtr = sync.OnceValue(func() uintptr {
		return syscall.NewCallback(keyboardHookProc)
	})
)

var (
	errKeyboardHookCallbackNil   = errors.New("keyboard hook callback is nil")
	errKeyboardHookInstallFailed = errors.New(
		"SetWindowsHookExW failed: keyboard hook not installed",
	)
)

// StartKeyboardHook installs a WH_KEYBOARD_LL hook and begins dispatching events.
func StartKeyboardHook(callback func(key string, isUp bool) bool) (*KeyboardHook, error) {
	if callback == nil {
		return nil, errKeyboardHookCallbackNil
	}

	hook := &KeyboardHook{
		callback: callback,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
		startErr: make(chan error, 1),
	}

	go hook.run()

	// Wait for the hook goroutine to report whether SetWindowsHookExW
	// succeeded. Without this, Enable stores a hook handle and flips
	// enabled=true even when no hook was actually installed, leaving grid and
	// hints mode with no keyboard input.
	select {
	case err := <-hook.startErr:
		if err != nil {
			return nil, err
		}
	case <-hook.doneCh:
		return nil, errKeyboardHookInstallFailed
	}

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
		// reaches while blocked). The in-loop stopCh check still covers a Stop
		// that fires before the first GetMessage. The queue exists by then (run
		// creates it before signaling readiness), and the WM_QUIT this leaves
		// queued dies with the locked thread.
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
	// Mode exit calls Stop while holding the handler mutex, and the key
	// callback no longer takes it: the event tap classifies the key on the
	// hook thread and hands it to its own dispatcher, so a key in flight
	// cannot park this thread on the lock. The bound is a backstop for a hook
	// thread wedged inside a Windows call. Past it the join moves to the
	// background: the goroutine observes stopCh/WM_QUIT and exits, and
	// human-scale latency before the next mode re-enable makes a double-hook
	// overlap during that window effectively impossible.
	select {
	case <-h.doneCh:
	case <-time.After(keyboardHookStopJoinTimeout):
		go func() { <-h.doneCh }()
	}
}

// keyboardHookProc is the WH_KEYBOARD_LL procedure Windows calls for every key
// event, for whichever hook is currently installed.
//
// lParam is typed unsafe.Pointer (not uintptr) so the KBDLLHOOKSTRUCT
// dereference is a Pointer->*T conversion, which keeps go vet's unsafeptr
// check happy. syscall.NewCallback accepts pointer-kind parameters.
func keyboardHookProc(code int, wParam uintptr, lParam unsafe.Pointer) uintptr {
	if code < 0 {
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, uintptr(lParam))

		return ret
	}

	current := activeKeyboardHook.Load()
	if current == nil || current.callback == nil {
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, uintptr(lParam))

		return ret
	}

	kbd := (*kbdLLHookStruct)(lParam)

	// Keys this process injected come back through this hook. Handing one
	// to the callback would feed the mode handler its own output, and the
	// down/up pair a modified scroll holds reads as the user tapping that
	// modifier — latching a sticky modifier nobody pressed. Only Neru's
	// own injection is skipped (neruInjectedTag); another tool's synthetic
	// input is still seen.
	if kbd.dwExtraInfo == neruInjectedTag {
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, uintptr(lParam))

		return ret
	}

	isUp := wParam == wmKeyUp || wParam == wmSysKeyUp || kbd.flags&llkhfUp != 0

	// A physical modifier release is what a modifier hold cannot read from
	// the keyboard, so it is recorded here before the callback can consume
	// the event.
	if isUp && isModifierVirtualKey(kbd.vkCode) {
		noteModifierReleased(kbd.vkCode)
	}

	key := hookKeyName(kbd.vkCode, isUp)
	if key != "" && current.callback(key, isUp) {
		return 1
	}

	ret, _, _ := procCallNextHookEx.Call(0, uintptr(code), wParam, uintptr(lParam))

	return ret
}

func (h *KeyboardHook) run() {
	// The hook and its message queue are thread-affine (doc.go), and the
	// thread is deliberately never unlocked: a goroutine that exits while
	// locked takes its OS thread down with it, queue included. Stop's WM_QUIT
	// is consumed by the GetMessage it wakes, but a Stop that lands before the
	// first GetMessage leaves it queued, and a thread handed back to the
	// scheduler with a WM_QUIT in its queue ends the next message pump that
	// runs on it. The display and foreground watchers lock whatever thread
	// they start on and would exit before their first real message.
	runtime.LockOSThread()

	defer close(h.doneCh)

	handle, _, _ := procSetWindowsHookExW.Call(
		whKeyboardLL,
		keyboardHookProcPtr(),
		moduleHandle(),
		0,
	)
	if handle == 0 {
		// Report the install failure so StartKeyboardHook returns an error
		// instead of a hook that never receives input.
		h.startErr <- errKeyboardHookInstallFailed

		return
	}

	// A thread gets a message queue on its first call that needs one, and
	// PostThreadMessage to a thread without one fails. Stop posts WM_QUIT as
	// soon as StartKeyboardHook returns, which can be before the loop's first
	// GetMessage, so the queue is forced into existence here, ahead of the
	// readiness signal. PeekMessage with PM_NOREMOVE is the documented way.
	var probe msg

	discardCall(procPeekMessageW.Call(uintptr(unsafe.Pointer(&probe)), 0, 0, 0, pmNoRemove))

	h.mu.Lock()
	h.hook = handle
	threadID, _, _ := procGetCurrentThreadID.Call()
	h.threadID = uint32(threadID)
	activeKeyboardHook.Store(h)
	h.mu.Unlock()

	// Signal successful install so StartKeyboardHook can return the hook.
	close(h.startErr)

	defer func() {
		h.mu.Lock()
		if h.hook != 0 {
			_, _, _ = procUnhookWindowsHookEx.Call(h.hook)
			h.hook = 0
		}

		// Only this hook's own registration is cleared: a later hook may
		// already have installed itself, and clearing unconditionally would
		// leave that one receiving key events with nowhere to deliver them.
		activeKeyboardHook.CompareAndSwap(h, nil)
		h.mu.Unlock()
	}()

	var message msg
	for {
		select {
		case <-h.stopCh:
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
