//go:build windows

package windows

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
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

	// A layout handle's high word says what kind of layout it is.
	layoutHighWordShift = 16
	layoutKindMask      = 0xF000
	layoutKindVariant   = 0xF000
	layoutKindIME       = 0xE000

	// keyboardLayoutsKey is where Windows lists the keyboard layouts it has,
	// one subkey per keyboard layout identifier.
	keyboardLayoutsKey = `SYSTEM\CurrentControlSet\Control\Keyboard Layouts`

	// referenceLayoutPollInterval is how often the watcher checks for a layout
	// switch. A punctuation hotkey lags a switch by at most this long.
	referenceLayoutPollInterval = time.Second
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

// forcedLayout is the layout handle general.kb_layout_to_use forces, or 0 for
// the automatic choice. forcedUnmatched is whether the last value it named
// matched no installed layout.
var (
	forcedLayout    atomic.Uintptr
	forcedUnmatched atomic.Bool
)

// SetReferenceKeyboardLayout forces the layout punctuation is named and parsed
// in, by its keyboard layout identifier as Windows writes it, such as
// 00010409 for US Dvorak, or returns to the automatic choice for "". It reports
// false when no installed layout has that identifier, and the automatic choice
// applies.
func SetReferenceKeyboardLayout(layoutID string) bool {
	forcedLayout.Store(0)
	forcedUnmatched.Store(false)

	if layoutID == "" {
		return true
	}

	for layout, id := range installedLayoutIDs() {
		if strings.EqualFold(id, layoutID) {
			forcedLayout.Store(layout)

			return true
		}
	}

	forcedUnmatched.Store(true)

	return false
}

// referenceLayout returns the layout handle punctuation is named and parsed in.
func referenceLayout() uintptr {
	if forced := forcedLayout.Load(); forced != 0 {
		return forced
	}

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

// installedLayoutIDs maps each installed layout handle to its keyboard layout
// identifier. Windows has no call for this, so it decodes the handle. For a
// primary layout the handle's high word is the identifier itself, so 0x0409 is
// 00000409. For a variant such as Dvorak it is an index into the registry's
// Layout Id values, so 0xF002 is the layout whose Layout Id is 0002. For an
// input method editor the identifier is the whole handle.
func installedLayoutIDs() map[uintptr]string {
	var layouts [maxKeyboardLayouts]uintptr

	count, _, _ := procGetKeyboardLayoutList.Call(
		uintptr(len(layouts)),
		uintptr(unsafe.Pointer(&layouts[0])),
	)

	ids := make(map[uintptr]string, count)
	for _, layout := range layouts[:count] {
		if id := layoutID(layout); id != "" {
			ids[layout] = id
		}
	}

	return ids
}

func layoutID(layout uintptr) string {
	high := uint32(layout>>layoutHighWordShift) & loWordMask

	switch high & layoutKindMask {
	case layoutKindVariant:
		return variantLayoutID(high &^ layoutKindMask)
	case layoutKindIME:
		return fmt.Sprintf("%08X", uint32(layout))
	default:
		return fmt.Sprintf("%08X", high)
	}
}

// variantLayoutID finds the keyboard layout identifier whose registry Layout Id
// is registryID.
func variantLayoutID(registryID uint32) string {
	layouts, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		keyboardLayoutsKey,
		registry.ENUMERATE_SUB_KEYS,
	)
	if err != nil {
		return ""
	}
	defer func() { _ = layouts.Close() }()

	names, err := layouts.ReadSubKeyNames(-1)
	if err != nil {
		return ""
	}

	for _, name := range names {
		if layoutRegistryID(name) == registryID {
			return strings.ToUpper(name)
		}
	}

	return ""
}

// layoutRegistryID reads the Layout Id value of one keyboard layout, or 0 when
// it has none.
func layoutRegistryID(name string) uint32 {
	layout, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		keyboardLayoutsKey+`\`+name,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return 0
	}
	defer func() { _ = layout.Close() }()

	value, _, err := layout.GetStringValue("Layout Id")
	if err != nil {
		return 0
	}

	id, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return 0
	}

	return uint32(id)
}

// KeyboardLayouts lists the installed layouts by keyboard layout identifier,
// as general.kb_layout_to_use takes them, and the one punctuation is named in.
// The last result is true when kb_layout_to_use names no installed layout.
func KeyboardLayouts() ([]string, string, bool) {
	ids := installedLayoutIDs()

	names := slices.Sorted(maps.Values(ids))

	return names, ids[referenceLayout()], forcedUnmatched.Load()
}

// layoutWatch runs the reference layout watcher SetReferenceLayoutChangeHandler
// starts, and stops it.
var layoutWatch struct {
	mu   sync.Mutex
	stop chan struct{}
}

// SetReferenceLayoutChangeHandler calls handler whenever the reference layout
// changes, or stops calling the previous one for nil. RegisterHotKey keeps the
// virtual key a hotkey resolved to, so a punctuation hotkey has to be
// registered again when the user switches to a layout that puts the character
// on another key. Windows tells a background process neither about that switch
// nor about a switch inside the focused window, so the watcher polls.
func SetReferenceLayoutChangeHandler(handler func()) {
	layoutWatch.mu.Lock()
	defer layoutWatch.mu.Unlock()

	if layoutWatch.stop != nil {
		close(layoutWatch.stop)
		layoutWatch.stop = nil
	}

	if handler == nil {
		return
	}

	layoutWatch.stop = make(chan struct{})

	// The starting layout is read here rather than in the goroutine, so a
	// switch right after this returns is seen as a change.
	go watchReferenceLayout(handler, referenceLayout(), layoutWatch.stop)
}

func watchReferenceLayout(handler func(), last uintptr, stop <-chan struct{}) {
	ticker := time.NewTicker(referenceLayoutPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if current := referenceLayout(); current != last {
				last = current

				handler()
			}
		}
	}
}
