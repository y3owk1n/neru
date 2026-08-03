package ports

// SystrayMenuItem is one entry in the tray menu.
//
// Items are created through SystrayPort.AddMenuItem or a parent's
// AddSubMenuItem, and stay valid for the life of the tray. Every method is safe
// to call from any goroutine; implementations serialize onto whatever thread
// their platform requires (the macOS main run loop, the Win32 message pump, the
// D-Bus connection goroutine).
type SystrayMenuItem interface {
	// Clicked returns the channel that receives once each time the user
	// selects this item.
	//
	// The channel is buffered by one and never closed, so a menu handler can
	// select over many items without draining. Sends are dropped when the
	// buffer is full, which means a click that arrives while the previous one
	// is still unhandled is discarded rather than queued — correct for a menu,
	// where a double activation should not run the action twice.
	Clicked() <-chan struct{}

	// SetTitle replaces the item's label.
	SetTitle(title string)

	// Enable and Disable control whether the item is selectable.
	Enable()
	Disable()

	// Check, Uncheck and Checked control and report the checkbox state.
	Check()
	Uncheck()
	Checked() bool

	// Show and Hide control whether the item appears in the menu.
	Show()
	Hide()

	// AddSubMenuItem appends a child item and returns it.
	AddSubMenuItem(title string) SystrayMenuItem

	// AddSeparator appends a divider inside this item's submenu.
	AddSeparator()
}

// SystrayPort is the tray icon and its menu — the mechanism only; the menu's
// contents are policy in internal/app/components/systray. The run loop is not
// part of the contract: it owns the main thread on macOS and Windows, so
// starting and stopping it belongs to the daemon host in cmd/neru.
type SystrayPort interface {
	// SetTitle sets the text shown next to the tray icon. Platforms with no
	// title area ignore it.
	SetTitle(title string)

	// SetTooltip sets the icon's hover text.
	SetTooltip(tooltip string)

	// SetIcon sets the tray icon from encoded image bytes.
	//
	// template asks the platform to treat the image as a monochrome template
	// that follows the system appearance — a macOS concept. Platforms without
	// it ignore the flag, which is why Linux and Windows ship a colored asset
	// instead: a white-on-transparent template glyph is invisible there.
	SetIcon(icon []byte, template bool)

	// AddMenuItem appends a top-level item and returns it.
	AddMenuItem(title string) SystrayMenuItem

	// AddSeparator appends a top-level divider.
	AddSeparator()
}
