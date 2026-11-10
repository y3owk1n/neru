//go:build linux

package linux

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/adapter/systray/icon"
)

// Linux tray API: the menu tree and item state, wired to the D-Bus SNI +
// dbusmenu server in dbus.go. The shared component
// (internal/app/components/systray) builds the menu.
//
// linuxTrayIconPNG is the tray tile until the first SetIcon, and the fallback
// for empty-byte calls.
var linuxTrayIconPNG = icon.Brand

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

	// trayPath must be the canonical /StatusNotifierItem. The KDE watcher and
	// plasmashell use that fixed path, so a custom one silently yields an
	// invisible tray item.
	trayPath = dbus.ObjectPath("/StatusNotifierItem")
	menuPath = dbus.ObjectPath("/org/y3owk1n/neru/menu")

	// quitMu guards quitCh and quitRequested. The daemon host starts app.Run
	// before the systray loop, so a fast startup failure can call Quit before
	// quitCh exists. quitRequested makes newQuitChannel return a closed channel
	// so the loop exits instead of blocking forever.
	quitMu        sync.Mutex
	quitCh        chan struct{}
	quitRequested bool
)

// MenuItem is a menu item in the system tray (Linux).
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

// SetTooltip is empty by protocol, because com.canonical.dbusmenu defines no
// per-item tooltip property (Linux). Nothing in Neru calls it.
// docs/contributing/platform-internals.md owns this as footnote 8 of the
// implementation matrix.
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

// Run starts the system tray loop (Linux). It exports the SNI and menu objects,
// runs onReadyFunc, registers with the StatusNotifierWatcher, and blocks until
// Quit. With no session bus it falls back to a headless loop.
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

	quit := newQuitChannel()

	if onReadyFunc != nil {
		onReadyFunc()
	}

	// Register with the watcher only after onReady populated IconPixmap and the
	// menu. If we register earlier, plasmashell fetches an empty icon and never
	// shows the item even after a later NewIcon signal.
	err = registerStatusNotifier(conn)
	if err != nil {
		_ = conn.Close()
		// onReady already ran, so the headless fallback must not invoke it again.
		runHeadlessLoop(nil, onExitFunc)

		return
	}

	// The host lazily fetches the menu on click, but emit a LayoutUpdated so a
	// host that cached an empty tree refreshes after onReady populated it.
	bumpMenu()

	emitSNI("NewIcon")

	<-quit

	running.Store(false)

	busConn = nil

	_ = conn.Close()

	if onExitFunc != nil {
		onExitFunc()
	}
}

// RunHeadless keeps the process alive until Quit with no status icon and no
// D-Bus registration (Linux).
func RunHeadless(onReadyFunc, onExitFunc func()) {
	runHeadlessLoop(onReadyFunc, onExitFunc)
}

// Quit quits the application (Linux). It is safe before Run/RunHeadless
// creates the quit channel, and the loop then exits as soon as it starts.
func Quit() {
	quitMu.Lock()
	defer quitMu.Unlock()

	if quitRequested {
		return
	}

	quitRequested = true

	if quitCh != nil {
		close(quitCh)
	}
}

// newQuitChannel creates the quit channel for a starting loop. If Quit already
// ran, the channel is returned pre-closed so the loop does not block.
func newQuitChannel() chan struct{} {
	quitMu.Lock()
	defer quitMu.Unlock()

	quitCh = make(chan struct{})

	if quitRequested {
		close(quitCh)
	}

	return quitCh
}

// SetTitle sets the title of the system tray icon (Linux).
func SetTitle(title string) {
	sniInst.mu.Lock()
	sniInst.title = title
	sniInst.mu.Unlock()

	emitSNI("NewTitle")
}

// SetIcon sets the tray icon from the passed PNG bytes. The shared component
// selects a host-appropriate asset, the colored brand tile on Linux, because a
// macOS template glyph would be invisible as an SNI IconPixmap. Empty bytes
// fall back to the embedded brand tile.
func SetIcon(iconBytes []byte) {
	if len(iconBytes) == 0 {
		iconBytes = linuxTrayIconPNG
	}

	setTrayIcon(iconBytes)
}

// SetTemplateIcon sets the tray icon from the passed PNG bytes. SNI has no
// template rendering, so the template flag is ignored.
func SetTemplateIcon(iconBytes []byte, template bool) {
	SetIcon(iconBytes)
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

	quitMu.Lock()
	quitCh = nil
	quitRequested = false
	quitMu.Unlock()
}

// runHeadlessLoop invokes onReady, blocks until Quit, then invokes onExit.
func runHeadlessLoop(onReadyFunc, onExitFunc func()) {
	running.Store(false)

	quit := newQuitChannel()

	if onReadyFunc != nil {
		onReadyFunc()
	}

	<-quit

	if onExitFunc != nil {
		onExitFunc()
	}
}

// bumpMenu increments the menu revision and emits LayoutUpdated so the host
// re-fetches the tree. Before the tray runs, the state is still recorded.
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

// exportTray exports the SNI and menu objects and requests the well-known
// item name. Watcher registration waits for onReady (see Run).
func exportTray(conn *dbus.Conn) error {
	sniInst.menuPath = menuPath

	err := conn.Export(sniInst, trayPath, "org.kde.StatusNotifierItem")
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

	return nil
}

// registerStatusNotifier tells StatusNotifierWatcher about this item so
// plasmashell (or another host) begins displaying it.
func registerStatusNotifier(conn *dbus.Conn) error {
	name := fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", os.Getpid())

	obj := conn.Object("org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher")

	err := obj.Call(
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

	for _, p := range []string{"Version", propStatus, "TextDirection"} {
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
