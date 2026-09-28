//go:build windows

package windows

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

// This file picks the keyboard layout that punctuation is named in. Letters
// and digits never need it, because every layout keeps VK_A to VK_Z and VK_0 to
// VK_9 on the keys of the same name, Russian included. Punctuation lives on the
// VK_OEM_* codes, whose characters come from the layout, so on a non-Latin
// layout they type letters Neru cannot bind. The reference layout is the active
// one when its letter row types ASCII, else the first installed layout whose
// letter row does, else the active one. X11 and macOS follow the same rule.

const (
	// mapvkVscToVk is MapVirtualKeyEx's MAPVK_VSC_TO_VK mode: translate a
	// scan code to the virtual-key code it carries in a given layout.
	mapvkVscToVk = 1

	// toUnicodeNoStateChange is ToUnicodeEx's flag bit 2, available from
	// Windows 10 1607. It translates without touching the keyboard state, so
	// the check does not consume a dead key the user is typing.
	toUnicodeNoStateChange = 0x4

	// The letter row's scan codes, Q through P on a US keyboard.
	letterRowFirstScan = 0x10
	letterRowLastScan  = 0x19

	// maxKeyboardLayouts bounds GetKeyboardLayoutList.
	maxKeyboardLayouts = 64
)

var (
	procGetKeyboardLayout        = user32.NewProc("GetKeyboardLayout")
	procGetKeyboardLayoutList    = user32.NewProc("GetKeyboardLayoutList")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	procMapVirtualKeyExW         = user32.NewProc("MapVirtualKeyExW")
	procToUnicodeEx              = user32.NewProc("ToUnicodeEx")
	procVkKeyScanExW             = user32.NewProc("VkKeyScanExW")
)

// asciiLayouts caches layoutIsASCII per layout handle. A layout's keys never
// change while it is loaded, so the answer is computed once.
var asciiLayouts sync.Map

// fallbackLayout remembers the reference chosen for the last non-ASCII active
// layout, so GetKeyboardLayoutList stays off the hook path until the user
// switches layout. A layout installed meanwhile counts from the next switch.
var fallbackLayout atomic.Pointer[[2]uintptr]

// referenceLayout returns the layout handle punctuation is named and parsed in.
func referenceLayout() uintptr {
	active := activeLayout()
	if layoutIsASCII(active) {
		return active
	}

	if last := fallbackLayout.Load(); last != nil && last[0] == active {
		return last[1]
	}

	reference := firstASCIILayout(active)
	fallbackLayout.Store(&[2]uintptr{active, reference})

	return reference
}

// forgetFallbackLayout drops the cached fallback and reports whether there was
// one. A lookup that failed in the reference layout calls it and tries again,
// because the cached layout may have been unloaded while the active one stayed.
func forgetFallbackLayout() bool {
	return fallbackLayout.Swap(nil) != nil
}

// firstASCIILayout returns the first installed layout whose letter row types
// ASCII, or active when none does.
func firstASCIILayout(active uintptr) uintptr {
	var layouts [maxKeyboardLayouts]uintptr

	count, _, _ := procGetKeyboardLayoutList.Call(
		uintptr(len(layouts)),
		uintptr(unsafe.Pointer(&layouts[0])),
	)
	for _, layout := range layouts[:count] {
		if layoutIsASCII(layout) {
			return layout
		}
	}

	return active
}

// activeLayout is the layout of the window the user is typing in. Neru's own
// threads have a layout of their own, which need not follow the user's
// switches, so the foreground window's thread is asked instead. With no
// foreground window it falls back to the calling thread's layout.
func activeLayout() uintptr {
	var thread uintptr
	if hwnd := windows.GetForegroundWindow(); hwnd != 0 {
		thread, _, _ = procGetWindowThreadProcessID.Call(uintptr(hwnd), 0)
	}

	layout, _, _ := procGetKeyboardLayout.Call(thread)

	return layout
}

// layoutIsASCII reports whether every letter-row key types a printable ASCII
// character, unshifted, in layout. It asks ToUnicodeEx rather than looking at
// virtual-key codes, because a Russian layout keeps VK_Q on the key that types
// й.
func layoutIsASCII(layout uintptr) bool {
	if cached, ok := asciiLayouts.Load(layout); ok {
		ascii, _ := cached.(bool)

		return ascii
	}

	ascii := true

	var (
		state [256]byte
		buf   [4]uint16
	)

	for scan := uintptr(letterRowFirstScan); scan <= letterRowLastScan; scan++ {
		virtualKey, _, _ := procMapVirtualKeyExW.Call(scan, mapvkVscToVk, layout)

		written, _, _ := procToUnicodeEx.Call(
			virtualKey,
			scan,
			uintptr(unsafe.Pointer(&state[0])),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),
			toUnicodeNoStateChange,
			layout,
		)
		if int32(written) != 1 || buf[0] <= 0x20 || buf[0] >= 0x7F {
			ascii = false

			break
		}
	}

	asciiLayouts.Store(layout, ascii)

	return ascii
}
