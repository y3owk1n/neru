//go:build windows

// Pure-Go IUIAutomation (COM) element discovery for the Windows hints mode.
// Does not perform actions or build a deep cached tree; it returns a flat
// list of on-screen, clickable controls for the given top-level window.

package windows

import (
	"image"
	"runtime"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// roleUnknown is the AX-style role returned for UIA control types that neru
// does not treat as clickable hint targets.
const roleUnknown = "AXUnknown"

// discardCall consumes a fire-and-forget COM/oleaut32 syscall result. These
// release calls have no actionable failure path; the sink keeps errcheck happy
// without a `_, _, _ =` assignment (which trips dogsled).
func discardCall(uintptr, uintptr, error) {}

// CGO is disabled on Windows (see justfile), so UI Automation is driven
// through raw COM vtable calls rather than a C wrapper. All COM work for a
// single enumeration happens on one locked OS thread: CoInitialize, object
// creation, property reads, and release. Every property is copied into a
// plain Go value before the COM object is released, so no COM pointer ever
// escapes this file or crosses a goroutine boundary.

var (
	modole32    = windows.NewLazySystemDLL("ole32.dll")
	modoleaut32 = windows.NewLazySystemDLL("oleaut32.dll")

	procCoInitializeEx   = modole32.NewProc("CoInitializeEx")
	procCoUninitialize   = modole32.NewProc("CoUninitialize")
	procCoCreateInstance = modole32.NewProc("CoCreateInstance")
	procSysFreeString    = modoleaut32.NewProc("SysFreeString")
)

const (
	// Multithreaded apartment: UIA enumeration runs on a background worker
	// goroutine with no Windows message pump, so MTA is required to avoid the
	// STA cross-process marshaling deadlock (Microsoft recommends MTA for UIA
	// calls made off the UI thread).
	coinitMultithreaded = 0x0
	clsctxInprocServer  = 0x1

	// TreeScope_Descendants: every element below the root, at any depth.
	treeScopeDescendants = 0x4

	// COM HRESULT success codes returned by CoInitializeEx when this call owns
	// initialization on the thread (and therefore must balance CoUninitialize).
	hresultSOK    = 0
	hresultSFalse = 1
)

// COM GUIDs for the default UI Automation client object and interface.
var (
	clsidCUIAutomation = guidMust("{FF48DBA4-60EF-4201-AA87-54103EEF594E}")
	iidIUIAutomation   = guidMust("{30CBE57D-D9D0-452A-AB13-7AC5AC4825EE}")
)

func guidMust(value string) windows.GUID {
	guid, err := windows.GUIDFromString(value)
	if err != nil {
		panic(err)
	}

	return guid
}

// Vtable slot indices (IUnknown occupies 0,1,2). These match the public
// UIAutomationClient IDL and have been stable since Windows 7.
const (
	vtRelease = 2

	// IUIAutomation.
	vtElementFromHandle   = 6
	vtCreateTrueCondition = 21

	// IUIAutomationElement.
	vtFindAll                     = 6
	vtGetCurrentControlType       = 21
	vtGetCurrentName              = 23
	vtGetCurrentIsOffscreen       = 38
	vtGetCurrentBoundingRectangle = 43

	// IUIAutomationElementArray.
	vtArrayGetLength  = 3
	vtArrayGetElement = 4
)

// UI Automation control-type names referenced from more than one place.
const (
	uiaControlButton      = "Button"
	uiaControlEdit        = "Edit"
	uiaControlHyperlink   = "Hyperlink"
	uiaControlCustom      = "Custom"
	uiaControlSplitButton = "SplitButton"
	uiaControlPane        = "Pane"
)

// controlTypeNames maps UI Automation CONTROLTYPEID values to the programmatic
// names behind the UIA_*ControlTypeId constants. These names — not the
// localized LocalizedControlType, which changes with the system language — are
// the vocabulary users address with the "uia:" prefix.
//
// The range is contiguous and stable: 50000 is Button and 50040 is AppBar, the
// last control type added (Windows 8).
var controlTypeNames = map[int32]string{
	50000: uiaControlButton,
	50001: "Calendar",
	50002: "CheckBox",
	50003: "ComboBox",
	50004: uiaControlEdit,
	50005: uiaControlHyperlink,
	50006: "Image",
	50007: "ListItem",
	50008: "List",
	50009: "Menu",
	50010: "MenuBar",
	50011: "MenuItem",
	50012: "ProgressBar",
	50013: "RadioButton",
	50014: "ScrollBar",
	50015: "Slider",
	50016: "Spinner",
	50017: "StatusBar",
	50018: "Tab",
	50019: "TabItem",
	50020: "Text",
	50021: "ToolBar",
	50022: "ToolTip",
	50023: "Tree",
	50024: "TreeItem",
	50025: uiaControlCustom,
	50026: "Group",
	50027: "Thumb",
	50028: "DataGrid",
	50029: "DataItem",
	50030: "Document",
	50031: uiaControlSplitButton,
	50032: "Window",
	50033: uiaControlPane,
	50034: "Header",
	50035: "HeaderItem",
	50036: "Table",
	50037: "TitleBar",
	50038: "Separator",
	50039: "SemanticZoom",
	50040: "AppBar",
}

// winRect mirrors the Win32 RECT returned by get_CurrentBoundingRectangle.
type winRect struct {
	left   int32
	top    int32
	right  int32
	bottom int32
}

// winElement is the extracted, COM-free description of a clickable control.
type winElement struct {
	bounds    image.Rectangle
	role      string
	name      string
	clickable bool
}

// comCall invokes the method at vtable slot index on the COM object this.
// It returns the HRESULT (or boolean/handle) in the low bits of the result.
func comCall(this unsafe.Pointer, index int, args ...uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(this)
	method := *(*uintptr)(unsafe.Add(vtbl, uintptr(index)*unsafe.Sizeof(uintptr(0))))

	full := make([]uintptr, 0, len(args)+1)
	full = append(full, uintptr(this))
	full = append(full, args...)

	ret, _, _ := syscall.SyscallN(method, full...)

	return ret
}

// failed reports whether an HRESULT indicates failure (high bit set).
func failed(hresult uintptr) bool {
	return int32(hresult) < 0
}

// enumerateClickableElements returns the on-screen, clickable controls of the
// given top-level window handle. It returns nil on any failure; callers treat
// an empty result as "no hints", never as a crash.
func enumerateClickableElements(hwnd uintptr, keptRoles map[string]struct{}) []winElement {
	if len(keptRoles) == 0 {
		keptRoles = defaultClickableRoles
	}

	if hwnd == 0 {
		return nil
	}

	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	hresult, _, _ := procCoInitializeEx.Call(0, coinitMultithreaded)

	// S_OK and S_FALSE mean this call owns initialization on the thread and
	// must balance it with CoUninitialize. RPC_E_CHANGED_MODE means COM is
	// already up in another mode; leave it alone.
	if uint32(hresult) == hresultSOK || uint32(hresult) == hresultSFalse {
		defer func() { discardCall(procCoUninitialize.Call()) }()
	}

	automation := createAutomation()
	if automation == nil {
		return nil
	}
	defer comCall(automation, vtRelease)

	var root unsafe.Pointer

	hresult = comCall(
		automation,
		vtElementFromHandle,
		hwnd,
		uintptr(unsafe.Pointer(&root)),
	)
	if failed(hresult) || root == nil {
		return nil
	}
	defer comCall(root, vtRelease)

	var condition unsafe.Pointer

	hresult = comCall(automation, vtCreateTrueCondition, uintptr(unsafe.Pointer(&condition)))
	if failed(hresult) || condition == nil {
		return nil
	}
	defer comCall(condition, vtRelease)

	var array unsafe.Pointer

	hresult = comCall(
		root,
		vtFindAll,
		uintptr(treeScopeDescendants),
		uintptr(condition),
		uintptr(unsafe.Pointer(&array)),
	)
	if failed(hresult) || array == nil {
		return nil
	}
	defer comCall(array, vtRelease)

	return collectArray(array, keptRoles)
}

// createAutomation creates the default IUIAutomation instance.
func createAutomation() unsafe.Pointer {
	var automation unsafe.Pointer

	hresult, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidCUIAutomation)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIUIAutomation)),
		uintptr(unsafe.Pointer(&automation)),
	)
	if failed(hresult) {
		return nil
	}

	return automation
}

// collectArray walks an IUIAutomationElementArray and extracts the clickable
// controls. Each element is released as soon as its data is copied out.
func collectArray(array unsafe.Pointer, keptRoles map[string]struct{}) []winElement {
	var length int32

	hresult := comCall(array, vtArrayGetLength, uintptr(unsafe.Pointer(&length)))
	if failed(hresult) || length <= 0 {
		return nil
	}

	result := make([]winElement, 0, length)

	for i := range length {
		var element unsafe.Pointer

		hresult = comCall(array, vtArrayGetElement, uintptr(i), uintptr(unsafe.Pointer(&element)))
		if failed(hresult) || element == nil {
			continue
		}

		extracted, ok := extractWinElement(element, keptRoles)

		comCall(element, vtRelease)

		if ok {
			result = append(result, extracted)
		}
	}

	return result
}

// extractWinElement copies the relevant properties from a single UIA element.
// It returns ok=false for offscreen or zero-size controls, and for controls
// whose role is not in keptRoles.
//
// Role selection happens here rather than downstream because UIA is queried
// with a true condition: the whole subtree comes back, and rejecting an
// element before its bounds and name are read saves three cross-process COM
// calls per unwanted element.
func extractWinElement(element unsafe.Pointer, keptRoles map[string]struct{}) (winElement, bool) {
	var controlType int32
	if failed(comCall(element, vtGetCurrentControlType, uintptr(unsafe.Pointer(&controlType)))) {
		return winElement{}, false
	}

	role, known := controlTypeName(controlType)
	if !known {
		// Unknown provider-specific control type: keep it addressable by id
		// rather than dropping it outright.
		role = strconv.FormatInt(int64(controlType), 10)
	}

	if _, ok := keptRoles[role]; !ok {
		return winElement{}, false
	}

	var offscreen int32
	if !failed(comCall(element, vtGetCurrentIsOffscreen, uintptr(unsafe.Pointer(&offscreen)))) &&
		offscreen != 0 {
		return winElement{}, false
	}

	var rect winRect
	if failed(comCall(element, vtGetCurrentBoundingRectangle, uintptr(unsafe.Pointer(&rect)))) {
		return winElement{}, false
	}

	bounds := image.Rect(int(rect.left), int(rect.top), int(rect.right), int(rect.bottom))
	if bounds.Empty() {
		return winElement{}, false
	}

	return winElement{
		bounds:    bounds,
		role:      role,
		name:      currentName(element),
		clickable: true,
	}, true
}

// currentName reads the element's name (BSTR) and frees it.
func currentName(element unsafe.Pointer) string {
	var bstr *uint16
	if failed(comCall(element, vtGetCurrentName, uintptr(unsafe.Pointer(&bstr)))) || bstr == nil {
		return ""
	}

	name := windows.UTF16PtrToString(bstr)

	discardCall(procSysFreeString.Call(uintptr(unsafe.Pointer(bstr))))

	return name
}

// controlTypeName returns the programmatic name for a UIA CONTROLTYPEID. The
// second result is false for ids outside the known range, which a third-party
// provider may still report; those elements carry the numeric id so they remain
// addressable rather than silently disappearing.
func controlTypeName(controlType int32) (string, bool) {
	name, ok := controlTypeNames[controlType]
	if !ok {
		return roleUnknown, false
	}

	return name, true
}

// defaultClickableRoles is used when the caller supplies no role filter. It is
// the shipped default role list resolved into UIA control-type names, so the
// fallback and a default configuration select exactly the same elements.
var defaultClickableRoles = func() map[string]struct{} {
	resolution := element.ResolveRoles(element.DefaultClickableRoles, "windows")

	set := make(map[string]struct{}, len(resolution.Native))
	for _, native := range resolution.Native {
		set[native] = struct{}{}
	}

	return set
}()
