//go:build integration && windows

package windows

import (
	"runtime"
	"slices"
	"testing"
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
