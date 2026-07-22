//go:build linux

// internal/core/infra/accessibility/atspi_linux.go
// AT-SPI (D-Bus) accessibility client for Linux: enables assistive-tech mode,
// finds the active window, and walks its tree for clickable elements so hints
// mode works on KDE/Wayland and other AT-SPI desktops.
// It does NOT implement input injection (that stays on the embedded
// InfraAXClient, which routes clicks through wlroots/libei).

package accessibility

import (
	"context"
	"image"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

const (
	a11yBusDest   = "org.a11y.Bus"
	a11yBusPath   = "/org/a11y/bus"
	a11yStatusIfc = "org.a11y.Status"

	atspiRegistryDest = "org.a11y.atspi.Registry"
	atspiRootPath     = dbus.ObjectPath("/org/a11y/atspi/accessible/root")

	atspiAccessibleIfc = "org.a11y.atspi.Accessible"
	atspiComponentIfc  = "org.a11y.atspi.Component"

	// ATSPI_COORD_TYPE_SCREEN: extents relative to the screen origin.
	atspiCoordScreen = uint32(0)

	// AT-SPI state bit indices (atspi-constants.h). All < 32, so they live in
	// the first uint32 of the state bitfield array.
	atspiStateActive  = 1
	atspiStateShowing = 25

	// AT-SPI packs state into an array of uint32 words (one bit per state).
	atspiStateBitsPerWord = 32

	// Walk bounds to keep D-Bus traffic sane on deep trees.
	atspiMaxDepth = 40
	atspiMaxNodes = 2000

	// Capacity hint for the clickable-node slice: most windows fit well under
	// this and the slice grows if a dense tree exceeds it.
	atspiClickableNodesCap = 128

	// org.a11y.Status D-Bus property names.
	a11yPropIsEnabled    = "IsEnabled"
	a11yPropScreenReader = "ScreenReaderEnabled"
)

// accRef is the AT-SPI (bus-name, object-path) reference returned by
// GetChildren and the registry root.
type accRef struct {
	Name string
	Path dbus.ObjectPath
}

// atspiExtents mirrors the (iiii) struct returned by Component.GetExtents.
type atspiExtents struct {
	X int32
	Y int32
	W int32
	H int32
}

// AX role names that appear in more than one place in the role maps below.
const (
	axRoleButton    = "AXButton"
	axRoleMenuItem  = "AXMenuItem"
	axRoleTextField = "AXTextField"
	axRoleRow       = "AXRow"
)

// atspiToAXRole maps AT-SPI role names (lowercase, as returned by
// Accessible.GetRoleName) to the macOS-style "AX*" role names that Neru's config
// and the cross-platform filter pipeline speak. Neru's clickable_roles config is
// authored in AX vocabulary (AXButton, AXLink, ...) and Adapter.MatchesFilter
// re-checks elem.Role() against those same AX names, so the AT-SPI client must
// emit AX role names for any of this to match. Roles with no clickable AX
// equivalent are intentionally absent so containers (section, heading, label)
// are skipped.
var atspiToAXRole = map[string]string{
	"push button":     axRoleButton,
	"button":          axRoleButton,
	"toggle button":   axRoleButton,
	"menu button":     "AXMenuButton",
	"combo box":       "AXComboBox",
	"check box":       "AXCheckBox",
	"check menu item": axRoleMenuItem,
	"radio button":    "AXRadioButton",
	"radio menu item": axRoleMenuItem,
	"link":            "AXLink",
	"entry":           axRoleTextField,
	"password text":   axRoleTextField,
	"slider":          "AXSlider",
	"page tab":        "AXTabButton",
	"menu item":       axRoleMenuItem,
	"list item":       axRoleRow,
	"table cell":      "AXCell",
	"table row":       axRoleRow,
}

// defaultClickableAXRoles is used when the caller passes no explicit role
// filter. It mirrors the AX names in the shipped default config.
var defaultClickableAXRoles = map[string]struct{}{
	axRoleButton:    {},
	"AXMenuButton":  {},
	"AXComboBox":    {},
	"AXCheckBox":    {},
	"AXRadioButton": {},
	"AXLink":        {},
	"AXPopUpButton": {},
	axRoleTextField: {},
	"AXSlider":      {},
	"AXTabButton":   {},
	"AXSwitch":      {},
	"AXTextArea":    {},
	axRoleMenuItem:  {},
	"AXCell":        {},
	axRoleRow:       {},
}

// ATSPIClient is the Linux AXClient. It walks the AT-SPI tree for hints and
// delegates everything else (input injection, focused-app identity) to the
// embedded InfraAXClient.
type ATSPIClient struct {
	*InfraAXClient

	logger *zap.Logger
	kwin   *kwinBridge

	mu        sync.Mutex
	a11y      *dbus.Conn
	a11yReady bool

	// a11y state management.
	a11yMu    sync.Mutex
	activated bool   // true once we enabled AT-SPI this session
	savedIsOn bool   // original IsEnabled before our first enable
	savedSrOn bool   // original ScreenReaderEnabled before our first enable
}

// NewATSPIClient builds the Linux accessibility client and best-effort enables
// assistive-tech mode so apps start exposing their trees before the first
// hints request. When hints are disabled, accessibility is not activated
// to avoid triggering unnecessary screen-reader prompts in applications.
func NewATSPIClient(
	logger *zap.Logger,
	configProvider config.Provider,
	hintsEnabled bool,
) *ATSPIClient {
	if logger == nil {
		logger = zap.NewNop()
	}

	client := &ATSPIClient{
		InfraAXClient: NewInfraAXClient(logger, configProvider),
		logger:        logger.Named("accessibility.atspi"),
		kwin:          newKWinBridge(logger),
	}

	// Always reset a11y status on startup to clear any state left by a previous
	// run. If hints are enabled, re-enable after the reset.
	go func() {
		_ = client.disableAccessibility()

		if hintsEnabled {
			err := client.enableAccessibility()
			if err != nil {
				client.logger.Warn("Failed to enable AT-SPI accessibility status", zap.Error(err))
			}

			// Install the KWin geometry bridge so AT-SPI window-relative
			// coordinates can be offset into screen coordinates.
			go client.kwin.start()
		}
	}()

	return client
}

// FrontmostWindow returns the active top-level window via AT-SPI.
func (c *ATSPIClient) FrontmostWindow(_ context.Context) (AXWindow, error) {
	conn, err := c.ensureA11yConn()
	if err != nil {
		return nil, err
	}

	frame, ok := c.findActiveFrame(conn)
	if !ok {
		c.logger.Debug("AT-SPI: no active frame found")

		// No active frame: hand back an empty window so the adapter simply
		// finds no clickable elements rather than erroring out.
		return &atspiWindow{}, nil
	}

	c.logger.Debug("AT-SPI: selected active frame",
		zap.String("bus", frame.Name),
		zap.String("app", c.name(conn, accRef{Name: frame.Name, Path: atspiRootPath})),
		zap.String("frameTitle", c.name(conn, frame)))

	return &atspiWindow{ref: frame, valid: true}, nil
}

// FrontmostAndPopoverWindows returns the active window (popovers are part of
// the same AT-SPI tree, so the single-window walk already covers them).
func (c *ATSPIClient) FrontmostAndPopoverWindows(ctx context.Context) ([]AXWindow, error) {
	win, err := c.FrontmostWindow(ctx)
	if err != nil {
		return nil, err
	}

	w, ok := win.(*atspiWindow)
	if !ok || !w.valid {
		return nil, nil
	}

	return []AXWindow{win}, nil
}

// AllWindows is not used by hints; return empty rather than erroring.
func (c *ATSPIClient) AllWindows(_ context.Context) ([]AXWindow, error) {
	return nil, nil
}

// ClickableNodes walks the given window for clickable elements.
func (c *ATSPIClient) ClickableNodes(
	ctx context.Context,
	root AXElement,
	roles []string,
	_ int,
) ([]AXNode, error) {
	win, ok := root.(*atspiWindow)
	if !ok || !win.valid {
		return nil, nil
	}

	conn, err := c.ensureA11yConn()
	if err != nil {
		return nil, err
	}

	start := time.Now()

	// Validate the cached KWin origin against the frame actually being walked
	// (by size): a stale origin from a previous window would offset every hint
	// to the wrong screen position. When the frame extents are unavailable the
	// origin cannot be validated, so no offset is applied at all — unoffset
	// hints beat hints offset to a previous window or monitor.
	var (
		offX, offY int
		haveOrigin bool
	)

	frameRect, frameOK := c.extents(conn, win.ref)
	if frameOK {
		offX, offY, haveOrigin = c.kwin.originFor(frameRect.Dx(), frameRect.Dy())
	}

	out := make([]AXNode, 0, atspiClickableNodesCap)
	visited := 0
	c.walk(ctx, conn, win.ref, rolesSet(roles), 0, &out, &visited, offX, offY)

	c.logger.Debug("AT-SPI clickable walk complete",
		zap.Int("count", len(out)),
		zap.Int("visited", visited),
		zap.Int("offsetX", offX),
		zap.Int("offsetY", offY),
		zap.Bool("haveOrigin", haveOrigin),
		zap.Duration("elapsed", time.Since(start)))

	return out, nil
}

// Close restores the org.a11y.Status to the values that were active before
// our first enable, and releases the dedicated D-Bus connection.
func (c *ATSPIClient) Close() error {
	c.a11yMu.Lock()
	wasActivated := c.activated
	restoreIsOn := c.savedIsOn
	restoreSrOn := c.savedSrOn
	c.a11yMu.Unlock()

	if wasActivated {
		err := c.setA11yStatus(restoreSrOn, restoreIsOn)
		if err != nil {
			c.logger.Warn("Failed to restore AT-SPI a11y status", zap.Error(err))
		}
	}

	c.mu.Lock()
	if c.a11y != nil {
		closeErr := c.a11y.Close()
		if closeErr != nil {
			c.logger.Warn("Failed to close AT-SPI D-Bus connection", zap.Error(closeErr))
		}
		c.a11y = nil
		c.a11yReady = false
	}
	c.mu.Unlock()

	return nil
}

// readA11yStatus reads the current IsEnabled and ScreenReaderEnabled D-Bus
// properties from org.a11y.Status.
func (c *ATSPIClient) readA11yStatus() (bool, bool, error) {
	conn, connErr := dbus.SessionBus()
	if connErr != nil {
		return false, false, connErr
	}
	obj := conn.Object(a11yBusDest, a11yBusPath)

	var isVariant dbus.Variant
	getErr := obj.Call("org.freedesktop.DBus.Properties.Get", 0,
		a11yStatusIfc, a11yPropIsEnabled).Store(&isVariant)
	if getErr != nil {
		return false, false, getErr
	}
	isOn, _ := isVariant.Value().(bool)

	var srVariant dbus.Variant
	getErr = obj.Call("org.freedesktop.DBus.Properties.Get", 0,
		a11yStatusIfc, a11yPropScreenReader).Store(&srVariant)
	if getErr != nil {
		return false, false, getErr
	}
	srOn, _ := srVariant.Value().(bool)

	return isOn, srOn, nil
}

// setA11yStatus writes the given values for IsEnabled and ScreenReaderEnabled
// on org.a11y.Status. When disabling, ScreenReaderEnabled is cleared before
// IsEnabled; when enabling, IsEnabled is set first.
func (c *ATSPIClient) setA11yStatus(srEnabled, isEnabled bool) error {
	conn, connErr := dbus.SessionBus()
	if connErr != nil {
		return connErr
	}
	obj := conn.Object(a11yBusDest, a11yBusPath)

	var props []string
	if !srEnabled {
		props = []string{a11yPropScreenReader, a11yPropIsEnabled}
	} else {
		props = []string{a11yPropIsEnabled, a11yPropScreenReader}
	}

	for _, prop := range props {
		var propVal bool
		switch prop {
		case a11yPropIsEnabled:
			propVal = isEnabled
		case a11yPropScreenReader:
			propVal = srEnabled
		}
		err := obj.Call("org.freedesktop.DBus.Properties.Set", 0,
			a11yStatusIfc, prop, dbus.MakeVariant(propVal)).Err
		if err != nil {
			return err
		}
	}

	c.logger.Debug("AT-SPI status set",
		zap.Bool(a11yPropIsEnabled, isEnabled),
		zap.Bool(a11yPropScreenReader, srEnabled))

	return nil
}

// enableAccessibility flips org.a11y.Status so toolkits expose their trees.
func (c *ATSPIClient) enableAccessibility() error {
	obj, connErr := c.getA11yStatusObj()
	if connErr != nil {
		return connErr
	}

	t := true
	for _, prop := range []string{"IsEnabled", "ScreenReaderEnabled"} {
		callErr := obj.Call(
			"org.freedesktop.DBus.Properties.Set", 0,
			a11yStatusIfc, prop, dbus.MakeVariant(t),
		).Err
		if callErr != nil {
			return callErr
		}
	}

	c.logger.Debug("AT-SPI accessibility status enabled")

	return nil
}

// disableAccessibility clears org.a11y.Status so toolkits stop exposing their
// accessibility trees and apps no longer detect a screen reader.
func (c *ATSPIClient) disableAccessibility() error {
	obj, connErr := c.getA11yStatusObj()
	if connErr != nil {
		return connErr
	}

	f := false
	for _, prop := range []string{"ScreenReaderEnabled", "IsEnabled"} {
		callErr := obj.Call(
			"org.freedesktop.DBus.Properties.Set", 0,
			a11yStatusIfc, prop, dbus.MakeVariant(f),
		).Err
		if callErr != nil {
			return callErr
		}
	}

	c.logger.Debug("AT-SPI accessibility status disabled")

	return nil
}

// getA11yStatusObj returns a D-Bus object for the org.a11y.Status interface.
func (c *ATSPIClient) getA11yStatusObj() (dbus.BusObject, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, err
	}

	return conn.Object(a11yBusDest, a11yBusPath), nil
}

// ensureA11yConn lazily connects to the dedicated AT-SPI bus.
func (c *ATSPIClient) ensureA11yConn() (*dbus.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.a11yReady && c.a11y != nil && c.a11y.Connected() {
		return c.a11y, nil
	}

	session, err := dbus.SessionBus()
	if err != nil {
		return nil, err
	}

	var addr string

	getAddrErr := session.Object(a11yBusDest, a11yBusPath).
		Call("org.a11y.Bus.GetAddress", 0).Store(&addr)
	if getAddrErr != nil {
		return nil, getAddrErr
	}

	conn, connErr := dbus.Connect(addr)
	if connErr != nil {
		return nil, connErr
	}

	c.a11y = conn
	c.a11yReady = true

	return conn, nil
}

// children returns the AT-SPI children of an accessible. It prefers the bulk
// GetChildren method and falls back to ChildCount + GetChildAtIndex for older
// toolkits that do not expose GetChildren.
func (c *ATSPIClient) children(conn *dbus.Conn, ref accRef) []accRef {
	obj := conn.Object(ref.Name, ref.Path)

	var kids []accRef

	err := obj.Call(atspiAccessibleIfc+".GetChildren", 0).Store(&kids)
	if err == nil {
		return kids
	}

	countVar, propErr := obj.GetProperty(atspiAccessibleIfc + ".ChildCount")
	if propErr != nil {
		return nil
	}

	count, _ := countVar.Value().(int32)
	for i := range count {
		var child accRef

		err := obj.Call(atspiAccessibleIfc+".GetChildAtIndex", 0, i).Store(&child)
		if err != nil {
			continue
		}

		kids = append(kids, child)
	}

	return kids
}

// roleName returns the AT-SPI localized-independent role name (e.g. "push button").
func (c *ATSPIClient) roleName(conn *dbus.Conn, ref accRef) string {
	var name string

	err := conn.Object(ref.Name, ref.Path).
		Call(atspiAccessibleIfc+".GetRoleName", 0).Store(&name)
	if err != nil {
		return ""
	}

	return name
}

// name returns the accessible Name property (used as the element title).
func (c *ATSPIClient) name(conn *dbus.Conn, ref accRef) string {
	val, err := conn.Object(ref.Name, ref.Path).
		GetProperty(atspiAccessibleIfc + ".Name")
	if err != nil {
		return ""
	}

	s, _ := val.Value().(string)

	return s
}

// stateHas reports whether the accessible has the given AT-SPI state bit set.
func (c *ATSPIClient) stateHas(conn *dbus.Conn, ref accRef, bit uint) bool {
	var states []uint32

	err := conn.Object(ref.Name, ref.Path).
		Call(atspiAccessibleIfc+".GetState", 0).Store(&states)
	if err != nil || len(states) == 0 {
		return false
	}

	word := bit / atspiStateBitsPerWord
	if int(word) >= len(states) {
		return false
	}

	return states[word]&(1<<(bit%atspiStateBitsPerWord)) != 0
}

// extents returns the on-screen rectangle of an accessible.
func (c *ATSPIClient) extents(conn *dbus.Conn, ref accRef) (image.Rectangle, bool) {
	var ext atspiExtents

	err := conn.Object(ref.Name, ref.Path).
		Call(atspiComponentIfc+".GetExtents", 0, atspiCoordScreen).Store(&ext)
	if err != nil {
		return image.Rectangle{}, false
	}

	if ext.W <= 0 || ext.H <= 0 {
		return image.Rectangle{}, false
	}

	return image.Rect(int(ext.X), int(ext.Y), int(ext.X+ext.W), int(ext.Y+ext.H)), true
}

// isVirtualKeyboardApp reports whether an AT-SPI application is an on-screen
// virtual keyboard, which must never be treated as the focused window.
func isVirtualKeyboardApp(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "plasma-keyboard", "maliit-keyboard", "maliit-server", "squeekboard":
		return true
	default:
		return false
	}
}

// isDesktopShellApp reports whether an AT-SPI application is the KDE desktop
// shell (plasmashell: the panel, taskbar, widgets and desktop background).
// KWin marks plasmashell ACTIVE the instant the pointer moves over the desktop
// — which happens immediately after a hint selection moves the cursor — so it
// would otherwise hijack the active frame on re-activation and yield no app
// hints. It is deprioritised to a last resort rather than skipped entirely so
// its own UI (panel/widgets) can still be hinted when the desktop is genuinely
// the focused surface.
func isDesktopShellApp(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "plasmashell", "org.kde.plasmashell":
		return true
	default:
		return false
	}
}

// isNonTargetSurfaceApp reports whether an AT-SPI application is a system
// surface that is never a valid hint target and must never be picked as the
// focused window — even as a last resort. The XWayland video bridge
// ("xwaylandvideobridge") and the KDE portal consent dialog briefly steal the
// ACTIVE state the moment we inject a cursor move via libei, which on
// re-activation makes findActiveFrame select an empty surface and tears the
// hints overlay down. This mirrors the KWin geometry bridge blocklist in
// kwin_geometry_linux.go so both code paths ignore the same noise.
func isNonTargetSurfaceApp(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "xwaylandvideobridge",
		"org.freedesktop.impl.portal.desktop.kde":
		return true
	default:
		return false
	}
}

// findActiveFrame locates the focused top-level window across all registered
// applications by looking for a frame with the ACTIVE state.
// On KDE several frames can report the ACTIVE state at once (e.g. the maliit
// virtual keyboard "plasma-keyboard" is persistently active but off-screen).
// The genuinely focused window is the one that is ACTIVE *and* SHOWING, so we
// prefer that and fall back to any active, then any showing, frame.
func (c *ATSPIClient) findActiveFrame(conn *dbus.Conn) (accRef, bool) {
	root := accRef{Name: atspiRegistryDest, Path: atspiRootPath}

	var (
		activeShowing accRef
		haveAS        bool
		activeAny     accRef
		haveAA        bool
		showingAny    accRef
		haveSA        bool
		// Desktop-shell (plasmashell) frames are only used as a last resort —
		// see isDesktopShellApp — so a desktop that goes ACTIVE right after a
		// cursor move cannot hijack a still-showing real application window.
		shellShowing accRef
		haveShell    bool
	)

	for _, app := range c.children(conn, root) {
		appName := c.name(conn, app)

		// Skip surfaces that are never valid hint targets and that steal the
		// ACTIVE state right after a libei cursor move: on-screen virtual
		// keyboards (e.g. the maliit "plasma-keyboard"), the XWayland video
		// bridge, and the portal consent dialog. Being iterated before the real
		// app, any of these would otherwise be picked as the focused window and
		// kill the overlay on re-activation.
		if isVirtualKeyboardApp(appName) || isNonTargetSurfaceApp(appName) {
			continue
		}

		isShell := isDesktopShellApp(appName)

		for _, frame := range c.children(conn, app) {
			role := c.roleName(conn, frame)
			if role != "frame" && role != "window" && role != "dialog" {
				continue
			}

			active := c.stateHas(conn, frame, atspiStateActive)
			showing := c.stateHas(conn, frame, atspiStateShowing)

			// The desktop shell never wins the active-frame race; it is kept
			// aside and only used if no real application frame is found.
			if isShell {
				if showing && !haveShell {
					shellShowing = frame
					haveShell = true
				}

				continue
			}

			if active && showing && !haveAS {
				activeShowing = frame
				haveAS = true
			}

			if active && !haveAA {
				activeAny = frame
				haveAA = true
			}

			if showing && !haveSA {
				showingAny = frame
				haveSA = true
			}
		}
	}

	switch {
	case haveAS:
		return activeShowing, true
	case haveAA:
		return activeAny, true
	case haveSA:
		return showingAny, true
	case haveShell:
		return shellShowing, true
	default:
		return accRef{}, false
	}
}

// walk recursively collects clickable, showing nodes under ref.
func (c *ATSPIClient) walk(
	ctx context.Context,
	conn *dbus.Conn,
	ref accRef,
	roles map[string]struct{},
	depth int,
	out *[]AXNode,
	visited *int,
	offX int,
	offY int,
) {
	if depth > atspiMaxDepth || len(*out) >= atspiMaxNodes {
		return
	}

	if ctx.Err() != nil {
		return
	}

	*visited++

	// Translate the AT-SPI role into Neru's AX vocabulary, then match against
	// the requested role set (also AX names). This keeps the whole pipeline,
	// including the downstream Adapter.MatchesFilter, speaking one vocabulary.
	axRole, mappable := atspiToAXRole[strings.ToLower(c.roleName(conn, ref))]

	if mappable {
		if _, ok := roles[axRole]; ok && c.stateHas(conn, ref, atspiStateShowing) {
			if rect, valid := c.extents(conn, ref); valid {
				// AT-SPI reports window-relative coords on Wayland; offset by the
				// focused window's screen origin from the KWin bridge.
				rect = rect.Add(image.Pt(offX, offY))
				*out = append(*out, &atspiNode{
					id:    string(ref.Path) + "@" + ref.Name,
					role:  axRole,
					title: c.name(conn, ref),
					rect:  rect,
				})
			}
		}
	}

	for _, child := range c.children(conn, ref) {
		c.walk(ctx, conn, child, roles, depth+1, out, visited, offX, offY)
	}
}

// rolesSet converts the caller's AX role list into a lookup set, falling back
// to the default clickable AX role set when empty. AX names are case-sensitive
// (e.g. "AXButton") and must match the config and Adapter.MatchesFilter exactly,
// so they are NOT lowercased.
func rolesSet(roles []string) map[string]struct{} {
	if len(roles) == 0 {
		return defaultClickableAXRoles
	}

	set := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		if trimmed := strings.TrimSpace(r); trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}

	if len(set) == 0 {
		return defaultClickableAXRoles
	}

	return set
}

// atspiWindow implements AXWindow for an AT-SPI frame.
type atspiWindow struct {
	ref   accRef
	valid bool
}

func (w *atspiWindow) Release()     {}
func (w *atspiWindow) Role() string { return "frame" }

// atspiNode implements AXNode for a clickable AT-SPI element.
type atspiNode struct {
	id    string
	role  string
	title string
	rect  image.Rectangle
}

func (n *atspiNode) ID() string              { return n.id }
func (n *atspiNode) Bounds() image.Rectangle { return n.rect }
func (n *atspiNode) Role() string            { return n.role }
func (n *atspiNode) Title() string           { return n.title }
func (n *atspiNode) Description() string     { return "" }
func (n *atspiNode) Value() string           { return "" }
func (n *atspiNode) IsClickable() bool       { return true }
func (n *atspiNode) Release()                {}
