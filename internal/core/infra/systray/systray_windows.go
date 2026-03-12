//go:build windows

package systray

// MenuItem represents a menu item in the system tray (Windows stub).
type MenuItem struct {
	ClickedCh chan struct{}
}

// Title returns the menu item title (Windows stub).
func (m *MenuItem) Title() string { return "" }

// Disabled returns whether the menu item is disabled (Windows stub).
func (m *MenuItem) Disabled() bool { return false }

// Checked returns whether the menu item is checked (Windows stub).
func (m *MenuItem) Checked() bool { return false }

// Hidden returns whether the menu item is hidden (Windows stub).
func (m *MenuItem) Hidden() bool { return false }

// SetTitle sets the menu item title (Windows stub).
func (m *MenuItem) SetTitle(title string) {}

// SetTooltip sets the menu item tooltip (Windows stub).
func (m *MenuItem) SetTooltip(tooltip string) {}

// SetIcon sets the menu item icon (Windows stub).
func (m *MenuItem) SetIcon(icon []byte) {}

// Enable enables the menu item (Windows stub).
func (m *MenuItem) Enable() {}

// Disable disables the menu item (Windows stub).
func (m *MenuItem) Disable() {}

// Check checks the menu item (Windows stub).
func (m *MenuItem) Check() {}

// Uncheck unchecks the menu item (Windows stub).
func (m *MenuItem) Uncheck() {}

// Show shows the menu item (Windows stub).
func (m *MenuItem) Show() {}

// Hide hides the menu item (Windows stub).
func (m *MenuItem) Hide() {}

// AddSubMenuItem adds a sub menu item to the menu item (Windows stub).
func (m *MenuItem) AddSubMenuItem(title string) *MenuItem {
	return &MenuItem{ClickedCh: make(chan struct{}, 1)}
}

// AddSeparator adds a separator to the menu item (Windows stub).
func (m *MenuItem) AddSeparator() {}

// Run starts the system tray loop (Windows stub).
func Run(onReady, onExit func()) {}

// RunHeadless starts the system tray loop without a status icon (Windows stub).
func RunHeadless(onReady, onExit func()) {}

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
	return &MenuItem{ClickedCh: make(chan struct{}, 1)}
}

// AddSeparator adds a separator to the system tray (Windows stub).
func AddSeparator() {}
