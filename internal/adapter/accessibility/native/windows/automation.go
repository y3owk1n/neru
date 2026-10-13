//go:build windows

package windows

import (
	"context"
	"image"
	"runtime"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/y3owk1n/neru/internal/config"
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
	procVariantClear     = modoleaut32.NewProc("VariantClear")
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
	vtElementFromHandle          = 6
	vtElementFromPointBuildCache = 11
	vtGetControlViewWalker       = 14
	vtGetControlViewCondition    = 18
	vtCreateCacheRequest         = 20

	// IUIAutomationTreeWalker.
	vtWalkerGetParentElementBuildCache      = 9
	vtWalkerGetFirstChildElementBuildCache  = 10
	vtWalkerGetNextSiblingElementBuildCache = 12

	// IUIAutomationElement. Only the cached getters are called: every property
	// the walk reads is fetched by the cache request, so a Current* getter
	// would be a second cross-process call for data already in hand.
	vtFindAllBuildCache          = 8
	vtGetCachedPropertyValue     = 12
	vtGetCachedControlType       = 53
	vtGetCachedName              = 55
	vtGetCachedHasKeyboardFocus  = 58
	vtGetCachedIsEnabled         = 60
	vtGetCachedAutomationID      = 61
	vtGetCachedHelpText          = 63
	vtGetCachedIsPassword        = 67
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
	propHasKeyboardFocus  = 30008
	propIsEnabled         = 30010
	propAutomationID      = 30011
	propHelpText          = 30013
	propIsPassword        = 30019
	propIsOffscreen       = 30022
	propValueValue        = 30045
)

// vtBSTR is the VARTYPE of a VARIANT holding a BSTR.
const vtBSTR = 8

// AutomationElementMode_None: cached elements carry only the requested
// properties and no live reference back to the provider, which is all FindAll
// needs since every value is copied out before the array is released.
// AutomationElementMode_Full keeps that reference, which a tree walker needs
// to move from an element to its children and siblings.
const (
	automationElementModeNone = 0
	automationElementModeFull = 1
)

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

// variant mirrors the Win32 VARIANT: a VARTYPE, three reserved words, and a
// union two pointers wide, which is 16 bytes on 386 and 24 on 64-bit targets.
// cachedValue reads only the BSTR member of the union.
type variant struct {
	vt       uint16
	reserved [3]uint16
	bstr     *uint16
	_        uintptr
}

// winElement is the extracted, COM-free description of a clickable control.
// description is the UIA HelpText, which providers fill from a tooltip or an
// ARIA title. value is the Value pattern's string, and stays empty for a
// password field.
type winElement struct {
	bounds      image.Rectangle
	role        string
	name        string
	description string
	value       string
	identifier  string
	enabled     bool
	focused     bool
	clickable   bool
}

// enumerateOptions steers one enumeration.
type enumerateOptions struct {
	// frame is the window's visible frame. Controls outside it are dropped
	// and the rest are clipped to it. An empty frame disables both.
	frame image.Rectangle
	// roles is the set of UIA control-type names to keep. An empty set falls
	// back to the shipped defaults.
	roles map[string]struct{}
	// anyState keeps controls that are not enabled. It is
	// hints.ignore_clickable_check.
	anyState bool
	// maxDepth is the deepest control-view level read below the window, 0 for
	// unlimited. It is hints.max_depth.
	maxDepth int
	// visibleCheck hit-tests each control's center and drops the ones another
	// element covers. It is hints.visible_check_enabled on Windows.
	visibleCheck bool
	// readState also reads each control's identifier and focused state (see
	// ports.ElementFilter.ReadElementState).
	readState bool
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
// given top-level window handle, filtered as opts says. It returns nil on any
// failure; callers treat an empty result as "no hints", never as a crash.
func enumerateClickableElements(
	ctx context.Context,
	hwnd uintptr,
	opts enumerateOptions,
) []winElement {
	keptRoles := opts.roles
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

	// FindAll matches the whole subtree and cannot stop at a depth, so a
	// limit below the default goes to the walker.
	walk := opts.maxDepth > 0 && opts.maxDepth < config.DefaultMaxDepth

	mode := uintptr(automationElementModeNone)
	if walk {
		mode = automationElementModeFull
	}

	cache := createCacheRequest(automation, condition, mode, opts.readState)
	if cache == nil {
		return nil
	}
	defer comCall(cache, vtRelease)

	var controls []winElement

	if walk {
		controls = walkControls(ctx, automation, root, cache, opts.maxDepth, opts.frame, keptRoles,
			opts.anyState, opts.readState)
	} else {
		controls = findAllControls(root, condition, cache, opts.frame, keptRoles, opts.anyState,
			opts.readState)
	}

	if opts.visibleCheck {
		controls = visibleControls(automation, cache, opts.frame, controls)
	}

	return controls
}

// findAllControls returns the clickable controls anywhere below root.
func findAllControls(
	root, condition, cache unsafe.Pointer,
	frame image.Rectangle,
	keptRoles map[string]struct{},
	anyState bool,
	readState bool,
) []winElement {
	var array unsafe.Pointer

	// One cross-process round trip fetches the whole control-view subtree with
	// every property the walk reads already attached, so depth costs nothing
	// per node.
	hresult := comCall(
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

	return collectArray(array, frame, keptRoles, anyState, readState)
}

// walkControls returns the clickable controls no deeper than maxDepth below
// root. The window's children are depth one, as on macOS. It walks the
// control view one element at a time, with one round trip per element,
// because FindAll cannot stop at a depth.
func walkControls(
	ctx context.Context,
	automation, root, cache unsafe.Pointer,
	maxDepth int,
	frame image.Rectangle,
	keptRoles map[string]struct{},
	anyState bool,
	readState bool,
) []winElement {
	var walker unsafe.Pointer

	hresult := comCall(automation, vtGetControlViewWalker, uintptr(unsafe.Pointer(&walker)))
	if failed(hresult) || walker == nil {
		return nil
	}
	defer comCall(walker, vtRelease)

	var result []winElement

	walkChildren(ctx, walker, cache, root, 1, maxDepth, func(element unsafe.Pointer) {
		result = appendControl(result, element, frame, keptRoles, anyState, readState)
	})

	return result
}

// walkChildren calls visit on each control-view child of parent at depth,
// then on their children, down to maxDepth. It stops once ctx is done.
func walkChildren(
	ctx context.Context,
	walker, cache, parent unsafe.Pointer,
	depth, maxDepth int,
	visit func(unsafe.Pointer),
) {
	if depth > maxDepth {
		return
	}

	var child unsafe.Pointer

	hresult := comCall(
		walker,
		vtWalkerGetFirstChildElementBuildCache,
		uintptr(parent),
		uintptr(cache),
		uintptr(unsafe.Pointer(&child)),
	)

	for !failed(hresult) && child != nil {
		if ctx.Err() != nil {
			comCall(child, vtRelease)

			return
		}

		visit(child)
		walkChildren(ctx, walker, cache, child, depth+1, maxDepth, visit)

		var next unsafe.Pointer

		hresult = comCall(
			walker,
			vtWalkerGetNextSiblingElementBuildCache,
			uintptr(child),
			uintptr(cache),
			uintptr(unsafe.Pointer(&next)),
		)

		comCall(child, vtRelease)

		child = next
	}
}

// visibleControls keeps the controls whose center hit-tests to themselves or
// to a descendant, the same gate the AX walk applies behind
// hints.visible_check_enabled. A control another element covers, in this
// window or another, hit-tests to that element and is dropped. A hit-test
// that fails keeps the control, so a provider that cannot answer does not
// blank the hints.
func visibleControls(
	automation, cache unsafe.Pointer,
	frame image.Rectangle,
	controls []winElement,
) []winElement {
	var walker unsafe.Pointer

	hresult := comCall(automation, vtGetControlViewWalker, uintptr(unsafe.Pointer(&walker)))
	if failed(hresult) || walker == nil {
		return controls
	}
	defer comCall(walker, vtRelease)

	kept := controls[:0]

	for _, control := range controls {
		hit := elementAt(automation, cache, centerOf(control.bounds))
		if hit == nil || hitReaches(walker, cache, hit, frame, control) {
			kept = append(kept, control)
		}
	}

	return kept
}

// elementAt returns the cached element under point, or nil when UIA has no
// answer. The caller releases it.
func elementAt(automation, cache unsafe.Pointer, point image.Point) unsafe.Pointer {
	var hit unsafe.Pointer

	hresult := comCall(
		automation,
		vtElementFromPointBuildCache,
		packPoint(point),
		uintptr(cache),
		uintptr(unsafe.Pointer(&hit)),
	)
	if failed(hresult) || hit == nil {
		return nil
	}

	return hit
}

// maxHitAncestors bounds the walk from a hit element up to the window root.
// Real trees are far shallower; the bound only stops a provider that cycles.
const maxHitAncestors = 64

// hitReaches reports whether the hit element, or one of its control-view
// ancestors, is the control: the same walk the AX check does with element
// identity. Cached elements from two queries carry no comparable identity, so
// each ancestor is matched on its control type, name and frame-clipped
// bounds, all three of which the control itself satisfies and a covering
// element does not. It releases hit.
func hitReaches(
	walker, cache unsafe.Pointer,
	hit unsafe.Pointer,
	frame image.Rectangle,
	control winElement,
) bool {
	current := hit

	for range maxHitAncestors {
		candidate, ok := extractWinElement(current, frame, nil)
		if ok && sameControl(candidate, control) {
			comCall(current, vtRelease)

			return true
		}

		var parent unsafe.Pointer

		hresult := comCall(
			walker,
			vtWalkerGetParentElementBuildCache,
			uintptr(current),
			uintptr(cache),
			uintptr(unsafe.Pointer(&parent)),
		)

		comCall(current, vtRelease)

		if failed(hresult) || parent == nil {
			return false
		}

		current = parent
	}

	comCall(current, vtRelease)

	return false
}

// sameControl reports whether two extracted descriptions name one control.
func sameControl(a, b winElement) bool {
	return a.bounds == b.bounds && a.role == b.role && a.name == b.name
}

// packPoint lays a Win32 POINT out as the single register ElementFromPoint
// takes it in: x in the low 32 bits, y in the high 32 bits.
func packPoint(point image.Point) uintptr {
	return uintptr(uint32(int32(point.X))) | uintptr(uint32(int32(point.Y)))<<32
}

func centerOf(rect image.Rectangle) image.Point {
	return image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2)
}

// createCacheRequest builds the cache request that FindAllBuildCache and the
// walker fill, in the given AutomationElementMode. It holds the properties
// extractWinElement and readControlDetails read, filtered to the control view
// so the provider never serializes raw-view scaffolding. It adds
// the identifier and focus properties, which only `neru query hints` reports,
// only with readState, because the provider computes every cached property
// for every control.
func createCacheRequest(
	automation unsafe.Pointer,
	filter unsafe.Pointer,
	mode uintptr,
	readState bool,
) unsafe.Pointer {
	var cache unsafe.Pointer

	hresult := comCall(automation, vtCreateCacheRequest, uintptr(unsafe.Pointer(&cache)))
	if failed(hresult) || cache == nil {
		return nil
	}

	properties := []uintptr{
		propControlType, propName, propBoundingRectangle, propIsOffscreen, propIsEnabled,
		propHelpText, propIsPassword, propValueValue,
	}
	if readState {
		properties = append(properties, propAutomationID, propHasKeyboardFocus)
	}

	for _, property := range properties {
		if failed(comCall(cache, vtCacheAddProperty, property)) {
			comCall(cache, vtRelease)

			return nil
		}
	}

	if failed(comCall(cache, vtCachePutTreeFilter, uintptr(filter))) ||
		failed(comCall(cache, vtCachePutAutomationElementMode, mode)) {
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
func collectArray(
	array unsafe.Pointer,
	frame image.Rectangle,
	keptRoles map[string]struct{},
	anyState bool,
	readState bool,
) []winElement {
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

		result = appendControl(result, element, frame, keptRoles, anyState, readState)

		comCall(element, vtRelease)
	}

	return result
}

// appendControl appends element to result when it is a clickable control. It
// drops a disabled control before reading its text, since a disabled control
// gets no hint, as on macOS, unless anyState keeps it for
// hints.ignore_clickable_check. The caller releases element.
func appendControl(
	result []winElement,
	element unsafe.Pointer,
	frame image.Rectangle,
	keptRoles map[string]struct{},
	anyState bool,
	readState bool,
) []winElement {
	extracted, ok := extractWinElement(element, frame, keptRoles)
	if !ok {
		return result
	}

	extracted.enabled = cachedBool(element, vtGetCachedIsEnabled, true)
	if !extracted.enabled && !anyState {
		return result
	}

	readControlDetails(element, &extracted, readState)

	return append(result, extracted)
}

// extractWinElement copies the relevant properties from a single UIA element.
// It returns ok=false for offscreen or zero-size controls, for controls whose
// role is not in keptRoles (nil keeps every role), and for controls outside
// frame. Bounds are clipped
// to frame so a control straddling the window edge targets its visible part.
//
// Role selection happens here rather than downstream because the cache request
// returns the whole control-view subtree: rejecting an element by role before
// its bounds and name are decoded keeps the per-element work to one cached
// read.
func extractWinElement(
	element unsafe.Pointer,
	frame image.Rectangle,
	keptRoles map[string]struct{},
) (winElement, bool) {
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

	if _, ok := keptRoles[role]; keptRoles != nil && !ok {
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

	bounds := clipToFrame(
		image.Rect(int(rect.left), int(rect.top), int(rect.right), int(rect.bottom)),
		frame,
	)
	if bounds.Empty() {
		return winElement{}, false
	}

	return winElement{
		bounds:    bounds,
		role:      role,
		name:      cachedString(element, vtGetCachedName),
		clickable: true,
	}, true
}

// readControlDetails copies a kept control's description and value and, with
// readState, its identifier and focused state. The visible check matches
// controls on what extractWinElement reads, so these reads happen only for a
// control that becomes a hint.
func readControlDetails(element unsafe.Pointer, control *winElement, readState bool) {
	control.description = cachedString(element, vtGetCachedHelpText)

	if !cachedBool(element, vtGetCachedIsPassword, true) {
		control.value = cachedValue(element)
	}

	if readState {
		control.identifier = cachedString(element, vtGetCachedAutomationID)
		control.focused = cachedBool(element, vtGetCachedHasKeyboardFocus, false)
	}
}

// clipToFrame returns the part of bounds inside the window frame, which is
// empty for a control laid out past the window edge. An empty frame means the
// frame is unknown and bounds come back unchanged.
//
// The provider sets IsOffscreen, and Chromium leaves it false for controls it
// has laid out past the window edge, such as the hidden part of a long
// vertical tab strip in Edge. Clipping to the window frame catches those.
func clipToFrame(bounds, frame image.Rectangle) image.Rectangle {
	if frame.Empty() {
		return bounds
	}

	return bounds.Intersect(frame)
}

// cachedString reads the cached BSTR property behind the getter at slot and
// frees it.
func cachedString(element unsafe.Pointer, slot int) string {
	var bstr *uint16
	if failed(comCall(element, slot, uintptr(unsafe.Pointer(&bstr)))) || bstr == nil {
		return ""
	}

	text := windows.UTF16PtrToString(bstr)

	discardCall(procSysFreeString.Call(uintptr(unsafe.Pointer(bstr))))

	return text
}

// cachedBool reads the cached BOOL property behind the getter at slot, or
// fallback when the provider cannot answer.
func cachedBool(element unsafe.Pointer, slot int, fallback bool) bool {
	var value int32
	if failed(comCall(element, slot, uintptr(unsafe.Pointer(&value)))) {
		return fallback
	}

	return value != 0
}

// cachedValue reads the Value pattern's cached string. A control without the
// pattern answers with an empty VARIANT, which reads as "".
func cachedValue(element unsafe.Pointer) string {
	var value variant
	if failed(
		comCall(element, vtGetCachedPropertyValue, propValueValue, uintptr(unsafe.Pointer(&value))),
	) {
		return ""
	}

	defer func() { discardCall(procVariantClear.Call(uintptr(unsafe.Pointer(&value)))) }()

	if value.vt != vtBSTR || value.bstr == nil {
		return ""
	}

	return windows.UTF16PtrToString(value.bstr)
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
