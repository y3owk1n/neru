package systray

import (
	"github.com/y3owk1n/neru/internal/ports"
)

// Adapter implements ports.SystrayPort over the platform tray backends in this
// package (NSStatusItem, D-Bus StatusNotifierItem, Shell_NotifyIcon).
//
// The backends expose package-level functions and a concrete *MenuItem because
// each one wraps a single process-wide native tray. The adapter is the seam
// that turns them into an injectable contract, so the app's menu component
// depends on ports.SystrayPort rather than on this package.
type Adapter struct{}

// NewAdapter returns a tray adapter bound to this process's tray.
func NewAdapter() *Adapter {
	return &Adapter{}
}

// SetTitle sets the text shown next to the tray icon.
func (a *Adapter) SetTitle(title string) {
	setTitle(title)
}

// SetTooltip sets the icon's hover text.
func (a *Adapter) SetTooltip(tooltip string) {
	setTooltip(tooltip)
}

// SetIcon sets the tray icon, optionally as a macOS template image.
func (a *Adapter) SetIcon(icon []byte, template bool) {
	setTemplateIcon(icon, template)
}

// AddMenuItem appends a top-level item.
func (a *Adapter) AddMenuItem(title string) ports.SystrayMenuItem {
	return menuItemAdapter{item: addMenuItem(title)}
}

// AddSeparator appends a top-level divider.
func (a *Adapter) AddSeparator() {
	addSeparator()
}

// menuItemAdapter adapts the backend *MenuItem to ports.SystrayMenuItem.
//
// It is a value type wrapping a pointer, so passing it around costs nothing and
// two adapters for the same item compare equal.
type menuItemAdapter struct {
	item *menuItem
}

// Clicked returns the item's click channel.
func (m menuItemAdapter) Clicked() <-chan struct{} {
	return m.item.ClickedCh
}

// SetTitle replaces the item's label.
func (m menuItemAdapter) SetTitle(title string) { m.item.SetTitle(title) }

// Enable makes the item selectable.
func (m menuItemAdapter) Enable() { m.item.Enable() }

// Disable makes the item unselectable.
func (m menuItemAdapter) Disable() { m.item.Disable() }

// Check ticks the item's checkbox.
func (m menuItemAdapter) Check() { m.item.Check() }

// Uncheck clears the item's checkbox.
func (m menuItemAdapter) Uncheck() { m.item.Uncheck() }

// Checked reports the item's checkbox state.
func (m menuItemAdapter) Checked() bool { return m.item.Checked() }

// Show makes the item visible.
func (m menuItemAdapter) Show() { m.item.Show() }

// Hide removes the item from the menu.
func (m menuItemAdapter) Hide() { m.item.Hide() }

// AddSubMenuItem appends a child item.
func (m menuItemAdapter) AddSubMenuItem(title string) ports.SystrayMenuItem {
	return menuItemAdapter{item: m.item.AddSubMenuItem(title)}
}

// AddSeparator appends a divider inside this item's submenu.
func (m menuItemAdapter) AddSeparator() { m.item.AddSeparator() }

// Ensure the adapters satisfy the port.
var (
	_ ports.SystrayPort     = (*Adapter)(nil)
	_ ports.SystrayMenuItem = menuItemAdapter{}
)
