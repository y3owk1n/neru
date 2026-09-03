//go:build windows

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

// Pure-Go IUIAutomation (COM) element discovery for the Windows hints mode.
// Does not perform actions; it returns a flat list of on-screen, clickable
// controls at any depth under the given top-level window.
//
// roleUnknown is the AX-style role returned for UIA control types that neru
// does not treat as clickable hint targets.
const roleUnknown = "AXUnknown"

// discardCall consumes a fire-and-forget COM/oleaut32 syscall result. These
// release calls have no actionable failure path; the sink keeps errcheck happy
// without a `_, _, _ =` assignment (which trips dogsled).
func discardCall(uintptr, uintptr, error) {}

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
	vtElementFromHandle       = 6
	vtGetControlViewCondition = 18
	vtCreateCacheRequest      = 20

	// IUIAutomationElement. Only the cached getters are called: every property
	// the walk reads is fetched by the cache request, so a Current* getter
	// would be a second cross-process call for data already in hand.
	vtFindAllBuildCache          = 8
	vtGetCachedControlType       = 53
	vtGetCachedName              = 55
	vtGetCachedIsOffscreen       = 70
	vtGetCachedBoundingRectangle = 75

	// IUIAutomationCacheRequest.
	vtCacheAddProperty              = 3
	vtCachePutTreeFilter            = 9
	vtCachePutAutomationElementMode = 11

	// IUIAutomationElementArray.
	vtArrayGetLength  = 3
	vtArrayGetElement = 4
)

// UIA property ids the cache request prefetches (UIA_*PropertyId).
const (
	propBoundingRectangle = 30001
	propControlType       = 30003
	propName              = 30005
	propIsOffscreen       = 30022
)

// AutomationElementMode_None: cached elements carry only the requested
// properties and no live reference back to the provider, which is all the
// walk needs since every value is copied out before the array is released.
const automationElementModeNone = 0

// UI Automation control-type names referenced from more than one place.
const (
	uiaControlButton      = "Button"
	uiaControlEdit        = "Edit"
	uiaControlHyperlink   = "Hyperlink"
	uiaControlCustom      = "Custom"
	uiaControlSplitButton = "SplitButton"
	uiaControlPane        = "Pane"
	uiaControlCheckBox    = "CheckBox"
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
	50002: uiaControlCheckBox,
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

// winRect mirrors the Win32 RECT returned by get_CachedBoundingRectangle.
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
// comCall invokes the method at index in this object's COM vtable.
//
// CGO is disabled on Windows (see the justfile), so UI Automation is driven
// through raw vtable calls rather than a C wrapper. All COM work for one
// enumeration happens on a single locked OS thread — CoInitialize, object
// creation, property reads, release — and every property is copied into a plain
// Go value before its object is released, so no COM pointer escapes this file
// or crosses a goroutine.
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

	hresult = comCall(automation, vtGetControlViewCondition, uintptr(unsafe.Pointer(&condition)))
	if failed(hresult) || condition == nil {
		return nil
	}
	defer comCall(condition, vtRelease)

	cache := createCacheRequest(automation, condition)
	if cache == nil {
		return nil
	}
	defer comCall(cache, vtRelease)

	var array unsafe.Pointer

	// One cross-process round trip fetches the whole control-view subtree with
	// every property the walk reads already attached, so depth costs nothing
	// per node.
	hresult = comCall(
		root,
		vtFindAllBuildCache,
		uintptr(treeScopeDescendants),
		uintptr(condition),
		uintptr(cache),
		uintptr(unsafe.Pointer(&array)),
	)
	if failed(hresult) || array == nil {
		return nil
	}
	defer comCall(array, vtRelease)

	return collectArray(array, keptRoles)
}

// createCacheRequest builds the cache request FindAllBuildCache fills: the
// four properties extractWinElement reads, filtered to the control view so
// the provider never serializes raw-view scaffolding.
func createCacheRequest(automation unsafe.Pointer, filter unsafe.Pointer) unsafe.Pointer {
	var cache unsafe.Pointer

	hresult := comCall(automation, vtCreateCacheRequest, uintptr(unsafe.Pointer(&cache)))
	if failed(hresult) || cache == nil {
		return nil
	}

	for _, property := range []uintptr{propControlType, propName, propBoundingRectangle, propIsOffscreen} {
		if failed(comCall(cache, vtCacheAddProperty, property)) {
			comCall(cache, vtRelease)

			return nil
		}
	}

	if failed(comCall(cache, vtCachePutTreeFilter, uintptr(filter))) ||
		failed(comCall(cache, vtCachePutAutomationElementMode, automationElementModeNone)) {
		comCall(cache, vtRelease)

		return nil
	}

	return cache
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
// Role selection happens here rather than downstream because the cache request
// returns the whole control-view subtree: rejecting an element by role before
// its bounds and name are decoded keeps the per-element work to one cached
// read.
func extractWinElement(element unsafe.Pointer, keptRoles map[string]struct{}) (winElement, bool) {
	var controlType int32
	if failed(comCall(element, vtGetCachedControlType, uintptr(unsafe.Pointer(&controlType)))) {
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
	if !failed(comCall(element, vtGetCachedIsOffscreen, uintptr(unsafe.Pointer(&offscreen)))) &&
		offscreen != 0 {
		return winElement{}, false
	}

	var rect winRect
	if failed(comCall(element, vtGetCachedBoundingRectangle, uintptr(unsafe.Pointer(&rect)))) {
		return winElement{}, false
	}

	bounds := image.Rect(int(rect.left), int(rect.top), int(rect.right), int(rect.bottom))
	if bounds.Empty() {
		return winElement{}, false
	}

	return winElement{
		bounds:    bounds,
		role:      role,
		name:      cachedName(element),
		clickable: true,
	}, true
}

// cachedName reads the element's cached name (BSTR) and frees it.
func cachedName(element unsafe.Pointer) string {
	var bstr *uint16
	if failed(comCall(element, vtGetCachedName, uintptr(unsafe.Pointer(&bstr)))) || bstr == nil {
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
