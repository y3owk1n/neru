package systray

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "systray.h"
*/
import "C"

import (
	"runtime"
	"sync"
	"unsafe"
)

var (
	menuItems     = make(map[int]*MenuItem)
	menuItemsLock sync.RWMutex
	nextID        = 1
	onReady       func()
	onExit        func()
)

// MenuItem represents a menu item in the system tray.
type MenuItem struct {
	ClickedCh chan struct{}
	id        int
	title     string
	tooltip   string
	disabled  bool
	checked   bool
}

// Title returns the menu item title.
func (m *MenuItem) Title() string { return m.title }

// Tooltip returns the menu item tooltip.
func (m *MenuItem) Tooltip() string { return m.tooltip }

// Disabled returns whether the menu item is disabled.
func (m *MenuItem) Disabled() bool { return m.disabled }

// Checked returns whether the menu item is checked.
func (m *MenuItem) Checked() bool { return m.checked }

// Run starts the system tray loop. It must be called from the main thread.
func Run(onReadyFunc, onExitFunc func()) {
	runtime.LockOSThread()
	onReady = onReadyFunc
	onExit = onExitFunc
	C.nativeLoop()
}

// Quit quits the application.
func Quit() {
	C.quit()
}

// SetTitle sets the title of the system tray icon.
func SetTitle(title string) {
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle)) //nolint
	C.setTitle(cTitle)
}

// SetTooltip sets the tooltip of the system tray icon.
func SetTooltip(tooltip string) {
	cTooltip := C.CString(tooltip)
	defer C.free(unsafe.Pointer(cTooltip)) //nolint
	C.setTooltip(cTooltip)
}

// SetIcon sets the icon of the system tray item.
func SetIcon(icon []byte) {
	if len(icon) == 0 {
		return
	}
	cIcon := (*C.char)(unsafe.Pointer(&icon[0]))
	C.setIcon(cIcon, C.int(len(icon)), C.bool(false))
}

// SetTemplateIcon sets the icon of the system tray item as a template icon (monochrome).
func SetTemplateIcon(icon []byte, template bool) {
	if len(icon) == 0 {
		return
	}
	cIcon := (*C.char)(unsafe.Pointer(&icon[0]))
	C.setIcon(cIcon, C.int(len(icon)), C.bool(template))
}

// AddSeparator adds a separator to the main menu.
func AddSeparator() {
	C.add_separator(C.int(0))
}

// AddSeparator adds a separator to a submenu.
func (m *MenuItem) AddSeparator() {
	C.add_separator(C.int(m.id))
}

// AddMenuItem adds a menu item to the system tray menu.
func AddMenuItem(title string, tooltip string) *MenuItem {
	item := &MenuItem{
		ClickedCh: make(chan struct{}, 1),
		title:     title,
		tooltip:   tooltip,
	}
	item.id = registerMenuItem(item)

	cTitle := C.CString(title)
	cTooltip := C.CString(tooltip)
	defer C.free(unsafe.Pointer(cTitle))   //nolint
	defer C.free(unsafe.Pointer(cTooltip)) //nolint

	C.add_menu_item(C.int(item.id), cTitle, cTooltip, C.short(0), C.short(0))

	return item
}

// AddSubMenuItem adds a nested menu item to a parent menu item.
func (m *MenuItem) AddSubMenuItem(title string, tooltip string) *MenuItem {
	item := &MenuItem{
		ClickedCh: make(chan struct{}, 1),
		title:     title,
		tooltip:   tooltip,
	}
	item.id = registerMenuItem(item)

	cTitle := C.CString(title)
	cTooltip := C.CString(tooltip)
	defer C.free(unsafe.Pointer(cTitle))   //nolint
	defer C.free(unsafe.Pointer(cTooltip)) //nolint

	C.add_sub_menu_item(C.int(m.id), C.int(item.id), cTitle, cTooltip, C.short(0), C.short(0))

	return item
}

// SetTitle sets the title of the menu item.
func (m *MenuItem) SetTitle(title string) {
	m.title = title
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle)) //nolint
	C.set_item_title(C.int(m.id), cTitle)
}

// SetTooltip sets the tooltip of the menu item.
func (m *MenuItem) SetTooltip(tooltip string) {
	m.tooltip = tooltip
	cTooltip := C.CString(tooltip)
	defer C.free(unsafe.Pointer(cTooltip)) //nolint
	C.set_item_tooltip(C.int(m.id), cTooltip)
}

// Enable enables the menu item.
func (m *MenuItem) Enable() {
	m.disabled = false
	C.set_item_disabled(C.int(m.id), C.short(0))
}

// Disable disables the menu item.
func (m *MenuItem) Disable() {
	m.disabled = true
	C.set_item_disabled(C.int(m.id), C.short(1))
}

// Check checks the menu item.
func (m *MenuItem) Check() {
	m.checked = true
	C.set_item_checked(C.int(m.id), C.short(1))
}

// Uncheck unchecks the menu item.
func (m *MenuItem) Uncheck() {
	m.checked = false
	C.set_item_checked(C.int(m.id), C.short(0))
}

// Hide hides the menu item.
func (m *MenuItem) Hide() {
	C.hide_menu_item(C.int(m.id))
}

// Show shows the menu item.
func (m *MenuItem) Show() {
	C.show_menu_item(C.int(m.id))
}

//export systray_on_ready
func systray_on_ready() {
	if onReady != nil {
		onReady()
	}
}

//export systray_on_exit
func systray_on_exit() {
	if onExit != nil {
		onExit()
	}
}

//export systray_menu_item_selected
func systray_menu_item_selected(id C.int) {
	menuItemsLock.RLock()
	item, ok := menuItems[int(id)]
	menuItemsLock.RUnlock()
	if ok {
		select {
		case item.ClickedCh <- struct{}{}:
		default:
		}
	}
}

func registerMenuItem(item *MenuItem) int {
	menuItemsLock.Lock()
	defer menuItemsLock.Unlock()

	id := nextID
	nextID++
	menuItems[id] = item

	return id
}

// ResetForTesting resets all global state. Only use in tests.
func ResetForTesting() {
	menuItemsLock.Lock()
	defer menuItemsLock.Unlock()

	menuItems = make(map[int]*MenuItem)
	nextID = 1
	onReady = nil
	onExit = nil
}
