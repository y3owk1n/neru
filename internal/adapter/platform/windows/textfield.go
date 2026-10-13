//go:build windows

package windows

import (
	"errors"
	"fmt"
	"image"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
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

	errTextFieldNotForeground = errors.New("the hint search field did not get the foreground")
	errTextFieldAlreadyOpen   = errors.New("a hint search field is already open")
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
	closing  atomic.Bool
	done     chan struct{}
	startErr chan error
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

	err := <-field.startErr
	if err != nil {
		<-field.done

		return nil, err
	}

	return field, nil
}

// Close destroys the field, waits for its thread to end, and gives the
// foreground back to the window the user was in. No event fires after it
// returns.
func (f *TextField) Close() {
	if f.closing.Swap(true) {
		<-f.done

		return
	}

	discardCall(procFieldPostMessageW.Call(f.hwnd, wmClose, 0, 0))
	<-f.done

	if f.previous != 0 {
		discardCall(procFieldSetForegroundWindow.Call(uintptr(f.previous)))
	}
}

func (f *TextField) run(bounds image.Rectangle) {
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()
	defer close(f.done)
	defer activeTextField.CompareAndSwap(f, nil)

	err := f.create(bounds)
	if err != nil {
		f.closing.Store(true)

		if f.hwnd != 0 {
			discardCall(procDestroyWindow.Call(f.hwnd))
			f.pump()
		}

		f.startErr <- err

		return
	}

	f.startErr <- nil

	f.pump()
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
		return fmt.Errorf("CreateWindowExW: %w", callErr)
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
		return fmt.Errorf("CreateWindowExW(EDIT): %w", callErr)
	}

	f.edit = edit
	f.editProc, _, _ = procFieldSetWindowLongPtrW.Call(edit, gwlpWndProc, textFieldEditProcPtr())

	discardCall(procShowWindow.Call(hwnd, swShow))

	f.takeForeground()

	if windows.GetForegroundWindow() != windows.HWND(hwnd) {
		return errTextFieldNotForeground
	}

	discardCall(procFieldSetFocus.Call(edit))

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
			if wParam>>16 == enChange && field.events.QueryChanged != nil {
				field.events.QueryChanged(field.query())
			}

			return 0
		case wmActivate:
			// Losing the foreground ends the search, so the keyboard hook comes
			// back on rather than leaving no one reading keys.
			if wParam&0xFFFF == waInactive && !field.closing.Load() && field.events.Cancel != nil {
				field.events.Cancel()
			}
		case wmClose:
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
			if field.events.Confirm != nil {
				field.events.Confirm()
			}

			return 0
		case vkEscape:
			if field.events.Cancel != nil {
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
			errTextFieldClass = fmt.Errorf("RegisterClassExW: %w", err)
		}
	})

	return errTextFieldClass
}
