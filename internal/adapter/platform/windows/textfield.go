//go:build windows

package windows

import (
	"image"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The hint-search field: a nearly transparent tool window over the drawn
// search box, holding a standard EDIT control that takes the keyboard while
// search is open. The control gets input-method text through IMM and dead keys
// through TranslateMessage, the way any text box does, so hint search types
// like one. It runs on its own message-loop thread.
//
// The query is keystream. Nothing here logs it.

const (
	textFieldClassName = "NeruHintSearchField"

	wsChild        = 0x40000000
	wsVisible      = 0x10000000
	esAutoHScroll  = 0x0080
	wmCommand      = 0x0111
	wmChar         = 0x0102
	wmActivate     = 0x0006
	wmClose        = 0x0010
	wmDestroy      = 0x0002
	waInactive     = 0
	enChange       = 0x0300
	gwlpWndProc    = ^uintptr(3) // GWLP_WNDPROC, -4
	swShow         = 5
	textFieldAlpha = 1 // fully transparent windows get no input, so one step above
)

var (
	procFieldSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procFieldSetFocus            = user32.NewProc("SetFocus")
	procFieldPostMessageW        = user32.NewProc("PostMessageW")
	procFieldPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procFieldAttachThreadInput   = user32.NewProc("AttachThreadInput")
	procFieldGetWindowTextW      = user32.NewProc("GetWindowTextW")
	procFieldGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	procFieldSetWindowLongPtrW   = user32.NewProc("SetWindowLongPtrW")
	procFieldCallWindowProcW     = user32.NewProc("CallWindowProcW")
	procFieldActivateLayout      = user32.NewProc("ActivateKeyboardLayout")

	// activeTextField is the field the window procedures dispatch to. One
	// search is open at a time, and the procedures run on its thread.
	activeTextField atomic.Pointer[TextField]

	// The procedures are allocated once for the process: a NewCallback slot is
	// never freed (see keyboardHookProcPtr).
	textFieldWndProcPtr = sync.OnceValue(func() uintptr {
		return syscall.NewCallback(textFieldWndProc)
	})
	textFieldEditProcPtr = sync.OnceValue(func() uintptr {
		return syscall.NewCallback(textFieldEditProc)
	})

	textFieldClassOnce sync.Once
	errTextFieldClass  error

	errTextFieldNotForeground = derrors.New(derrors.CodeActionFailed,
		"the hint search field did not get the foreground")
	errTextFieldAlreadyOpen = derrors.New(derrors.CodeActionFailed,
		"a hint search field is already open")
	errTextFieldTimedOut = derrors.New(derrors.CodeTimeout,
		"the hint search field did not open in time")
)

// textFieldWait bounds each wait on the field's thread. The caller holds the
// mode handler's lock, and the thread joins another app's input queue to take
// the foreground, so an app that stopped responding costs at most this.
const textFieldWait = 500 * time.Millisecond

// A field's life: opening until its thread has the foreground, then open, or
// abandoned when the opener stopped waiting first.
const (
	textFieldOpening int32 = iota
	textFieldOpen
	textFieldAbandoned
)

// TextFieldEvents receives what the user does in the field, on the field's
// thread. It must not block.
type TextFieldEvents struct {
	QueryChanged func(query string)
	Confirm      func()
	Cancel       func()
}

// TextField is one open hint-search field.
type TextField struct {
	events   TextFieldEvents
	hwnd     uintptr
	edit     uintptr
	editProc uintptr
	previous windows.HWND
	done     chan struct{}
	startErr chan error

	state   atomic.Int32
	closing atomic.Bool
	// focusTaken is set once the field holds the foreground and the keyboard,
	// and lostForeground when it lost them after that, for example because
	// the user moved to another window, which closing must then leave in
	// front.
	focusTaken     atomic.Bool
	lostForeground atomic.Bool
}

// OpenTextField places the field over bounds, in global coordinates, and gives
// it the keyboard. It fails when Windows does not let the field take the
// foreground, so the caller can keep reading raw keys.
func OpenTextField(bounds image.Rectangle, events TextFieldEvents) (*TextField, error) {
	field := &TextField{
		events:   events,
		previous: windows.GetForegroundWindow(),
		done:     make(chan struct{}),
		startErr: make(chan error, 1),
	}

	if !activeTextField.CompareAndSwap(nil, field) {
		return nil, errTextFieldAlreadyOpen
	}

	go field.run(bounds)

	select {
	case err := <-field.startErr:
		if err != nil {
			return nil, err
		}

		return field, nil
	case <-time.After(textFieldWait):
		if field.state.CompareAndSwap(textFieldOpening, textFieldAbandoned) {
			return nil, errTextFieldTimedOut
		}

		// The field opened just as the wait ran out.
		return field, <-field.startErr
	}
}

// Close destroys the field, which gives the foreground back to the window the
// user was in unless they moved to another one. It waits for the field's
// thread for at most textFieldWait. No event fires after it returns.
func (f *TextField) Close() {
	if !f.closing.Swap(true) {
		discardCall(procFieldPostMessageW.Call(f.hwnd, wmClose, 0, 0))
	}

	select {
	case <-f.done:
	case <-time.After(textFieldWait):
		// The thread ends once Windows lets it, and nothing waits for it.
	}
}

// live reports whether the field may report events: it is open and not
// closing.
func (f *TextField) live() bool {
	return f.state.Load() == textFieldOpen && !f.closing.Load()
}

func (f *TextField) run(bounds image.Rectangle) {
	// The thread is never unlocked, so Go ends it when run returns rather than
	// handing it to other goroutines with the keyboard layout and input state
	// the field gave it. A later FeedKey reads the layout of whichever thread
	// it runs on.
	runtime.LockOSThread()

	defer close(f.done)
	defer activeTextField.CompareAndSwap(f, nil)

	err := f.create(bounds)

	// Focus can move away between taking it and this point, and then the field
	// would hold no keyboard while the caller turns its key capture off.
	if err == nil &&
		(f.lostForeground.Load() || windows.GetForegroundWindow() != windows.HWND(f.hwnd)) {
		err = errTextFieldNotForeground
	}

	// The opener may have stopped waiting, and then nobody will close this
	// field.
	if err == nil && !f.state.CompareAndSwap(textFieldOpening, textFieldOpen) {
		err = errTextFieldTimedOut
	}

	if err != nil {
		f.closing.Store(true)

		if f.hwnd != 0 {
			f.restoreForeground()
			discardCall(procDestroyWindow.Call(f.hwnd))
			f.pump()
		}

		f.startErr <- err

		return
	}

	f.startErr <- nil

	f.pump()
}

// restoreForeground hands the foreground back to the window the user was in,
// while the field still holds it, which is when Windows allows it. A user who
// moved to another window keeps that one.
func (f *TextField) restoreForeground() {
	if f.previous != 0 && !f.lostForeground.Load() &&
		windows.GetForegroundWindow() == windows.HWND(f.hwnd) {
		discardCall(procFieldSetForegroundWindow.Call(uintptr(f.previous)))
	}
}

func (f *TextField) pump() {
	var message msg

	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(ret) <= 0 {
			return
		}

		discardCall(procTranslateMessage.Call(uintptr(unsafe.Pointer(&message))))
		discardCall(procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message))))
	}
}

func (f *TextField) create(bounds image.Rectangle) error {
	err := registerTextFieldClass()
	if err != nil {
		return err
	}

	className, err := windows.UTF16PtrFromString(textFieldClassName)
	if err != nil {
		return err
	}

	editClass, err := windows.UTF16PtrFromString("EDIT")
	if err != nil {
		return err
	}

	hwnd, _, callErr := procCreateWindowExW.Call(
		wsExToolWindow|wsExTopmost|wsExLayered,
		uintptr(unsafe.Pointer(className)),
		0,
		wsPopup,
		uintptr(bounds.Min.X), uintptr(bounds.Min.Y),
		uintptr(bounds.Dx()), uintptr(bounds.Dy()),
		0, 0, moduleHandle(), 0,
	)
	if hwnd == 0 {
		return derrors.Wrap(callErr, derrors.CodeActionFailed, "CreateWindowExW")
	}

	f.hwnd = hwnd

	discardCall(procSetLayeredWindowAttributes.Call(hwnd, 0, textFieldAlpha, lwaAlpha))

	edit, _, callErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(editClass)),
		0,
		wsChild|wsVisible|esAutoHScroll,
		0, 0, uintptr(bounds.Dx()), uintptr(bounds.Dy()),
		hwnd, 0, moduleHandle(), 0,
	)
	if edit == 0 {
		return derrors.Wrap(callErr, derrors.CodeActionFailed, "CreateWindowExW(EDIT)")
	}

	f.edit = edit

	f.editProc, _, callErr = procFieldSetWindowLongPtrW.Call(
		edit,
		gwlpWndProc,
		textFieldEditProcPtr(),
	)
	if f.editProc == 0 {
		return derrors.Wrap(callErr, derrors.CodeActionFailed, "SetWindowLongPtrW")
	}

	discardCall(procShowWindow.Call(hwnd, swShow))

	if f.state.Load() == textFieldAbandoned {
		return errTextFieldTimedOut
	}

	f.takeForeground()

	if windows.GetForegroundWindow() != windows.HWND(hwnd) {
		return errTextFieldNotForeground
	}

	discardCall(procFieldSetFocus.Call(edit))
	f.focusTaken.Store(true)

	return nil
}

// takeForeground asks for the foreground. Windows grants it only to the
// process the user last gave input to, and the key that opened search went to
// Neru's keyboard hook rather than to any window. Joining the foreground
// window's input queue for the call is what lets it through.
func (f *TextField) takeForeground() {
	ourThread, _, _ := procGetCurrentThreadID.Call()

	var foregroundThread uintptr

	if f.previous != 0 {
		foregroundThread, _, _ = procGetWindowThreadProcessID.Call(uintptr(f.previous), 0)
	}

	// A keyboard layout belongs to a thread, so the field's thread starts with
	// Neru's. Taking the layout the user was typing in keeps the same
	// characters and input method in the search.
	if foregroundThread != 0 {
		layout, _, _ := procGetKeyboardLayout.Call(foregroundThread)
		if layout != 0 {
			discardCall(procFieldActivateLayout.Call(layout, 0))
		}
	}

	attached := foregroundThread != 0 && foregroundThread != ourThread
	if attached {
		discardCall(procFieldAttachThreadInput.Call(ourThread, foregroundThread, 1))
	}

	discardCall(procFieldSetForegroundWindow.Call(f.hwnd))

	if attached {
		discardCall(procFieldAttachThreadInput.Call(ourThread, foregroundThread, 0))
	}
}

// query reads the EDIT control's text.
func (f *TextField) query() string {
	length, _, _ := procFieldGetWindowTextLength.Call(f.edit)

	buf := make([]uint16, length+1)
	discardCall(procFieldGetWindowTextW.Call(f.edit, uintptr(unsafe.Pointer(&buf[0])), length+1))

	return windows.UTF16ToString(buf)
}

func textFieldWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	field := activeTextField.Load()

	if field != nil && field.hwnd == hwnd {
		switch message {
		case wmCommand:
			if wParam>>16 == enChange && field.live() && field.events.QueryChanged != nil {
				field.events.QueryChanged(field.query())
			}

			return 0
		case wmActivate:
			// Losing the foreground ends the search, so the keyboard hook comes
			// back on rather than leaving no one reading keys.
			if wParam&0xFFFF == waInactive && field.focusTaken.Load() && !field.closing.Load() {
				field.lostForeground.Store(true)

				if field.live() && field.events.Cancel != nil {
					field.events.Cancel()
				}
			}
		case wmClose:
			field.restoreForeground()
			discardCall(procDestroyWindow.Call(hwnd))

			return 0
		case wmDestroy:
			discardCall(procFieldPostQuitMessage.Call(0))

			return 0
		}
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)

	return ret
}

// textFieldEditProc catches Return and Escape before the EDIT control, which
// would only beep. While an input method is composing, it consumes those keys
// itself and they arrive as VK_PROCESSKEY instead.
func textFieldEditProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	field := activeTextField.Load()
	if field == nil || field.edit != hwnd {
		ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)

		return ret
	}

	switch message {
	case wmKeyDown:
		switch wParam {
		case vkReturn:
			if field.live() && field.events.Confirm != nil {
				field.events.Confirm()
			}

			return 0
		case vkEscape:
			if field.live() && field.events.Cancel != nil {
				field.events.Cancel()
			}

			return 0
		}
	case wmChar:
		if wParam == '\r' || wParam == 0x1B {
			return 0
		}
	}

	ret, _, _ := procFieldCallWindowProcW.Call(
		field.editProc,
		hwnd,
		uintptr(message),
		wParam,
		lParam,
	)

	return ret
}

func registerTextFieldClass() error {
	textFieldClassOnce.Do(func() {
		className, err := windows.UTF16PtrFromString(textFieldClassName)
		if err != nil {
			errTextFieldClass = err

			return
		}

		class := wndClassEx{
			cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
			lpfnWndProc:   textFieldWndProcPtr(),
			hInstance:     windows.Handle(moduleHandle()),
			lpszClassName: className,
		}

		atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
		if atom == 0 {
			errTextFieldClass = derrors.Wrap(err, derrors.CodeActionFailed, "RegisterClassExW")
		}
	})

	return errTextFieldClass
}
