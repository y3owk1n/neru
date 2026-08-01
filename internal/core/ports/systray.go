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

// SystrayPort is the system tray / notification-area icon and its menu.
//
// Three unrelated native mechanisms sit behind this: NSStatusItem on macOS,
// the D-Bus StatusNotifierItem + dbusmenu protocols on Linux, and
// Shell_NotifyIcon on Windows. The menu's *contents* are application policy and
// live in internal/app/components/systray; this port is only the mechanism.
//
// The tray's run loop is not part of the contract. It owns the process's main
// thread on macOS and Windows, so starting and stopping it belongs to the
// daemon host (cmd/neru) and the build-tagged platformQuit dispatch.
//
// Availability is reported through the systray entry in PlatformCapabilities.
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
