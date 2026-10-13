//go:build integration && windows

package windows

import (
	"context"
	"image"
	"maps"
	"runtime"
	"slices"
	"strconv"
	"testing"
	"time"
	"unsafe"

	"go.uber.org/zap"
	"golang.org/x/sys/windows"

	"github.com/y3owk1n/neru/internal/config"
)

// Real UI Automation test against a fixture window this process owns. The
// fixture nests a button inside a child pane so the walk has to reach depth
// two below the window: a walk that only reads the window's direct children
// reports three controls where four exist.
//
// The window is a caption-less popup: an overlapped window's title bar adds
// Minimize, Maximize and Close as Button hints of its own. Controls are
// counted by role because a Win32 edit box takes its UIA Name from a
// neighboring label rather than its own text.
//
// The window and the query share one locked thread: Win32 controls answer the
// MSAA proxy's WM_GETTEXT with SendMessage, which is synchronous on the
// creating thread and would need a message pump from any other.

var (
	moduser32                = windows.NewLazySystemDLL("user32.dll")
	modkernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procFixtureCreateWindowW = moduser32.NewProc("CreateWindowExW")
	procFixtureDestroyWindow = moduser32.NewProc("DestroyWindow")
	procFixtureSetWindowLong = moduser32.NewProc("SetWindowLongPtrW")
	procFixtureSetForeground = moduser32.NewProc("SetForegroundWindow")
	procFixtureGetForeground = moduser32.NewProc("GetForegroundWindow")
	procFixtureSetFocus      = moduser32.NewProc("SetFocus")
	procFixtureModuleHandleW = modkernel32.NewProc("GetModuleHandleW")
)

const (
	fixturePopup    = 0x80000000
	fixtureVisible  = 0x10000000
	fixtureChild    = 0x40000000
	fixtureCheckBox = 0x2
	fixtureDisabled = 0x08000000
	fixturePassword = 0x20
	// fixtureControlID is a control id the Win32 UIA proxy reports as the
	// control's AutomationId, and gwlpID is the window slot that holds it.
	fixtureControlID   = 4242
	gwlpID             = ^uintptr(11) // GWLP_ID, -12
	fixtureWindowSize  = 320
	fixtureControlSize = 40

	// fixtureDeadline is the budget hints mode gives element collection
	// (modes.HintTimeout); the adapter cannot import modes, so the value is
	// restated here.
	fixtureDeadline = 5 * time.Second
)

func createFixtureWindow(
	t *testing.T,
	class, title string,
	style uintptr,
	origin image.Point,
	width, height, parent uintptr,
) uintptr {
	t.Helper()

	className, err := windows.UTF16PtrFromString(class)
	if err != nil {
		t.Fatalf("UTF16PtrFromString: %v", err)
	}

	windowName, err := windows.UTF16PtrFromString(title)
	if err != nil {
		t.Fatalf("UTF16PtrFromString: %v", err)
	}

	module, _, _ := procFixtureModuleHandleW.Call(0)

	hwnd, _, callErr := procFixtureCreateWindowW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		style,
		uintptr(origin.X),
		uintptr(origin.Y),
		width,
		height,
		parent,
		0,
		module,
		0,
	)
	if hwnd == 0 {
		if parent == 0 {
			t.Skipf("skipping: CreateWindowExW requires an interactive desktop (%v)", callErr)
		}

		t.Fatalf("CreateWindowExW(%s): %v", class, callErr)
	}

	if parent == 0 {
		t.Cleanup(func() { discardCall(procFixtureDestroyWindow.Call(hwnd)) })
	}

	return hwnd
}

func TestBuildTree_ReportsControlsNestedBelowTheWindow(t *testing.T) {
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	const child = fixtureChild | fixtureVisible

	window := createFixtureWindow(t, "STATIC", "neru-uia-fixture",
		fixturePopup|fixtureVisible,
		image.Pt(0, 0), fixtureWindowSize, fixtureWindowSize, 0)

	createFixtureWindow(t, "BUTTON", "neru-fixture-button", child,
		image.Pt(10, 10), fixtureControlSize*3, fixtureControlSize, window)
	createFixtureWindow(t, "EDIT", "neru-fixture-edit", child,
		image.Pt(10, 60), fixtureControlSize*3, fixtureControlSize, window)
	createFixtureWindow(t, "BUTTON", "neru-fixture-checkbox", child|fixtureCheckBox,
		image.Pt(10, 110), fixtureControlSize*3, fixtureControlSize, window)

	pane := createFixtureWindow(t, "STATIC", "neru-fixture-pane", child,
		image.Pt(10, 160), fixtureControlSize*4, fixtureControlSize*2, window)
	createFixtureWindow(t, "BUTTON", "neru-fixture-nested-button", child,
		image.Pt(10, 10), fixtureControlSize*3, fixtureControlSize, pane)

	roles := map[string]struct{}{uiaControlButton: {}, uiaControlEdit: {}, uiaControlCheckBox: {}}

	opts := DefaultTreeOptions(zap.NewNop())
	opts.SetRoles(roles)

	ctx, cancel := context.WithTimeout(context.Background(), fixtureDeadline)
	defer cancel()

	started := time.Now()

	tree, err := BuildTree(ctx, &Element{hwnd: window}, opts)
	if err != nil {
		t.Fatalf("BuildTree took %v and failed: %v", time.Since(started), err)
	}

	hints := ProcessClickableNodes(tree, config.HintsConfig{})

	want := map[string]int{uiaControlButton: 2, uiaControlCheckBox: 1, uiaControlEdit: 1}
	got := make(map[string]int, len(want))
	nested := false

	for _, hint := range hints {
		got[hint.Info().Role()]++

		if hint.Info().Title() == "neru-fixture-nested-button" {
			nested = true
		}
	}

	for role, count := range want {
		if got[role] != count {
			t.Errorf("%s hints = %d, want %d (all roles: %v)", role, got[role], count, got)
		}
	}

	if !nested {
		t.Errorf("the button nested inside the child pane was not reported; hints: %v", got)
	}

	t.Logf("built tree with %d hints in %v", len(hints), time.Since(started))
}

func TestBuildTree_ReadsNoDeeperThanMaxDepth(t *testing.T) {
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	const (
		child  = fixtureChild | fixtureVisible
		depth1 = "neru-depth-1"
		depth2 = "neru-depth-2"
	)

	window := createFixtureWindow(t, "STATIC", "neru-uia-depth-fixture",
		fixturePopup|fixtureVisible,
		image.Pt(0, 0), fixtureWindowSize, fixtureWindowSize, 0)

	createFixtureWindow(t, "BUTTON", depth1, child,
		image.Pt(10, 10), fixtureControlSize*3, fixtureControlSize, window)

	pane := createFixtureWindow(t, "STATIC", "neru-depth-pane", child,
		image.Pt(10, 60), fixtureControlSize*4, fixtureControlSize*2, window)
	createFixtureWindow(t, "BUTTON", depth2, child,
		image.Pt(10, 10), fixtureControlSize*3, fixtureControlSize, pane)

	tests := []struct {
		name     string
		maxDepth int
		want     []string
	}{
		{"one level", 1, []string{depth1}},
		{"two levels", 2, []string{depth1, depth2}},
		{"unlimited", 0, []string{depth1, depth2}},
		{"the default", config.DefaultMaxDepth, []string{depth1, depth2}},
	}

	// The cases run inline, not as subtests. t.Run would move each one to
	// another goroutine, off the thread that owns the fixture window, and the
	// Win32 controls would then block the query, as the file comment explains.
	for _, test := range tests {
		opts := DefaultTreeOptions(zap.NewNop())
		opts.SetRoles(map[string]struct{}{uiaControlButton: {}})
		opts.SetMaxDepth(test.maxDepth)

		ctx, cancel := context.WithTimeout(context.Background(), fixtureDeadline)

		tree, err := BuildTree(ctx, &Element{hwnd: window}, opts)

		cancel()

		if err != nil {
			t.Fatalf("%s: BuildTree failed: %v", test.name, err)
		}

		hints := ProcessClickableNodes(tree, config.HintsConfig{})

		got := make([]string, 0, len(hints))
		for _, hint := range hints {
			got = append(got, hint.Info().Title())
		}

		slices.Sort(got)

		if !slices.Equal(got, test.want) {
			t.Errorf("%s: hinted %v, want %v", test.name, got, test.want)
		}
	}
}

func TestBuildTree_ReportsTextIdentifierAndFocusButNoDisabledOrPassword(t *testing.T) {
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	const child = fixtureChild | fixtureVisible

	window := createFixtureWindow(t, "STATIC", "neru-uia-fixture",
		fixturePopup|fixtureVisible,
		image.Pt(0, 0), fixtureWindowSize, fixtureWindowSize, 0)

	edit := createFixtureWindow(t, "EDIT", "neru-fixture-edit-text", child,
		image.Pt(10, 10), fixtureControlSize*3, fixtureControlSize, window)
	enabled := createFixtureWindow(t, "BUTTON", "neru-fixture-enabled", child,
		image.Pt(10, 60), fixtureControlSize*3, fixtureControlSize, window)
	createFixtureWindow(t, "BUTTON", "neru-fixture-disabled", child|fixtureDisabled,
		image.Pt(10, 110), fixtureControlSize*3, fixtureControlSize, window)
	createFixtureWindow(t, "EDIT", "neru-fixture-password", child|fixturePassword,
		image.Pt(10, 160), fixtureControlSize*3, fixtureControlSize, window)

	discardCall(procFixtureSetWindowLong.Call(enabled, gwlpID, fixtureControlID))

	// UI Automation reports keyboard focus only inside the foreground window,
	// which a runner may refuse to hand over, so focus is expected only when
	// the window got it.
	discardCall(procFixtureSetForeground.Call(window))
	discardCall(procFixtureSetFocus.Call(edit))

	foreground, _, _ := procFixtureGetForeground.Call()
	if foreground != window {
		t.Log("the fixture window could not take the foreground, so focus is not checked")
	}

	scan := func(readState bool) []*TreeNode {
		opts := DefaultTreeOptions(zap.NewNop())
		opts.SetRoles(map[string]struct{}{uiaControlButton: {}, uiaControlEdit: {}})
		opts.SetReadState(readState)

		ctx, cancel := context.WithTimeout(context.Background(), fixtureDeadline)
		defer cancel()

		tree, err := BuildTree(ctx, &Element{hwnd: window}, opts)
		if err != nil {
			t.Fatalf("BuildTree failed: %v", err)
		}

		return ProcessClickableNodes(tree, config.HintsConfig{})
	}

	var editValues []string

	for _, hint := range scan(false) {
		if hint.Info().Role() == uiaControlEdit {
			editValues = append(editValues, hint.Info().Value())
		}
	}

	// A Win32 edit takes its UIA name from a neighboring label, so the two
	// edits are told apart by their values: the plain edit reports its text
	// and the password edit reports none.
	slices.Sort(editValues)

	if !slices.Equal(editValues, []string{"", "neru-fixture-edit-text"}) {
		t.Errorf("edit values = %q, want the plain edit's text and an empty password value",
			editValues)
	}

	for _, readState := range []bool{true, false} {
		buttons := map[string]string{}
		editFocused := false

		for _, hint := range scan(readState) {
			info := hint.Info()

			switch {
			case info.Role() == uiaControlButton:
				buttons[info.Title()] = info.Identifier()
			case info.Value() == "neru-fixture-edit-text":
				editFocused = info.IsFocused()
			}
		}

		want := map[string]string{"neru-fixture-enabled": ""}
		if readState {
			want["neru-fixture-enabled"] = strconv.Itoa(fixtureControlID)
		}

		if !maps.Equal(buttons, want) {
			t.Errorf(
				"read state %v: buttons and identifiers = %v, want %v (the disabled one skipped)",
				readState,
				buttons,
				want,
			)
		}

		if wantFocused := readState && foreground == window; editFocused != wantFocused {
			t.Errorf("read state %v: the focused edit reported focused %v, want %v",
				readState, editFocused, wantFocused)
		}
	}
}
