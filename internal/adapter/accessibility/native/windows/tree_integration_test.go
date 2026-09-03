//go:build integration && windows

package windows

import (
	"context"
	"image"
	"runtime"
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
// The window and the query share one locked thread: Win32 controls answer the
// MSAA proxy's WM_GETTEXT with SendMessage, which is synchronous on the
// creating thread and would need a message pump from any other.

var (
	moduser32                = windows.NewLazySystemDLL("user32.dll")
	modkernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procFixtureCreateWindowW = moduser32.NewProc("CreateWindowExW")
	procFixtureDestroyWindow = moduser32.NewProc("DestroyWindow")
	procFixtureModuleHandleW = modkernel32.NewProc("GetModuleHandleW")
)

const (
	fixtureOverlappedWindow = 0x00CF0000
	fixtureVisible          = 0x10000000
	fixtureChild            = 0x40000000
	fixtureCheckBox         = 0x2
	fixtureUseDefault       = 0x80000000
	fixtureWindowSize       = 320
	fixtureControlSize      = 40

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
		fixtureOverlappedWindow|fixtureVisible,
		image.Pt(fixtureUseDefault, fixtureUseDefault), fixtureWindowSize, fixtureWindowSize, 0)

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

	want := map[string]bool{
		"neru-fixture-button":        false,
		"neru-fixture-edit":          false,
		"neru-fixture-checkbox":      false,
		"neru-fixture-nested-button": false,
	}

	for _, hint := range hints {
		if _, known := want[hint.Info().Title()]; known {
			want[hint.Info().Title()] = true
		}
	}

	for title, seen := range want {
		if !seen {
			t.Errorf("hint for %q missing; got %d hints", title, len(hints))
		}
	}

	if len(hints) != len(want) {
		t.Errorf("got %d hints, want %d", len(hints), len(want))
	}

	t.Logf("built tree with %d hints in %v", len(hints), time.Since(started))
}
