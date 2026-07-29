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
	"errors"
	"image"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/infra/platform"
	"github.com/y3owk1n/neru/internal/core/infra/platform/linux"
)

var errClientClosed = errors.New("AT-SPI client is closed")

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

	// Bit width of the int32 words AT-SPI packs its state and role sets into
	// (one bit per state/role index).
	atspiBitsPerWord = 32

	// Walk bounds to keep D-Bus traffic sane on deep trees.
	atspiMaxDepth = 40
	atspiMaxNodes = 2000

	// Capacity hint for the clickable-node slice: most windows fit well under
	// this and the slice grows if a dense tree exceeds it.
	atspiClickableNodesCap = 128

	// org.a11y.Status D-Bus property names.
	a11yPropIsEnabled    = "IsEnabled"
	a11yPropScreenReader = "ScreenReaderEnabled"

	// org.a11y.atspi.Collection.GetMatches fetches all descendants matching a
	// role+state predicate in a single D-Bus call, instead of the per-node walk.
	atspiCollectionIfc = "org.a11y.atspi.Collection"

	// AtspiCollectionMatchType values (atspi-constants.h): how a criterion set is
	// interpreted. MATCH_ALL requires all listed items; MATCH_ANY requires any.
	// An empty set with MATCH_ALL imposes no constraint.
	atspiMatchAll = int32(1)
	atspiMatchAny = int32(2)

	// AtspiCollectionSortOrder CANONICAL (atspi-constants.h). Ordering is
	// irrelevant here since hints are re-positioned, but a valid value is required.
	atspiSortCanonical = uint32(1)

	// Fixed word counts libatspi marshals the role/state sets into (see
	// _atspi_match_rule_marshal in at-spi2-core/atspi/atspi-matchrule.c): the role
	// bitfield is always 5 int32 words, the state bitset always 2.
	atspiRoleWords  = 5
	atspiStateWords = 2

	// collectionMaxWorkers bounds the goroutines that materialize GetMatches
	// results (role/extents/name lookups). These are latency-bound D-Bus calls
	// that godbus multiplexes over one connection, so overlapping them cuts the
	// per-match cost; the cap keeps a large page from flooding the a11y bus.
	collectionMaxWorkers = 16
)

// AtspiRole enum ids (declaration-order indices in atspi-constants.h), used to
// build the Collection.GetMatches role bitfield. The enum is ABI-stable
// (append-only), so these are fixed.
const (
	atspiRolePushButton     = 43
	atspiRoleToggleButton   = 62
	atspiRolePushButtonMenu = 129
	atspiRoleComboBox       = 11
	atspiRoleCheckBox       = 7
	atspiRoleCheckMenuItem  = 8
	atspiRoleRadioButton    = 44
	atspiRoleRadioMenuItem  = 45
	atspiRoleLinkID         = 88
	atspiRoleEntry          = 79
	atspiRolePasswordText   = 40
	atspiRoleSlider         = 51
	atspiRolePageTab        = 37
	atspiRoleMenuItemID     = 35
	atspiRoleListItem       = 32
	atspiRoleTableCell      = 56
	atspiRoleTableRow       = 90
)

// atspiRoleNameToID maps the AT-SPI role names Neru cares about (the keys of
// atspiToAXRole) to their AtspiRole id, for building the Collection.GetMatches
// role bitfield.
var atspiRoleNameToID = map[string]int32{
	"push button":     atspiRolePushButton,
	"button":          atspiRolePushButton,
	"toggle button":   atspiRoleToggleButton,
	"menu button":     atspiRolePushButtonMenu,
	"combo box":       atspiRoleComboBox,
	"check box":       atspiRoleCheckBox,
	"check menu item": atspiRoleCheckMenuItem,
	"radio button":    atspiRoleRadioButton,
	"radio menu item": atspiRoleRadioMenuItem,
	"link":            atspiRoleLinkID,
	"entry":           atspiRoleEntry,
	"password text":   atspiRolePasswordText,
	"slider":          atspiRoleSlider,
	"page tab":        atspiRolePageTab,
	"menu item":       atspiRoleMenuItemID,
	"list item":       atspiRoleListItem,
	"table cell":      atspiRoleTableCell,
	"table row":       atspiRoleTableRow,
}

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

	logger       *zap.Logger
	windowOrigin windowOriginSource

	mu        sync.Mutex
	a11y      *dbus.Conn
	a11yReady bool

	// a11y state management.
	a11yMu    sync.Mutex
	activated bool // true once we enabled AT-SPI this session
	closed    bool // true after Close() runs; prevents re-activation
	savedIsOn bool // original IsEnabled before our first enable
	savedSrOn bool // original ScreenReaderEnabled before our first enable
}

// NewATSPIClient builds the Linux accessibility client. AT-SPI is not
// activated until ensureA11yEnabled is called (lazily on first hints request),
// so hints-disabled sessions never touch the session-wide a11y status.
func NewATSPIClient(logger *zap.Logger, configProvider config.Provider) *ATSPIClient {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &ATSPIClient{
		InfraAXClient: NewInfraAXClient(logger, configProvider),
		logger:        logger.Named("accessibility.atspi"),
		windowOrigin:  newWindowOriginSource(logger),
	}
}

// FrontmostWindow returns the active top-level window via AT-SPI.
func (c *ATSPIClient) FrontmostWindow(_ context.Context) (AXWindow, error) {
	err := c.ensureA11yEnabled()
	if err != nil {
		c.logger.Warn("Failed to enable AT-SPI accessibility", zap.Error(err))
	}

	conn, err := c.ensureA11yConn()
	if err != nil {
		return nil, err
	}

	// Read the compositor's focused app_id and title once (together so they
	// describe one window) and use this single snapshot both to select the frame
	// and to record it on the window. Reading it again after selection could
	// capture a newer window's identity against the old frame, letting the
	// ClickableNodes stability check accept a stale frame.
	focusedAppID, focusedTitle, _ := linux.WaylandFocusedAppIdentity()

	frame, ok := c.findActiveFrame(conn, focusedAppID, focusedTitle)
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

	return &atspiWindow{
		ref:          frame,
		valid:        true,
		focusedAppID: focusedAppID,
		focusedTitle: focusedTitle,
	}, nil
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

	// Early-out: if focus already changed since this window was selected, the
	// frame is stale, so skip the (subprocess-spawning) geometry query below and
	// return nothing. When no focused app_id is available (X11/GNOME) there is
	// nothing to compare, so the walk proceeds. The stability is re-checked after
	// the geometry query to close the race window described there.
	if !c.focusStableSince(win.focusedAppID, win.focusedTitle) {
		return nil, nil
	}

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
		originX, originY, ok := c.windowOrigin.originFor(frameRect.Dx(), frameRect.Dy())
		if ok {
			// AT-SPI reports element coordinates in the app's own space, where the
			// frame content sits at frameRect.Min — non-zero when the toolkit adds a
			// margin (e.g. a GTK client-side-decoration shadow). The compositor
			// origin is the content's screen position, so shift element coordinates
			// by (compositor origin − frame margin) to avoid a constant offset.
			offX = originX - frameRect.Min.X
			offY = originY - frameRect.Min.Y
			haveOrigin = true
		}
	}

	// originFor reads the *current* focused window's geometry, so a focus change
	// between the early-out above and that read would pair this frame with a
	// different (equally sized) window's origin. Re-verify stability and abort if
	// focus moved, rather than walk a stale frame or apply a mismatched origin.
	if !c.focusStableSince(win.focusedAppID, win.focusedTitle) {
		return nil, nil
	}

	roleSet := rolesSet(roles)

	// Fast path: a single Collection.GetMatches call instead of the per-node
	// walk. Falls back to the walk when the toolkit doesn't implement Collection.
	if nodes, ok := c.collectViaCollection(ctx, conn, win.ref, roleSet, offX, offY); ok {
		c.logger.Debug("AT-SPI clickable scan complete",
			zap.String("path", "collection"),
			zap.Int("count", len(nodes)),
			zap.Int("offsetX", offX),
			zap.Int("offsetY", offY),
			zap.Bool("haveOrigin", haveOrigin),
			zap.Duration("elapsed", time.Since(start)))

		return nodes, nil
	}

	out := make([]AXNode, 0, atspiClickableNodesCap)
	visited := 0
	c.walk(ctx, conn, win.ref, roleSet, 0, &out, &visited, offX, offY)

	c.logger.Debug("AT-SPI clickable scan complete",
		zap.String("path", "walk"),
		zap.Int("count", len(out)),
		zap.Int("visited", visited),
		zap.Int("offsetX", offX),
		zap.Int("offsetY", offY),
		zap.Bool("haveOrigin", haveOrigin),
		zap.Duration("elapsed", time.Since(start)))

	return out, nil
}

// atspiMatchRule mirrors the AT-SPI Collection MatchRule D-Bus struct
// (aiia{ss}iaiiasib); godbus marshals the exported fields in declaration order.
type atspiMatchRule struct {
	States     []int32
	StateMatch int32
	Attributes map[string]string
	AttrMatch  int32
	Roles      []int32
	RoleMatch  int32
	Interfaces []string
	IfaceMatch int32
	Invert     bool
}

// deriveTargetRoleIDs returns the AtspiRole ids whose AX mapping is in roleSet —
// exactly the roles the per-node walk would emit — so the Collection query has
// identical semantics.
func deriveTargetRoleIDs(roleSet map[string]struct{}) []int32 {
	var ids []int32

	seen := make(map[int32]struct{})

	for atspiName, axRole := range atspiToAXRole {
		if _, ok := roleSet[axRole]; !ok {
			continue
		}

		roleID, ok := atspiRoleNameToID[atspiName]
		if !ok {
			continue
		}

		if _, dup := seen[roleID]; dup {
			continue
		}

		seen[roleID] = struct{}{}

		ids = append(ids, roleID)
	}

	return ids
}

// roleBitfield packs AtspiRole ids into the fixed 5-word int32 bitfield libatspi
// expects: bit (id%32) of word (id/32).
func roleBitfield(ids []int32) []int32 {
	words := make([]int32, atspiRoleWords)

	for _, roleID := range ids {
		if roleID < 0 {
			continue
		}

		word := roleID / atspiBitsPerWord
		if int(word) < len(words) {
			words[word] |= 1 << uint(roleID%atspiBitsPerWord)
		}
	}

	return words
}

// stateBitset packs AT-SPI state indices into the fixed 2-word int32 array
// libatspi expects: bit (s%32) of word (s/32).
func stateBitset(states ...uint) []int32 {
	words := make([]int32, atspiStateWords)

	for _, s := range states {
		w := s / atspiBitsPerWord
		if int(w) < len(words) {
			words[w] |= 1 << (s % atspiBitsPerWord)
		}
	}

	return words
}

// Close restores the org.a11y.Status to the values that were active before
// our first enable, and releases the dedicated D-Bus connection.
func (c *ATSPIClient) Close() error {
	c.a11yMu.Lock()
	wasActivated := c.activated
	restoreIsOn := c.savedIsOn
	restoreSrOn := c.savedSrOn
	c.closed = true
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

// collectViaCollection fetches clickable elements with a single
// org.a11y.atspi.Collection.GetMatches call rooted at the frame, instead of the
// recursive per-node walk. It returns (nodes, true) on success, or (nil, false)
// to signal the caller to fall back to the walk — when Collection is disabled,
// no requested role maps to an AtspiRole, or the toolkit does not implement the
// interface (the GetMatches call errors). Results have identical semantics to
// the walk: SHOWING elements whose role maps (via atspiToAXRole) into roleSet.
func (c *ATSPIClient) collectViaCollection(
	ctx context.Context,
	conn *dbus.Conn,
	root accRef,
	roleSet map[string]struct{},
	offX, offY int,
) ([]AXNode, bool) {
	if os.Getenv("NERU_ATSPI_NO_COLLECTION") != "" {
		return nil, false
	}

	ids := deriveTargetRoleIDs(roleSet)
	if len(ids) == 0 {
		// No requested role has an AtspiRole equivalent; the walk would also emit
		// nothing. Fall back rather than issue a match-nothing query.
		return nil, false
	}

	rule := atspiMatchRule{
		States:     stateBitset(atspiStateShowing),
		StateMatch: atspiMatchAll,
		Attributes: map[string]string{},
		AttrMatch:  atspiMatchAll,
		Roles:      roleBitfield(ids),
		RoleMatch:  atspiMatchAny,
		Interfaces: []string{},
		IfaceMatch: atspiMatchAll,
		Invert:     false,
	}

	var matches []accRef

	err := conn.Object(root.Name, root.Path).
		CallWithContext(ctx, atspiCollectionIfc+".GetMatches", 0,
			rule, atspiSortCanonical, int32(0), true).
		Store(&matches)
	if err != nil {
		c.logger.Debug("AT-SPI Collection.GetMatches unavailable; falling back to walk",
			zap.Error(err))

		return nil, false
	}

	// Collection is implemented but returned nothing. Don't trust that as
	// authoritative — a toolkit's Collection may be incomplete or scope the
	// subtree differently than the per-node walk (e.g. return zero on a page the
	// walk would find elements on). Fall back to the walk, which is the safe
	// baseline; genuinely empty windows are cheap to walk.
	if len(matches) == 0 {
		return nil, false
	}

	// Materialize each match (role re-check + extents + name) concurrently: the
	// lookups are independent, latency-bound D-Bus round-trips that godbus
	// multiplexes safely over the one connection. The bounded semaphore paces the
	// loop, so at most collectionMaxWorkers goroutines run at once no matter how
	// many matches there are. Each goroutine writes its own slot, so no lock is
	// needed; nil slots are dropped afterward.
	results := make([]*atspiNode, len(matches))

	sem := make(chan struct{}, collectionMaxWorkers)

	var (
		waitGroup  sync.WaitGroup
		validCount atomic.Int64
	)

	for index, ref := range matches {
		// Stop scheduling once canceled, or once we have enough *valid* nodes.
		// Capping on emitted nodes (like the walk) rather than raw candidates
		// keeps invalid/zero-sized matches from crowding out valid ones past the
		// limit. validCount lags slightly under concurrency, so a few in-flight
		// workers may push past the cap — the output loop below trims the excess.
		if ctx.Err() != nil || validCount.Load() >= atspiMaxNodes {
			break
		}

		waitGroup.Add(1)

		sem <- struct{}{}

		go func(slot int, ref accRef) {
			defer waitGroup.Done()
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			node := c.materializeMatch(conn, ref, roleSet, offX, offY)
			if node != nil {
				results[slot] = node

				validCount.Add(1)
			}
		}(index, ref)
	}

	waitGroup.Wait()

	out := make([]AXNode, 0, len(results))

	for _, node := range results {
		if node == nil {
			continue
		}

		out = append(out, node)
		if len(out) >= atspiMaxNodes {
			break
		}
	}

	// Every match failed to materialize (e.g. a dynamic UI invalidated every
	// reference between GetMatches and the per-match lookups). Treat that like an
	// empty result and fall back to the walk rather than report an empty success.
	if len(out) == 0 {
		return nil, false
	}

	return out, true
}

// materializeMatch turns a GetMatches result into an atspiNode, or nil if it
// should be dropped (role not in the requested set, or no valid on-screen
// extents). It mirrors the walk's emit filter and is safe for concurrent use —
// it only issues reads over the shared connection, which godbus multiplexes.
func (c *ATSPIClient) materializeMatch(
	conn *dbus.Conn,
	ref accRef,
	roleSet map[string]struct{},
	offX, offY int,
) *atspiNode {
	// Re-verify the role maps into the requested set: a toolkit may return a role
	// we did not ask for, and this keeps parity with the walk's filter.
	axRole, mappable := atspiToAXRole[strings.ToLower(c.roleName(conn, ref))]
	if !mappable {
		return nil
	}

	if _, ok := roleSet[axRole]; !ok {
		return nil
	}

	rect, valid := c.extents(conn, ref)
	if !valid {
		return nil
	}

	return &atspiNode{
		id:    string(ref.Path) + "@" + ref.Name,
		role:  axRole,
		title: c.name(conn, ref),
		rect:  rect.Add(image.Pt(offX, offY)),
	}
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

// setA11yProp writes a single org.a11y.Status property.
func (c *ATSPIClient) setA11yProp(prop string, val bool) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}

	return conn.Object(a11yBusDest, a11yBusPath).
		Call("org.freedesktop.DBus.Properties.Set", 0,
			a11yStatusIfc, prop, dbus.MakeVariant(val)).Err
}

// ensureA11yEnabled activates AT-SPI on first call (lazy, safe to call
// multiple times). It saves the original a11y status so Close can restore it.
// a11yMu is held across the D-Bus calls so Close() sees a consistent view:
// either activation has finished (activated == true) or it hasn't started.
func (c *ATSPIClient) ensureA11yEnabled() error {
	c.a11yMu.Lock()
	defer c.a11yMu.Unlock()

	if c.activated {
		return nil
	}

	if c.closed {
		return errClientClosed
	}

	savedIsOn, savedSrOn, err := c.readA11yStatus()
	if err != nil {
		return err
	}

	c.savedIsOn = savedIsOn
	c.savedSrOn = savedSrOn

	err = c.setA11yProp(a11yPropIsEnabled, true)
	if err != nil {
		return err
	}

	err = c.setA11yProp(a11yPropScreenReader, true)
	if err != nil {
		_ = c.setA11yProp(a11yPropIsEnabled, c.savedIsOn) // best-effort rollback

		return err
	}

	go c.windowOrigin.start()

	c.activated = true

	c.logger.Debug("AT-SPI accessibility enabled (lazy)")

	return nil
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

// focusStableSince reports whether the compositor's focused window still matches
// the one captured when the frame was selected. It compares the app_id and, for
// same-application window switches (which share an app_id), the window title. A
// mismatch means focus changed before the walk, so the selected frame is stale
// and must not be walked or offset. When no live app_id is available (X11/GNOME)
// there is nothing to compare, so it returns true. Identical or empty titles
// cannot distinguish sibling windows, so a switch between them is not detected —
// there is no distinguishing information to use.
func (c *ATSPIClient) focusStableSince(selectedAppID, selectedTitle string) bool {
	currentAppID, currentTitle, ok := linux.WaylandFocusedAppIdentity()
	if !ok {
		return true
	}

	return currentAppID == selectedAppID && currentTitle == selectedTitle
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

	word := bit / atspiBitsPerWord
	if int(word) >= len(states) {
		return false
	}

	return states[word]&(1<<(bit%atspiBitsPerWord)) != 0
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
// applications.
//
// The primary signal on Wayland is the compositor's focused app_id from
// wlr-foreign-toplevel-management (wlroots/KWin): the AT-SPI ACTIVE state is
// unreliable there — wlroots compositors such as niri/Sway/Hyprland leave the
// genuinely focused window ACTIVE=false while background frames report
// ACTIVE=true — so a frame whose application matches the focused app_id wins
// over the ACTIVE heuristic. When no app_id is available (X11, GNOME, or the
// focused app exposes no AT-SPI frame) we fall back to the ACTIVE-state
// heuristic: prefer ACTIVE+SHOWING, then any ACTIVE, then any SHOWING frame.
//
// If a focused app_id is reported but no AT-SPI application matched it, the
// ACTIVE/SHOWING fallback is used only on KWin/KDE, where ACTIVE reliably marks
// the focused window. On other Wayland compositors (wlroots) the fallback —
// including the desktop-shell last resort — could return a background surface,
// so no frame is returned and hints simply do not appear.
// The focusedAppID and focusedTitle are the caller's single focus snapshot
// (empty on X11, GNOME, or when nothing is focused), used so the selected frame
// and the identity recorded for the stability check come from the same read.
func (c *ATSPIClient) findActiveFrame(
	conn *dbus.Conn,
	focusedAppID, focusedTitle string,
) (accRef, bool) {
	root := accRef{Name: atspiRegistryDest, Path: atspiRootPath}

	// The title disambiguates multiple windows of the focused application, which
	// share an app_id.
	haveFocused := focusedAppID != ""
	haveFocusedTitle := focusedTitle != ""

	var (
		// Frames of the application the compositor reports as focused. A window
		// of that app is only chosen when uniquely identified: exactly one of its
		// windows matches the focused toplevel's title, or it has a single showing
		// window. When several windows share (or lack) the title, no unique match
		// exists and selection defers to the compositor ACTIVE state (reliable
		// only on KDE) rather than guessing a sibling.
		focusedTitleFrame   accRef
		focusedTitleCount   int // windows of the focused app matching the title
		focusedShowingFrame accRef
		haveFocusedShowing  bool
		focusedShowingCount int // showing windows of the focused app

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
		matchesFocused := haveFocused && appMatchesFocusedID(appName, focusedAppID)

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

			// Count the focused application's showing windows and how many match
			// the focused toplevel's title, so selectFrame can tell a unique
			// identification from an ambiguous one.
			if matchesFocused && showing {
				focusedShowingCount++

				if !haveFocusedShowing {
					focusedShowingFrame = frame
					haveFocusedShowing = true
				}

				if haveFocusedTitle && titleMatchesFocused(c.name(conn, frame), focusedTitle) {
					focusedTitleCount++

					if focusedTitleCount == 1 {
						focusedTitleFrame = frame
					}
				}
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

	return selectFrame(frameCandidates{
		focusedTitleFrame:   focusedTitleFrame,
		focusedTitleCount:   focusedTitleCount,
		focusedShowingFrame: focusedShowingFrame,
		focusedShowingCount: focusedShowingCount,
		activeShowing:       activeShowing,
		haveActiveShowing:   haveAS,
		activeAny:           activeAny,
		haveActiveAny:       haveAA,
		showingAny:          showingAny,
		haveShowingAny:      haveSA,
		shellShowing:        shellShowing,
		haveShell:           haveShell,
		haveFocused:         haveFocused,
		// The AT-SPI ACTIVE state reliably marks the compositor-focused window
		// only on KWin/KDE; on wlroots it is set inconsistently across frames.
		activeStateIdentifiesFocus: platform.DetectLinuxBackend() == platform.BackendWaylandKDE,
	})
}

// frameCandidates holds the frames found during a findActiveFrame walk, ranked
// by how reliably each identifies the focused window, plus the compositor
// signals needed to choose between them.
type frameCandidates struct {
	// Frames of the compositor-focused application, with the counts needed to
	// tell a unique identification from an ambiguous one.
	focusedTitleFrame   accRef // first window whose title matched the focused toplevel
	focusedTitleCount   int    // windows of the focused app matching that title
	focusedShowingFrame accRef // first showing window of the focused app
	focusedShowingCount int    // showing windows of the focused app

	// Cross-application ACTIVE/SHOWING fallbacks (reliable only on KDE).
	activeShowing     accRef
	haveActiveShowing bool
	activeAny         accRef
	haveActiveAny     bool
	showingAny        accRef
	haveShowingAny    bool

	// Desktop shell, used only as a last resort when nothing is focused.
	shellShowing accRef
	haveShell    bool

	// haveFocused is true when the compositor reported a focused app_id.
	haveFocused bool
	// activeStateIdentifiesFocus is true only where the AT-SPI ACTIVE state
	// reliably marks the focused window (KWin/KDE).
	activeStateIdentifiesFocus bool
}

// selectFrame chooses the focused frame from the ranked candidates. It is the
// pure decision half of findActiveFrame, split out so the ordering can be
// unit-tested without a live AT-SPI bus.
func selectFrame(cand frameCandidates) (accRef, bool) {
	switch {
	case cand.focusedTitleCount == 1:
		// Exactly one window of the focused app carries the focused toplevel's
		// title — an unambiguous match (works on any compositor).
		return cand.focusedTitleFrame, true
	case cand.focusedShowingCount == 1:
		// The focused app has a single showing window, so it is unambiguous even
		// without a title match.
		return cand.focusedShowingFrame, true
	case cand.haveFocused:
		// A focused app_id was reported but its window could not be uniquely
		// identified — no AT-SPI frame matched (name differs from the app_id, or
		// it exposes no AT-SPI), or several sibling windows share/lack the title.
		// On KWin/KDE the ACTIVE state still marks the focused window, so an
		// ACTIVE frame is it (covers name-mismatched apps such as GTK "Files" vs
		// org.gnome.Nautilus and duplicate-title siblings). On wlroots ACTIVE is
		// unreliable, so return nothing rather than guess a sibling or a
		// background window.
		if cand.activeStateIdentifiesFocus && cand.haveActiveShowing {
			return cand.activeShowing, true
		}

		if cand.activeStateIdentifiesFocus && cand.haveActiveAny {
			return cand.activeAny, true
		}

		return accRef{}, false
	case cand.haveActiveShowing:
		return cand.activeShowing, true
	case cand.haveActiveAny:
		return cand.activeAny, true
	case cand.haveShowingAny:
		return cand.showingAny, true
	case cand.haveShell:
		return cand.shellShowing, true
	default:
		return accRef{}, false
	}
}

// titleMatchesFocused reports whether an AT-SPI frame title equals the
// compositor's focused toplevel title. Both derive from the window's
// xdg_toplevel.title, so a whitespace-trimmed exact comparison disambiguates
// windows of the same application. Empty titles never match.
func titleMatchesFocused(frameTitle, focusedTitle string) bool {
	frameTitle = strings.TrimSpace(frameTitle)

	return frameTitle != "" && frameTitle == strings.TrimSpace(focusedTitle)
}

// appMatchesFocusedID reports whether an AT-SPI application name corresponds to
// the compositor's focused app_id. The two vocabularies differ: AT-SPI reports
// a display name ("Firefox") while wlr-foreign-toplevel reports an app_id that
// is often reverse-DNS ("org.mozilla.firefox") or a bare id ("helium"). The
// match is case-insensitive and also compares the last dot-segment of either
// side, so "Firefox" matches "org.mozilla.firefox" and "Helium" matches
// "helium".
func appMatchesFocusedID(atspiName, appID string) bool {
	name := strings.ToLower(strings.TrimSpace(atspiName))
	focusedID := strings.ToLower(strings.TrimSpace(appID))

	if name == "" || focusedID == "" {
		return false
	}

	if name == focusedID {
		return true
	}

	if seg := lastDotSegment(focusedID); seg != "" && seg == name {
		return true
	}

	if seg := lastDotSegment(name); seg != "" && seg == focusedID {
		return true
	}

	return false
}

// lastDotSegment returns the substring after the final '.', or "" when there is
// no interior dot (nothing after the dot, or no dot at all).
func lastDotSegment(value string) string {
	i := strings.LastIndex(value, ".")
	if i < 0 || i == len(value)-1 {
		return ""
	}

	return value[i+1:]
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
	// focusedAppID and focusedTitle are the compositor's focused app_id and
	// window title captured when this window was selected (empty on X11/GNOME).
	// ClickableNodes compares both against the live values so a focus change
	// between selection and the walk — including a switch to another window of
	// the same application (same app_id, different title) — is detected and the
	// stale frame is not walked or offset.
	focusedAppID string
	focusedTitle string
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
