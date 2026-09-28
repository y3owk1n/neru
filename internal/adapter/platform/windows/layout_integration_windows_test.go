//go:build integration && windows

package windows

import (
	"runtime"
	"slices"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Real Win32 test for the reference layout: with a non-Latin layout active in
// the foreground window, punctuation must still name and parse as the ASCII it
// types on the user's Latin layout. In-package because the foreground window
// helpers are unexported.

const (
	// Keyboard layout identifiers, as LoadKeyboardLayout takes them.
	klidUS      = "00000409"
	klidRussian = "00000419"

	// klfNoTellShell keeps the shell from being told a layout was loaded, so
	// the test does not add an entry to the user's language bar.
	klfNoTellShell = 0x80
)

var (
	procActivateKeyboardLayout = user32.NewProc("ActivateKeyboardLayout")
	procLoadKeyboardLayoutW    = user32.NewProc("LoadKeyboardLayoutW")
	procUnloadKeyboardLayout   = user32.NewProc("UnloadKeyboardLayout")
)

func TestKeyNameFromVirtualKey_NamesPunctuationWhileANonLatinLayoutIsActive(t *testing.T) {
	// activeLayout asks the foreground window's thread, so the window and the
	// layout switch have to share this one.
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	newForegroundTestWindow(t)

	loadTestKeyboardLayout(t, klidUS)
	russian := loadTestKeyboardLayout(t, klidRussian)

	previous, _, _ := procActivateKeyboardLayout.Call(russian, 0)
	t.Cleanup(func() { discardCall(procActivateKeyboardLayout.Call(previous, 0)) })

	if activeLayout() != russian {
		t.Skip("skipping: the session did not make Russian the foreground window's layout")
	}

	if got := KeyNameFromVirtualKey('Q'); got != "q" {
		t.Fatalf("KeyNameFromVirtualKey(VK_Q) with Russian active = %q, want %q", got, "q")
	}

	for _, name := range []string{"`", "/", ";", "[", "'"} {
		virtualKey, ok := nameToVirtualKey(name)
		if !ok {
			t.Fatalf("nameToVirtualKey(%q) with Russian active found no key", name)
		}

		if got := KeyNameFromVirtualKey(virtualKey); got != name {
			t.Fatalf("with Russian active, %q parses to %#x, which names as %q",
				name, virtualKey, got)
		}
	}
}

// loadTestKeyboardLayout loads the layout and returns its handle, unloading it
// afterwards unless the user already had it installed.
func loadTestKeyboardLayout(t *testing.T, klid string) uintptr {
	t.Helper()

	name, err := windows.UTF16PtrFromString(klid)
	if err != nil {
		t.Fatalf("UTF16PtrFromString(%q): %v", klid, err)
	}

	installed := installedKeyboardLayouts()

	layout, _, err := procLoadKeyboardLayoutW.Call(uintptr(unsafe.Pointer(name)), klfNoTellShell)
	if layout == 0 {
		t.Skipf("skipping: LoadKeyboardLayoutW(%s): %v", klid, err)
	}

	if !slices.Contains(installed, layout) {
		t.Cleanup(func() { discardCall(procUnloadKeyboardLayout.Call(layout)) })
	}

	return layout
}

func installedKeyboardLayouts() []uintptr {
	layouts := make([]uintptr, maxKeyboardLayouts)

	count, _, _ := procGetKeyboardLayoutList.Call(
		uintptr(len(layouts)),
		uintptr(unsafe.Pointer(&layouts[0])),
	)

	return layouts[:count]
}

// klidUSDvorak is a variant layout, whose handle carries a registry Layout Id
// rather than the identifier itself.
const klidUSDvorak = "00010409"

// TestSetReferenceKeyboardLayout_ForcesAnInstalledLayoutByIdentifier pins that
// general.kb_layout_to_use names a Windows layout by its keyboard layout
// identifier, a variant such as Dvorak included, that neru doctor lists it,
// and that an identifier no installed layout has is refused.
func TestSetReferenceKeyboardLayout_ForcesAnInstalledLayoutByIdentifier(t *testing.T) {
	russian := loadTestKeyboardLayout(t, klidRussian)
	dvorak := loadTestKeyboardLayout(t, klidUSDvorak)

	t.Cleanup(func() { SetReferenceKeyboardLayout("") })

	for _, test := range []struct {
		klid   string
		layout uintptr
	}{
		{klid: klidRussian, layout: russian},
		{klid: klidUSDvorak, layout: dvorak},
	} {
		if !SetReferenceKeyboardLayout(test.klid) {
			t.Fatalf("SetReferenceKeyboardLayout(%s) found no installed layout", test.klid)
		}

		if got := referenceLayout(); got != test.layout {
			t.Fatalf("forcing %s: reference layout %#x, want %#x", test.klid, got, test.layout)
		}

		names, reference, unmatched := KeyboardLayouts()
		if !slices.Contains(names, test.klid) || reference != test.klid || unmatched {
			t.Fatalf(
				"forcing %s: KeyboardLayouts() = %v, %q, unmatched %v",
				test.klid,
				names,
				reference,
				unmatched,
			)
		}
	}

	if SetReferenceKeyboardLayout("0000FFFF") {
		t.Fatal("SetReferenceKeyboardLayout accepted an identifier no installed layout has")
	}

	if forcedLayout.Load() != 0 {
		t.Fatal("a refused identifier left a layout forced")
	}

	if _, _, unmatched := KeyboardLayouts(); !unmatched {
		t.Fatal("KeyboardLayouts does not report the refused identifier as unmatched")
	}
}

// TestSetReferenceLayoutChangeHandler_CallsBackWhenTheReferenceChanges pins the
// watcher that re-registers hotkeys after a layout switch (#1713): a change of
// reference layout reaches the handler within a poll or two.
func TestSetReferenceLayoutChangeHandler_CallsBackWhenTheReferenceChanges(t *testing.T) {
	loadTestKeyboardLayout(t, klidRussian)

	t.Cleanup(func() {
		SetReferenceLayoutChangeHandler(nil)
		SetReferenceKeyboardLayout("")
	})

	changed := make(chan struct{}, 1)

	SetReferenceLayoutChangeHandler(func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	})

	if !SetReferenceKeyboardLayout(klidRussian) {
		t.Fatal("SetReferenceKeyboardLayout found no Russian layout")
	}

	select {
	case <-changed:
	case <-time.After(3 * referenceLayoutPollInterval):
		t.Fatal("the reference layout changed but the handler was not called")
	}
}
