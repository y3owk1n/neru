//go:build windows

package windows

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The Windows Runtime, driven the way UI Automation is in
// accessibility/native/windows: raw vtable calls through syscall, no CGO
// (the justfile builds Windows with CGO_ENABLED=0). Everything here is the
// plumbing a WinRT class needs before its own methods can be called — the
// apartment, activation factories, HSTRINGs and the IInspectable vtable
// prefix — and nothing here knows which class is being driven.
//
// Interface IDs and vtable orders are read from the Windows SDK headers
// (mingw-w64's copies of windows.media.ocr.h and friends); a wrong slot calls
// the wrong method with the wrong arguments, so every slot constant names the
// header method it stands for.

const (
	// roInitMultithreaded is RO_INIT_MULTITHREADED: the apartment the OCR
	// worker joins. WinRT objects created there are agile and can be used from
	// any thread in it, which is what lets one engine be cached for the
	// process.
	roInitMultithreaded = 1

	// The IInspectable vtable prefix every WinRT interface starts with:
	// IUnknown's three slots then GetIids, GetRuntimeClassName and
	// GetTrustLevel. A class's own methods start at inspectableSlots.
	inspectableSlots = 6

	winrtQueryInterface = 0
	winrtRelease        = 2
)

var (
	combase = windows.NewLazySystemDLL("combase.dll")

	procRoInitialize              = combase.NewProc("RoInitialize")
	procRoGetActivationFactory    = combase.NewProc("RoGetActivationFactory")
	procWindowsCreateString       = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString       = combase.NewProc("WindowsDeleteString")
	procWindowsGetStringRawBuffer = combase.NewProc("WindowsGetStringRawBuffer")
)

// errHRESULT is the static error every failed WinRT call wraps, with the code
// beside it: the code is the one thing a reader can look up, so it is spelled
// the way the SDK spells it.
var errHRESULT = errors.New("WinRT call failed")

// mustGUID parses the braced GUID string the SDK headers print. A typo here is
// a programming error, so it panics at package init rather than at first use.
func mustGUID(value string) windows.GUID {
	guid, err := windows.GUIDFromString(value)
	if err != nil {
		panic(fmt.Sprintf("invalid GUID %q: %v", value, err))
	}

	return guid
}

// hresultFailed reports whether hresult is a failure HRESULT (high bit set).
func hresultFailed(hresult uintptr) bool {
	return int32(hresult) < 0
}

// hresultError names the call that failed and the HRESULT it returned.
func hresultError(what string, hresult uintptr) error {
	return fmt.Errorf("%s: %w with HRESULT 0x%08X", what, errHRESULT, uint32(hresult))
}

// winrtCall invokes the method at vtable slot index on the WinRT object this.
// The object pointer is the implicit first argument; the return value is the
// HRESULT.
func winrtCall(this unsafe.Pointer, index int, args ...uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(this)
	method := *(*uintptr)(unsafe.Add(vtbl, uintptr(index)*unsafe.Sizeof(uintptr(0))))

	full := make([]uintptr, 0, len(args)+1)
	full = append(full, uintptr(this))
	full = append(full, args...)

	ret, _, _ := syscall.SyscallN(method, full...)

	return ret
}

// winrtReleaseObject drops one reference. Nil is accepted so a defer can be
// registered before the object exists.
func winrtReleaseObject(this unsafe.Pointer) {
	if this != nil {
		winrtCall(this, winrtRelease)
	}
}

// winrtQuery asks this for another of its interfaces.
func winrtQuery(this unsafe.Pointer, iid *windows.GUID) (unsafe.Pointer, error) {
	var out unsafe.Pointer

	hresult := winrtCall(this, winrtQueryInterface,
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&out)),
	)
	if hresultFailed(hresult) || out == nil {
		return nil, hresultError("QueryInterface", hresult)
	}

	return out, nil
}

// hstring is a WinRT HSTRING handle. The zero handle is the empty string,
// which WinRT accepts everywhere a string is read.
type hstring uintptr

// newHString allocates an HSTRING for s. The caller deletes it.
func newHString(s string) (hstring, error) {
	utf16, err := windows.UTF16FromString(s)
	if err != nil {
		return 0, err
	}

	var handle hstring

	// The length excludes the terminator UTF16FromString appends.
	hresult, _, _ := procWindowsCreateString.Call(
		uintptr(unsafe.Pointer(&utf16[0])),
		uintptr(len(utf16)-1),
		uintptr(unsafe.Pointer(&handle)),
	)
	if hresultFailed(hresult) {
		return 0, hresultError("WindowsCreateString", hresult)
	}

	return handle, nil
}

// String copies the HSTRING's text into a Go string. WinRT owns the buffer
// only as long as the handle lives, so the copy happens before delete.
func (h hstring) String() string {
	if h == 0 {
		return ""
	}

	var length uint32

	ret, _, _ := procWindowsGetStringRawBuffer.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&length)),
	)
	if ret == 0 || length == 0 {
		return ""
	}

	// Read through the address of the returned word rather than converting
	// it, which keeps go vet's unsafeptr check satisfied the way win32.go does.
	buffer := *(**uint16)(unsafe.Pointer(&ret))

	return windows.UTF16ToString(unsafe.Slice(buffer, length))
}

func (h hstring) delete() {
	if h != 0 {
		discardCall(procWindowsDeleteString.Call(uintptr(h)))
	}
}

// roInitialize joins the calling OS thread to the multithreaded apartment.
// S_OK and S_FALSE both mean the thread is in; RPC_E_CHANGED_MODE means it
// was already initialized in another mode, which no thread Neru owns does.
func roInitialize() error {
	hresult, _, _ := procRoInitialize.Call(roInitMultithreaded)
	if hresultFailed(hresult) {
		return hresultError("RoInitialize", hresult)
	}

	return nil
}

// activationFactory returns the interface iid of the activation factory for
// the runtime class className. The caller releases it.
func activationFactory(className string, iid *windows.GUID) (unsafe.Pointer, error) {
	name, err := newHString(className)
	if err != nil {
		return nil, err
	}
	defer name.delete()

	var factory unsafe.Pointer

	hresult, _, _ := procRoGetActivationFactory.Call(
		uintptr(name),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&factory)),
	)
	if hresultFailed(hresult) || factory == nil {
		return nil, hresultError("RoGetActivationFactory("+className+")", hresult)
	}

	return factory, nil
}

// vectorView is the IVectorView<T> the OCR result hands out for lines and
// words. The interface is generic, but its vtable is not: after the
// IInspectable prefix come GetAt, get_Size, IndexOf and GetMany, in that order
// for every T (windows.foundation.collections.h).
type vectorView struct{ ptr unsafe.Pointer }

const (
	vectorViewGetAt = inspectableSlots + iota
	vectorViewSize
)

// size is the element count.
func (v vectorView) size() (uint32, error) {
	var count uint32

	hresult := winrtCall(v.ptr, vectorViewSize, uintptr(unsafe.Pointer(&count)))
	if hresultFailed(hresult) {
		return 0, hresultError("IVectorView::get_Size", hresult)
	}

	return count, nil
}

// at returns element index, an object the caller releases.
func (v vectorView) at(index uint32) (unsafe.Pointer, error) {
	var item unsafe.Pointer

	hresult := winrtCall(v.ptr, vectorViewGetAt, uintptr(index), uintptr(unsafe.Pointer(&item)))
	if hresultFailed(hresult) || item == nil {
		return nil, hresultError("IVectorView::GetAt", hresult)
	}

	return item, nil
}
