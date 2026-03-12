//go:build linux

package systray

// MenuItem represents a menu item in the system tray (Linux stub).
type MenuItem struct {
	ClickedCh chan struct{}
}

// Title returns the menu item title (Linux stub).
func (m *MenuItem) Title() string { return "" }

// Disabled returns whether the menu item is disabled (Linux stub).
func (m *MenuItem) Disabled() bool { return false }

// Checked returns whether the menu item is checked (Linux stub).
func (m *MenuItem) Checked() bool { return false }

// Hidden returns whether the menu item is hidden (Linux stub).
func (m *MenuItem) Hidden() bool { return false }

// SetTitle sets the menu item title (Linux stub).
func (m *MenuItem) SetTitle(title string) {}

// SetTooltip sets the menu item tooltip (Linux stub).
func (m *MenuItem) SetTooltip(tooltip string) {}

// SetIcon sets the menu item icon (Linux stub).
func (m *MenuItem) SetIcon(icon []byte) {}

// Enable enables the menu item (Linux stub).
func (m *MenuItem) Enable() {}

// Disable disables the menu item (Linux stub).
func (m *MenuItem) Disable() {}

// Check checks the menu item (Linux stub).
func (m *MenuItem) Check() {}

// Uncheck unchecks the menu item (Linux stub).
func (m *MenuItem) Uncheck() {}

// Show shows the menu item (Linux stub).
func (m *MenuItem) Show() {}

// Hide hides the menu item (Linux stub).
func (m *MenuItem) Hide() {}

// AddSubMenuItem adds a sub menu item to the menu item (Linux stub).
func (m *MenuItem) AddSubMenuItem(title string) *MenuItem {
	return &MenuItem{ClickedCh: make(chan struct{}, 1)}
}

// AddSeparator adds a separator to the menu item (Linux stub).
func (m *MenuItem) AddSeparator() {}

// Run starts the system tray loop (Linux stub).
func Run(onReady, onExit func()) {}

// RunHeadless starts the system tray loop without a status icon (Linux stub).
func RunHeadless(onReady, onExit func()) {}

// Quit quits the application (Linux stub).
func Quit() {}

// SetTitle sets the title of the system tray icon (Linux stub).
func SetTitle(title string) {}

// SetTooltip sets the tooltip of the system tray icon (Linux stub).
func SetTooltip(tooltip string) {}

// SetIcon sets the icon of the system tray icon (Linux stub).
func SetIcon(icon []byte) {}

// SetTemplateIcon sets the icon of the system tray icon as a template icon (Linux stub).
func SetTemplateIcon(icon []byte, template bool) {}

// AddMenuItem adds a menu item to the system tray (Linux stub).
func AddMenuItem(title string) *MenuItem {
	return &MenuItem{ClickedCh: make(chan struct{}, 1)}
}

// AddSeparator adds a separator to the system tray (Linux stub).
func AddSeparator() {}

// ResetForTesting resets all global state (Linux stub).
func ResetForTesting() {}
