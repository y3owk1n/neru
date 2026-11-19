//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#include "../../platform/darwin/systray.h"
*/
import "C"

import (
	"sync"
	"unsafe"
)

var (
	menuItems     = make(map[int]*MenuItem)
	menuItemsLock sync.RWMutex
	// nextID must start at 1. 0 is reserved as the sentinel for the main menu
	// in the Objective-C NeruAddSeparator function (parentId == 0 means root menu).
	nextID  = 1
	onReady func()
	onExit  func()
)

// MenuItem is a menu item in the system tray.
type MenuItem struct {
	ClickedCh chan struct{}
	id        int
	mu        sync.RWMutex
	checked   bool
}

// Checked returns whether the menu item is checked.
func (m *MenuItem) Checked() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.checked
}

// Run starts the system tray loop. It must be called from the main thread.
func Run(onReadyFunc, onExitFunc func()) {
	onReady = onReadyFunc
	onExit = onExitFunc
	C.NeruNativeLoop()
}

// RunHeadless starts the system tray loop without a status icon.
// It must be called from the main thread.
func RunHeadless(onReadyFunc, onExitFunc func()) {
	onReady = onReadyFunc
	onExit = onExitFunc
	C.NeruNativeLoopHeadless()
}

// Quit quits the application.
func Quit() {
	C.NeruQuit()
}

// SetTitle sets the title of the system tray icon.
func SetTitle(title string) {
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	C.NeruSetTitle(cTitle)
}

// SetTemplateIcon sets the icon of the system tray item as a template icon (monochrome).
func SetTemplateIcon(icon []byte, template bool) {
	if len(icon) == 0 {
		return
	}
	cIcon := (*C.char)(unsafe.Pointer(&icon[0]))
	C.NeruSetIcon(cIcon, C.int(len(icon)), C.bool(template))
}

// AddSeparator adds a separator to the main menu.
func AddSeparator() {
	C.NeruAddSeparator(C.int(0))
}

// AddSeparator adds a separator to a submenu.
func (m *MenuItem) AddSeparator() {
	C.NeruAddSeparator(C.int(m.id))
}

// AddMenuItem adds a menu item to the system tray menu.
func AddMenuItem(title string) *MenuItem {
	item := &MenuItem{
		ClickedCh: make(chan struct{}, 1),
	}
	item.id = registerMenuItem(item)

	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))

	C.NeruAddMenuItem(C.int(item.id), cTitle, C.short(0), C.short(0))

	return item
}

// AddSubMenuItem adds a nested menu item to a parent menu item.
func (m *MenuItem) AddSubMenuItem(title string) *MenuItem {
	item := &MenuItem{
		ClickedCh: make(chan struct{}, 1),
	}
	item.id = registerMenuItem(item)

	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))

	C.NeruAddSubMenuItem(C.int(m.id), C.int(item.id), cTitle, C.short(0), C.short(0))

	return item
}

// SetTitle sets the title of the menu item.
func (m *MenuItem) SetTitle(title string) {
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	C.NeruSetItemTitle(C.int(m.id), cTitle)
}

// Enable enables the menu item.
func (m *MenuItem) Enable() {
	C.NeruSetItemDisabled(C.int(m.id), C.short(0))
}

// Disable disables the menu item.
func (m *MenuItem) Disable() {
	C.NeruSetItemDisabled(C.int(m.id), C.short(1))
}

// Check checks the menu item.
func (m *MenuItem) Check() {
	m.mu.Lock()
	m.checked = true
	m.mu.Unlock()
	C.NeruSetItemChecked(C.int(m.id), C.short(1))
}

// Uncheck unchecks the menu item.
func (m *MenuItem) Uncheck() {
	m.mu.Lock()
	m.checked = false
	m.mu.Unlock()
	C.NeruSetItemChecked(C.int(m.id), C.short(0))
}

// Hide hides the menu item.
func (m *MenuItem) Hide() {
	C.NeruHideMenuItem(C.int(m.id))
}

// Show shows the menu item.
func (m *MenuItem) Show() {
	C.NeruShowMenuItem(C.int(m.id))
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
