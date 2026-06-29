//go:build linux

// internal/core/infra/systray/systray_linux.go
// Linux tray API: maintains the menu tree and wires it to the D-Bus SNI +
// dbusmenu server in systray_linux_dbus.go. The menu is built by the
// platform-agnostic app component (internal/app/components/systray); this file
// only owns the transport and the item state.
// Does not implement the darwin/Windows tray; those have their own backends.

package systray

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/godbus/dbus/v5"
)

var (
	menuItemsLock sync.RWMutex
	menuItems     = make(map[int]*MenuItem)
	nextID        = 1
	rootChildren  []int

	// busConn is the session-bus connection used while the tray is running. It
	// is nil before Run or after Quit, so API methods must guard emits on
	// running.Load().
	busConn *dbus.Conn

	running atomic.Bool

	sniInst = &sniServer{
		id:       "neru",
		category: "ApplicationStatus",
		status:   "Active",
		title:    "Neru",
	}
	menuInst = &menuServer{}

	trayPath = dbus.ObjectPath("/org/y3owk1n/neru/sni")
	menuPath = dbus.ObjectPath("/org/y3owk1n/neru/menu")

	quitCh   chan struct{}
	quitOnce sync.Once
)

// MenuItem represents a menu item in the system tray (Linux).
type MenuItem struct {
	ClickedCh chan struct{}
	id        int
	parentID  int
	isSep     bool
	children  []int

	mu       sync.RWMutex
	title    string
	disabled bool
	checked  bool
	hidden   bool
	icon     []byte
}

// Title returns the menu item title (Linux).
func (m *MenuItem) Title() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.title
}

// Disabled returns whether the menu item is disabled (Linux).
func (m *MenuItem) Disabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.disabled
}

// Checked returns whether the menu item is checked (Linux).
func (m *MenuItem) Checked() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.checked
}

// Hidden returns whether the menu item is hidden (Linux).
func (m *MenuItem) Hidden() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.hidden
}

// SetTitle sets the menu item title (Linux).
func (m *MenuItem) SetTitle(title string) {
	m.mu.Lock()
	m.title = title
	m.mu.Unlock()

	bumpMenu()
}

// SetTooltip sets the menu item tooltip (Linux, no-op: dbusmenu has no per-item
// tooltip property).
func (m *MenuItem) SetTooltip(tooltip string) {}

// SetIcon sets the menu item icon (Linux).
func (m *MenuItem) SetIcon(icon []byte) {
	m.mu.Lock()
	m.icon = icon
	m.mu.Unlock()

	bumpMenu()
}

// Enable enables the menu item (Linux).
func (m *MenuItem) Enable() {
	m.mu.Lock()
	m.disabled = false
	m.mu.Unlock()

	bumpMenu()
}

// Disable disables the menu item (Linux).
func (m *MenuItem) Disable() {
	m.mu.Lock()
	m.disabled = true
	m.mu.Unlock()

	bumpMenu()
}

// Check checks the menu item (Linux).
func (m *MenuItem) Check() {
	m.mu.Lock()
	m.checked = true
	m.mu.Unlock()

	bumpMenu()
}

// Uncheck unchecks the menu item (Linux).
func (m *MenuItem) Uncheck() {
	m.mu.Lock()
	m.checked = false
	m.mu.Unlock()

	bumpMenu()
}

// Show shows the menu item (Linux).
func (m *MenuItem) Show() {
	m.mu.Lock()
	m.hidden = false
	m.mu.Unlock()

	bumpMenu()
}

// Hide hides the menu item (Linux).
func (m *MenuItem) Hide() {
	m.mu.Lock()
	m.hidden = true
	m.mu.Unlock()

	bumpMenu()
}

// AddSubMenuItem adds a sub menu item to the menu item (Linux).
func (m *MenuItem) AddSubMenuItem(title string) *MenuItem {
	item := &MenuItem{
		ClickedCh: make(chan struct{}, 1),
		title:     title,
		parentID:  m.id,
	}

	menuItemsLock.Lock()
	item.id = registerMenuItemLocked(item)
	m.children = append(m.children, item.id)
	menuItemsLock.Unlock()

	bumpMenu()

	return item
}

// AddSeparator adds a separator to the menu item (Linux).
func (m *MenuItem) AddSeparator() {
	sep := &MenuItem{
		isSep:    true,
		parentID: m.id,
	}

	menuItemsLock.Lock()
	sep.id = registerMenuItemLocked(sep)
	m.children = append(m.children, sep.id)
	menuItemsLock.Unlock()

	bumpMenu()
}

// props returns the com.canonical.dbusmenu property dict for this item. A nil
// filter returns all properties the host needs to render the row.
func (m *MenuItem) props(filter []string) map[string]dbus.Variant {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := map[string]dbus.Variant{
		"visible": dbus.MakeVariant(!m.hidden),
	}

	if m.isSep {
		all["type"] = dbus.MakeVariant("separator")

		return filterProps(all, filter)
	}

	all["type"] = dbus.MakeVariant("standard")
	all["label"] = dbus.MakeVariant(m.title)
	all["enabled"] = dbus.MakeVariant(!m.disabled)

	if len(m.children) > 0 {
		all["children-display"] = dbus.MakeVariant("submenu")
	}

	if m.checked {
		all["toggle-type"] = dbus.MakeVariant("checkmark")
		all["toggle-state"] = dbus.MakeVariant(int32(1))
	}

	if len(m.icon) > 0 {
		all["icon-data"] = dbus.MakeVariant(pngToPixmaps(m.icon))
	}

	return filterProps(all, filter)
}

// filterProps keeps only the requested keys (or all when filter is empty).
func filterProps(all map[string]dbus.Variant, filter []string) map[string]dbus.Variant {
	if len(filter) == 0 {
		return all
	}

	out := make(map[string]dbus.Variant, len(filter))
	for _, k := range filter {
		if v, ok := all[k]; ok {
			out[k] = v
		}
	}

	return out
}

// Run starts the system tray loop (Linux). It connects to the session bus,
// exports the SNI + menu objects, registers with the StatusNotifierWatcher,
// then invokes onReadyFunc (which builds the menu) and blocks until Quit. If no
// session bus is available it falls back to a headless blocking loop so the
// daemon still runs.
func Run(onReadyFunc, onExitFunc func()) {
	conn, err := dbus.SessionBus()
	if err != nil {
		runHeadlessLoop(onReadyFunc, onExitFunc)

		return
	}

	err = exportTray(conn)
	if err != nil {
		_ = conn.Close()

		runHeadlessLoop(onReadyFunc, onExitFunc)

		return
	}

	busConn = conn

	running.Store(true)

	quitCh = make(chan struct{})

	if onReadyFunc != nil {
		onReadyFunc()
	}

	// The host lazily fetches the menu on click, but emit a LayoutUpdated so a
	// host that cached an empty tree refreshes after onReady populated it.
	bumpMenu()

	<-quitCh

	running.Store(false)

	busConn = nil

	_ = conn.Close()

	if onExitFunc != nil {
		onExitFunc()
	}
}

// RunHeadless starts the system tray loop without a status icon (Linux). Used
// when there is no systray component: no D-Bus registration, just keep the
// process alive until Quit.
func RunHeadless(onReadyFunc, onExitFunc func()) {
	runHeadlessLoop(onReadyFunc, onExitFunc)
}

// Quit quits the application (Linux).
func Quit() {
	quitOnce.Do(func() {
		if quitCh != nil {
			close(quitCh)
		}
	})
}

// SetTitle sets the title of the system tray icon (Linux).
func SetTitle(title string) {
	sniInst.mu.Lock()
	sniInst.title = title
	sniInst.mu.Unlock()

	emitSNI("NewTitle")
}

// SetTooltip sets the tooltip of the system tray icon (Linux).
func SetTooltip(tooltip string) {
	sniInst.mu.Lock()
	sniInst.tip = toolTip{Title: tooltip}
	sniInst.mu.Unlock()

	emitSNI("NewToolTip")
}

// SetIcon sets the icon of the system tray icon (Linux).
func SetIcon(icon []byte) {
	setTrayIcon(icon)
}

// SetTemplateIcon sets the icon of the system tray icon as a template icon
// (Linux). Linux has no template-icon concept; the bytes are rendered as-is.
func SetTemplateIcon(icon []byte, template bool) {
	setTrayIcon(icon)
}

// AddMenuItem adds a menu item to the system tray (Linux).
func AddMenuItem(title string) *MenuItem {
	item := &MenuItem{
		ClickedCh: make(chan struct{}, 1),
		title:     title,
		parentID:  0,
	}

	menuItemsLock.Lock()
	item.id = registerMenuItemLocked(item)
	rootChildren = append(rootChildren, item.id)
	menuItemsLock.Unlock()

	bumpMenu()

	return item
}

// AddSeparator adds a separator to the system tray (Linux).
func AddSeparator() {
	sep := &MenuItem{
		isSep:    true,
		parentID: 0,
	}

	menuItemsLock.Lock()
	sep.id = registerMenuItemLocked(sep)
	rootChildren = append(rootChildren, sep.id)
	menuItemsLock.Unlock()

	bumpMenu()
}

// setTrayIcon converts the PNG bytes to an SNI pixmap, stores it on the SNI
// server, and notifies the host.
func setTrayIcon(icon []byte) {
	pix := pngToPixmaps(icon)

	sniInst.mu.Lock()
	sniInst.iconPix = pix
	sniInst.mu.Unlock()

	emitSNI("NewIcon")
}

// registerMenuItemLocked assigns the next id and stores the item. The caller
// must hold menuItemsLock.
func registerMenuItemLocked(item *MenuItem) int {
	id := nextID
	nextID++
	menuItems[id] = item

	return id
}

// ResetForTesting resets all global state (Linux).
func ResetForTesting() {
	menuItemsLock.Lock()
	menuItems = make(map[int]*MenuItem)
	nextID = 1
	rootChildren = nil
	menuItemsLock.Unlock()

	sniInst.mu.Lock()
	sniInst.title = "Neru"
	sniInst.iconPix = nil
	sniInst.tip = toolTip{}
	sniInst.status = "Active"
	sniInst.mu.Unlock()

	menuInst.mu.Lock()
	menuInst.revision = 0
	menuInst.mu.Unlock()

	quitOnce = sync.Once{}
}

// runHeadlessLoop invokes onReady then blocks until Quit, then invokes onExit.
// It is the fallback when no session bus is available and the implementation of
// RunHeadless.
func runHeadlessLoop(onReadyFunc, onExitFunc func()) {
	running.Store(false)

	quitCh = make(chan struct{})

	if onReadyFunc != nil {
		onReadyFunc()
	}

	<-quitCh

	if onExitFunc != nil {
		onExitFunc()
	}
}

// bumpMenu increments the menu revision and emits LayoutUpdated so the tray
// host re-fetches the tree. No-op when the tray is not running yet (state is
// still recorded; the host reads it on first open).
func bumpMenu() {
	menuInst.mu.Lock()
	menuInst.revision++
	rev := menuInst.revision
	menuInst.mu.Unlock()

	if !running.Load() || busConn == nil {
		return
	}

	_ = busConn.Emit(menuPath, "com.canonical.dbusmenu.LayoutUpdated", rev, int32(-1))
}

// emitSNI emits a StatusNotifierItem signal on the tray object path. No-op when
// the tray is not running.
func emitSNI(signal string) {
	if !running.Load() || busConn == nil {
		return
	}

	_ = busConn.Emit(trayPath, "org.kde.StatusNotifierItem."+signal)
}

// exportTray exports the SNI and menu objects on the connection, requests the
// well-known item name, and registers it with the StatusNotifierWatcher. It
// also wires the menu Properties interface (Version/Status/TextDirection).
func exportTray(conn *dbus.Conn) error {
	sniInst.menuPath = menuPath

	var err error

	err = conn.Export(sniInst, trayPath, "org.kde.StatusNotifierItem")
	if err != nil {
		return fmt.Errorf("export SNI: %w", err)
	}

	err = conn.Export(sniInst, trayPath, "org.freedesktop.DBus.Properties")
	if err != nil {
		return fmt.Errorf("export SNI properties: %w", err)
	}

	err = conn.Export(menuInst, menuPath, "com.canonical.dbusmenu")
	if err != nil {
		return fmt.Errorf("export menu: %w", err)
	}

	err = conn.Export(menuPropsServer{}, menuPath, "org.freedesktop.DBus.Properties")
	if err != nil {
		return fmt.Errorf("export menu properties: %w", err)
	}

	name := fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", os.Getpid())

	reply, err := conn.RequestName(name, dbus.NameFlagDoNotQueue)
	if err != nil {
		return fmt.Errorf("request name: %w", err)
	}

	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("%s: reply %d: %w", name, reply, errCannotOwnSNIName)
	}

	obj := conn.Object("org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher")

	err = obj.Call(
		"org.kde.StatusNotifierWatcher.RegisterStatusNotifierItem",
		0,
		name,
	).Err
	if err != nil {
		return fmt.Errorf("register with watcher: %w", err)
	}

	return nil
}

// menuPropsServer implements org.freedesktop.DBus.Properties for the menu object
// (com.canonical.dbusmenu has Version/Status/TextDirection properties).
type menuPropsServer struct{}

func (menuPropsServer) Get(iface, prop string) (dbus.Variant, *dbus.Error) {
	val, err := getMenuProperty(iface, prop)
	if err != nil {
		return dbus.Variant{}, dbus.NewError(
			"org.freedesktop.DBus.Properties.Error.UnknownProperty",
			[]any{err.Error()},
		)
	}

	return val, nil
}

func (menuPropsServer) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	out := map[string]dbus.Variant{}

	for _, p := range []string{"Version", "Status", "TextDirection"} {
		v, err := getMenuProperty(iface, p)
		if err == nil {
			out[p] = v
		}
	}

	return out, nil
}

func (menuPropsServer) Set(iface, prop string, val dbus.Variant) *dbus.Error {
	return dbus.NewError(
		"org.freedesktop.DBus.Properties.Error.ReadOnly",
		[]any{"menu properties are read-only"},
	)
}
