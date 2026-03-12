//go:build windows

package systray

import (
	"sync"
)

var (
	menuItems     = make(map[int]*MenuItem)
	menuItemsLock sync.RWMutex
	nextID        = 1
)

// MenuItem represents a menu item in the system tray (Windows stub).
type MenuItem struct {
	ClickedCh chan struct{}
	id        int
	mu        sync.RWMutex
	title     string
	disabled  bool
	checked   bool
	hidden    bool
}

// Title returns the menu item title (Windows stub).
func (m *MenuItem) Title() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.title
}

// Disabled returns whether the menu item is disabled (Windows stub).
func (m *MenuItem) Disabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.disabled
}

// Checked returns whether the menu item is checked (Windows stub).
func (m *MenuItem) Checked() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.checked
}

// Hidden returns whether the menu item is hidden (Windows stub).
func (m *MenuItem) Hidden() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.hidden
}

// SetTitle sets the menu item title (Windows stub).
func (m *MenuItem) SetTitle(title string) {
	m.mu.Lock()
	m.title = title
	m.mu.Unlock()
}

// SetTooltip sets the menu item tooltip (Windows stub).
func (m *MenuItem) SetTooltip(tooltip string) {}

// SetIcon sets the menu item icon (Windows stub).
func (m *MenuItem) SetIcon(icon []byte) {}

// Enable enables the menu item (Windows stub).
func (m *MenuItem) Enable() {
	m.mu.Lock()
	m.disabled = false
	m.mu.Unlock()
}

// Disable disables the menu item (Windows stub).
func (m *MenuItem) Disable() {
	m.mu.Lock()
	m.disabled = true
	m.mu.Unlock()
}

// Check checks the menu item (Windows stub).
func (m *MenuItem) Check() {
	m.mu.Lock()
	m.checked = true
	m.mu.Unlock()
}

// Uncheck unchecks the menu item (Windows stub).
func (m *MenuItem) Uncheck() {
	m.mu.Lock()
	m.checked = false
	m.mu.Unlock()
}

// Show shows the menu item (Windows stub).
func (m *MenuItem) Show() {
	m.mu.Lock()
	m.hidden = false
	m.mu.Unlock()
}

// Hide hides the menu item (Windows stub).
func (m *MenuItem) Hide() {
	m.mu.Lock()
	m.hidden = true
	m.mu.Unlock()
}

// AddSubMenuItem adds a sub menu item to the menu item (Windows stub).
func (m *MenuItem) AddSubMenuItem(title string) *MenuItem {
	item := &MenuItem{
		ClickedCh: make(chan struct{}, 1),
		title:     title,
	}
	item.id = registerMenuItem(item)

	return item
}

// AddSeparator adds a separator to the menu item (Windows stub).
func (m *MenuItem) AddSeparator() {}

// Run starts the system tray loop (Windows stub).
func Run(onReadyFunc, onExitFunc func()) {}

// RunHeadless starts the system tray loop without a status icon (Windows stub).
func RunHeadless(onReadyFunc, onExitFunc func()) {}

// Quit quits the application (Windows stub).
func Quit() {}

// SetTitle sets the title of the system tray icon (Windows stub).
func SetTitle(title string) {}

// SetTooltip sets the tooltip of the system tray icon (Windows stub).
func SetTooltip(tooltip string) {}

// SetIcon sets the icon of the system tray icon (Windows stub).
func SetIcon(icon []byte) {}

// SetTemplateIcon sets the icon of the system tray icon as a template icon (Windows stub).
func SetTemplateIcon(icon []byte, template bool) {}

// AddMenuItem adds a menu item to the system tray (Windows stub).
func AddMenuItem(title string) *MenuItem {
	item := &MenuItem{
		ClickedCh: make(chan struct{}, 1),
		title:     title,
	}
	item.id = registerMenuItem(item)

	return item
}

// AddSeparator adds a separator to the system tray (Windows stub).
func AddSeparator() {}

func registerMenuItem(item *MenuItem) int {
	menuItemsLock.Lock()
	defer menuItemsLock.Unlock()

	id := nextID
	nextID++
	menuItems[id] = item

	return id
}

// ResetForTesting resets all global state (Windows stub).
func ResetForTesting() {
	menuItemsLock.Lock()
	defer menuItemsLock.Unlock()

	menuItems = make(map[int]*MenuItem)
	nextID = 1
}
