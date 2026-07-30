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
	"github.com/y3owk1n/neru/internal/core/domain/element"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
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

	// atspiCallTimeout bounds a single AT-SPI D-Bus round-trip. A healthy
	// toolkit answers each read (role, extents, name, children, state) in
	// single-digit milliseconds; this only trips when an app is wedged or not
	// answering accessibility queries. Without it a single hung call would
	// consume the whole element-scan budget and the CLI would give up with a
	// bare timeout (or the daemon would log a broken-pipe write after the
	// client disconnected). Bounding each call lets a stuck node be dropped and
	// the scan continue.
	atspiCallTimeout = 500 * time.Millisecond

	// atspiPropertiesGet is the standard D-Bus Properties.Get method, called
	// with a context so a wedged property read cannot block indefinitely (the
	// godbus GetProperty convenience method has no context-aware variant).
	atspiPropertiesGet = "org.freedesktop.DBus.Properties.Get"

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

// atspiRoleNameToID maps AT-SPI role names, as returned by
// Accessible.GetRoleName, to their AtspiRole id — the declaration-order index
// in atspi-constants.h. Ids are needed to build the Collection.GetMatches role
// bitfield, so any role missing here forces the slow per-node walk instead.
//
// The enum is ABI-stable (append-only), so these are fixed. The table is
// complete as of AT-SPI 2.50 (ATSPI_ROLE_PUSH_BUTTON_MENU, the highest id
// below, is the last member); newer roles simply fall back to the walk.
var atspiRoleNameToID = map[string]int32{
	"invalid":               0,
	"accelerator label":     1,
	"alert":                 2,
	"animation":             3,
	"arrow":                 4,
	"calendar":              5,
	"canvas":                6,
	"check box":             7,
	"check menu item":       8,
	"color chooser":         9,
	"column header":         10,
	"combo box":             11,
	"date editor":           12,
	"desktop icon":          13,
	"desktop frame":         14,
	"dial":                  15,
	"dialog":                16,
	"directory pane":        17,
	"drawing area":          18,
	"file chooser":          19,
	"filler":                20,
	"focus traversable":     21,
	"font chooser":          22,
	"frame":                 23,
	"glass pane":            24,
	"html container":        25,
	"icon":                  26,
	"image":                 27,
	"internal frame":        28,
	"label":                 29,
	"layered pane":          30,
	"list":                  31,
	"list item":             32,
	"menu":                  33,
	"menu bar":              34,
	"menu item":             35,
	"option pane":           36,
	"page tab":              37,
	"page tab list":         38,
	"panel":                 39,
	"password text":         40,
	"popup menu":            41,
	"progress bar":          42,
	"push button":           43,
	"radio button":          44,
	"radio menu item":       45,
	"root pane":             46,
	"row header":            47,
	"scroll bar":            48,
	"scroll pane":           49,
	"separator":             50,
	"slider":                51,
	"spin button":           52,
	"split pane":            53,
	"status bar":            54,
	"table":                 55,
	"table cell":            56,
	"table column header":   57,
	"table row header":      58,
	"tearoff menu item":     59,
	"terminal":              60,
	"text":                  61,
	"toggle button":         62,
	"tool bar":              63,
	"tool tip":              64,
	"tree":                  65,
	"tree table":            66,
	"unknown":               67,
	"viewport":              68,
	"window":                69,
	"extended":              70,
	"header":                71,
	"footer":                72,
	"paragraph":             73,
	"ruler":                 74,
	"application":           75,
	"autocomplete":          76,
	"editbar":               77,
	"embedded":              78,
	"entry":                 79,
	"chart":                 80,
	"caption":               81,
	"document frame":        82,
	"heading":               83,
	"page":                  84,
	"section":               85,
	"redundant object":      86,
	"form":                  87,
	"link":                  88,
	"input method window":   89,
	"table row":             90,
	"tree item":             91,
	"document spreadsheet":  92,
	"document presentation": 93,
	"document text":         94,
	"document web":          95,
	"document email":        96,
	"comment":               97,
	"list box":              98,
	"grouping":              99,
	"image map":             100,
	"notification":          101,
	"info bar":              102,
	"level bar":             103,
	"title bar":             104,
	"block quote":           105,
	"audio":                 106,
	"video":                 107,
	"definition":            108,
	"article":               109,
	"landmark":              110,
	"log":                   111,
	"marquee":               112,
	"math":                  113,
	"rating":                114,
	"timer":                 115,
	"static":                116,
	"math fraction":         117,
	"math root":             118,
	"subscript":             119,
	"superscript":           120,
	"description list":      121,
	"description term":      122,
	"description value":     123,
	"footnote":              124,
	"content deletion":      125,
	"content insertion":     126,
	"mark":                  127,
	"suggestion":            128,
	"push button menu":      129,
	"switch":                130,

	// Compatibility aliases. Accessible.GetRoleName returns the GEnum nick of
	// the role with hyphens replaced by spaces, so id 43 reports "button" on
	// current at-spi2-core (ATSPI_ROLE_BUTTON) and "push button" on releases
	// predating that rename. Both are accepted so one config works either way.
	// "menu button" is accepted because users reach for it before the less
	// obvious "push button menu".
	"button":      43,
	"menu button": 129,
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

// defaultClickableRoles is used when the caller passes no explicit role filter.
// It is the shipped default role list resolved into AT-SPI role names, so the
// fallback and a default configuration select exactly the same elements.
var defaultClickableRoles = func() map[string]struct{} {
	resolution := element.ResolveRoles(element.DefaultClickableRoles, "linux")

	set := make(map[string]struct{}, len(resolution.Native))
	for _, native := range resolution.Native {
		set[strings.ToLower(native)] = struct{}{}
	}

	return set
}()

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

	// Event-driven active-window cache. A background listener records the AT-SPI
	// window that most recently emitted a window:activate event, so frame
	// selection can skip the registry scan entirely when the cached window still
	// matches the compositor's focused toplevel. See atspi_events_linux.go.
	activeMu     sync.Mutex
	activeWindow *atspiActiveWindow // nil until the first window:activate arrives
	eventsReady  bool               // true once the listener is registered on c.a11y
	eventsStop   chan struct{}      // closed to stop the listener goroutine
	eventsClosed bool               // true after Close; blocks starting a new listener
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
func (c *ATSPIClient) FrontmostWindow(ctx context.Context) (AXWindow, error) {
	err := c.ensureA11yEnabled()
	if err != nil {
		c.logger.Warn("Failed to enable AT-SPI accessibility", zap.Error(err))
	}

	conn, err := c.ensureA11yConn()
	if err != nil {
		return nil, err
	}

	// Start (once) the window:activate listener so subsequent frame selections can
	// read the event-tracked active window instead of scanning the registry.
	c.ensureA11yEvents(conn)

	// Attach a scan diagnostic so the bounded frame-selection reads can flag a
	// fatal AT-SPI failure (a timeout, a closed connection, or the registry/app
	// leaving the bus). Without it a registry that stops responding looks like
	// "no active frame" and hints show nothing with no error.
	diag := &atspiScanDiag{}
	ctx = context.WithValue(ctx, atspiScanDiagKey{}, diag)

	// Read the compositor's focused app_id and title once (together so they
	// describe one window) and use this single snapshot both to select the frame
	// and to record it on the window. Reading it again after selection could
	// capture a newer window's identity against the old frame, letting the
	// ClickableNodes stability check accept a stale frame.
	focusedAppID, focusedTitle, _ := linux.WaylandFocusedAppIdentity()

	frame, ok := c.findActiveFrame(ctx, conn, focusedAppID, focusedTitle)
	if !ok {
		// Distinguish a genuine "nothing focused" from an AT-SPI failure during
		// selection: only the latter recorded a scan-fatal error. A frame that
		// *was* found is returned even if some other app errored mid-scan.
		ferr := diag.fatalErr()
		if ferr != nil {
			return nil, scanFailureError(ferr)
		}

		c.logger.Debug("AT-SPI: no active frame found")

		// No active frame: hand back an empty window so the adapter simply
		// finds no clickable elements rather than erroring out.
		return &atspiWindow{}, nil
	}

	c.logger.Debug("AT-SPI: selected active frame",
		zap.String("bus", frame.Name),
		zap.String("app", c.name(ctx, conn, accRef{Name: frame.Name, Path: atspiRootPath})),
		zap.String("frameTitle", c.name(ctx, conn, frame)))

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

	// Attach a per-scan diagnostic so the leaf D-Bus helpers can flag a per-call
	// timeout. It lets an empty result caused by an unresponsive app surface as a
	// clear timeout below instead of looking like a window with no elements.
	diag := &atspiScanDiag{}
	ctx = context.WithValue(ctx, atspiScanDiagKey{}, diag)

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

	frameRect, frameOK := c.extents(ctx, conn, win.ref)
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

	// An empty scan is normal for a window with nothing clickable, but if we got
	// here because reads kept failing fatally — the app timed out, exited, or
	// dropped its accessibility connection — report that rather than silently
	// showing no hints. A partial result (some nodes collected) is returned as-is.
	if len(out) == 0 {
		ferr := diag.fatalErr()
		if ferr != nil {
			return nil, scanFailureError(ferr)
		}
	}

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

// deriveTargetRoleIDs returns the AtspiRole ids for the requested role names —
// exactly the roles the per-node walk would keep — so the Collection query has
// identical semantics. Requested names with no known id are skipped; the caller
// falls back to the walk when nothing resolves.
func deriveTargetRoleIDs(roleSet map[string]struct{}) []int32 {
	var ids []int32

	seen := make(map[int32]struct{})

	for roleName := range roleSet {
		roleID, ok := atspiRoleNameToID[roleName]
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

	// Stop the window:activate listener goroutine before the connection closes,
	// and block any in-flight ensureA11yEvents from starting a new one.
	c.activeMu.Lock()
	if c.eventsStop != nil {
		close(c.eventsStop)
		c.eventsStop = nil
	}

	c.eventsReady = false
	c.eventsClosed = true
	c.activeMu.Unlock()

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

		// GetMatches runs on the full scan budget rather than a per-call
		// deadline, so it is the one scan call the leaf helpers don't cover.
		// Record a scan-fatal error (a deadline, or the app leaving the bus)
		// here so an empty fallback walk still surfaces it; a benign
		// "Collection not implemented" error is ignored and the walk proceeds.
		c.noteCallErr(ctx, err)

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

			node := c.materializeMatch(ctx, conn, ref, roleSet, offX, offY)
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
	ctx context.Context,
	conn *dbus.Conn,
	ref accRef,
	roleSet map[string]struct{},
	offX, offY int,
) *atspiNode {
	// Re-verify the role is in the requested set: a toolkit may return a role we
	// did not ask for, and this keeps parity with the walk's filter.
	roleName := strings.ToLower(c.roleName(ctx, conn, ref))

	if _, ok := roleSet[roleName]; !ok {
		return nil
	}

	rect, valid := c.extents(ctx, conn, ref)
	if !valid {
		return nil
	}

	return &atspiNode{
		id:    string(ref.Path) + "@" + ref.Name,
		role:  roleName,
		title: c.name(ctx, conn, ref),
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

	// Re-check closed while holding mu, not before it. Close sets closed (under
	// a11yMu) before its own mu section runs, so once Close's teardown has
	// completed, any ensureA11yConn that then acquires mu sees closed==true and
	// bails — it cannot publish a fresh connection that Close would never clean
	// up. If instead this call wins mu first, it publishes c.a11y and Close's
	// later mu section closes that connection. Either ordering leaks nothing.
	c.a11yMu.Lock()
	closed := c.closed
	c.a11yMu.Unlock()

	if closed {
		return nil, errClientClosed
	}

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

	// The window:activate listener is bound to the previous connection. Stop the
	// old goroutine, force a re-register on the new connection, and drop the
	// now-unreachable cached window. Closing the old stop here (rather than relying
	// on the old connection's signal channel closing) keeps listener lifetime
	// independent of godbus internals.
	c.activeMu.Lock()

	if c.eventsStop != nil {
		close(c.eventsStop)
		c.eventsStop = nil
	}

	c.eventsReady = false
	c.activeWindow = nil

	c.activeMu.Unlock()

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

// atspiScanDiag records diagnostics for a single ClickableNodes scan. It rides
// on the scan context so the leaf D-Bus helpers can report a scan-fatal failure
// (a per-call timeout, a closed connection, or the target app disappearing from
// the bus) without threading an extra parameter through the whole walk. A scan
// that ends up empty for one of those reasons can then surface an actionable
// error instead of a silent empty result; benign per-node errors are ignored.
type atspiScanDiag struct {
	mu  sync.Mutex
	err error
}

// note records the first scan-fatal error. Safe for concurrent use by the
// materialize/walk goroutines.
func (d *atspiScanDiag) note(err error) {
	d.mu.Lock()
	if d.err == nil {
		d.err = err
	}
	d.mu.Unlock()
}

// fatalErr returns the first scan-fatal error, or nil. Call after the scan's
// goroutines have joined.
func (d *atspiScanDiag) fatalErr() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.err
}

// atspiScanDiagKey is the context key for the active scan's *atspiScanDiag.
type atspiScanDiagKey struct{}

// D-Bus error names that mean the scan cannot make progress: the target
// application has left the bus, or its connection is gone. Deliberately NOT
// including UnknownMethod (GetChildren has a fallback) or per-node errors on a
// stale node, so an otherwise-healthy — or genuinely empty — window is never
// misreported as a failure.
const (
	atspiErrServiceUnknown = "org.freedesktop.DBus.Error.ServiceUnknown"
	atspiErrNoReply        = "org.freedesktop.DBus.Error.NoReply"
	atspiErrDisconnected   = "org.freedesktop.DBus.Error.Disconnected"
)

// isScanFatalErr reports whether err means the whole scan is failing rather than
// a single benign node: a per-call context deadline, a closed connection, or the
// application having disappeared from the bus.
func isScanFatalErr(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, dbus.ErrClosed) {
		return true
	}

	if dbusErr, ok := errors.AsType[dbus.Error](err); ok {
		switch dbusErr.Name {
		case atspiErrServiceUnknown, atspiErrNoReply, atspiErrDisconnected:
			return true
		}
	}

	return false
}

// noteCallErr records a scan-fatal error on the scan's diagnostic (if one is
// attached to ctx). Benign errors, and contexts with no diag attached (e.g. the
// frame-selection reads in findActiveFrame), are ignored.
func (c *ATSPIClient) noteCallErr(ctx context.Context, err error) {
	if !isScanFatalErr(err) {
		return
	}

	if d, ok := ctx.Value(atspiScanDiagKey{}).(*atspiScanDiag); ok {
		d.note(err)
	}
}

// scanFailureError turns a scan-fatal error into a user-facing hints error,
// distinguishing an unresponsive app (per-call deadline) from one that has gone.
func scanFailureError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return derrors.New(derrors.CodeTimeout,
			"AT-SPI element scan timed out; the app may be slow or "+
				"unresponsive to accessibility queries")
	}

	return derrors.Wrap(err, derrors.CodeAccessibilityFailed,
		"AT-SPI element scan failed; the app may have exited or lost its "+
			"accessibility connection")
}

// children returns the AT-SPI children of an accessible. It prefers the bulk
// GetChildren method and falls back to ChildCount + GetChildAtIndex for older
// toolkits that do not expose GetChildren.
func (c *ATSPIClient) children(ctx context.Context, conn *dbus.Conn, ref accRef) []accRef {
	obj := conn.Object(ref.Name, ref.Path)

	var kids []accRef

	childrenCtx, cancel := context.WithTimeout(ctx, atspiCallTimeout)
	defer cancel()

	err := obj.CallWithContext(childrenCtx, atspiAccessibleIfc+".GetChildren", 0).Store(&kids)
	if err == nil {
		return kids
	}

	c.noteCallErr(ctx, err)

	var countVar dbus.Variant

	countCtx, cancelCount := context.WithTimeout(ctx, atspiCallTimeout)
	defer cancelCount()

	propErr := obj.CallWithContext(countCtx, atspiPropertiesGet, 0,
		atspiAccessibleIfc, "ChildCount").Store(&countVar)
	if propErr != nil {
		c.noteCallErr(ctx, propErr)

		return nil
	}

	count, _ := countVar.Value().(int32)
	for i := range count {
		var child accRef

		childCtx, cancelChild := context.WithTimeout(ctx, atspiCallTimeout)

		err := obj.CallWithContext(childCtx, atspiAccessibleIfc+".GetChildAtIndex", 0, i).
			Store(&child)

		cancelChild()

		if err != nil {
			c.noteCallErr(ctx, err)

			continue
		}

		kids = append(kids, child)
	}

	return kids
}

// isWindowRole reports whether an AT-SPI role name denotes a top-level window
// (the only frames hint selection considers).
func isWindowRole(role string) bool {
	return role == "frame" || role == "window" || role == "dialog"
}

// roleName returns the AT-SPI localized-independent role name (e.g. "push button").
func (c *ATSPIClient) roleName(ctx context.Context, conn *dbus.Conn, ref accRef) string {
	callCtx, cancel := context.WithTimeout(ctx, atspiCallTimeout)
	defer cancel()

	var name string

	err := conn.Object(ref.Name, ref.Path).
		CallWithContext(callCtx, atspiAccessibleIfc+".GetRoleName", 0).Store(&name)
	if err != nil {
		c.noteCallErr(ctx, err)

		return ""
	}

	return name
}

// name returns the accessible Name property (used as the element title).
func (c *ATSPIClient) name(ctx context.Context, conn *dbus.Conn, ref accRef) string {
	callCtx, cancel := context.WithTimeout(ctx, atspiCallTimeout)
	defer cancel()

	var val dbus.Variant

	err := conn.Object(ref.Name, ref.Path).
		CallWithContext(callCtx, atspiPropertiesGet, 0, atspiAccessibleIfc, "Name").Store(&val)
	if err != nil {
		c.noteCallErr(ctx, err)

		return ""
	}

	s, _ := val.Value().(string)

	return s
}

// states reads the AT-SPI state bitfield of an accessible in a single GetState
// round-trip. Callers pull individual bits with atspiStateBit, so a frame's
// ACTIVE and SHOWING state come from one call rather than two.
func (c *ATSPIClient) states(ctx context.Context, conn *dbus.Conn, ref accRef) ([]uint32, bool) {
	callCtx, cancel := context.WithTimeout(ctx, atspiCallTimeout)
	defer cancel()

	var states []uint32

	err := conn.Object(ref.Name, ref.Path).
		CallWithContext(callCtx, atspiAccessibleIfc+".GetState", 0).Store(&states)
	if err != nil || len(states) == 0 {
		c.noteCallErr(ctx, err)

		return nil, false
	}

	return states, true
}

// atspiStateBit reports whether the given AT-SPI state bit is set in a state
// bitfield returned by GetState.
func atspiStateBit(states []uint32, bit uint) bool {
	word := bit / atspiBitsPerWord
	if int(word) >= len(states) {
		return false
	}

	return states[word]&(1<<(bit%atspiBitsPerWord)) != 0
}

// stateHas reports whether the accessible has the given AT-SPI state bit set.
func (c *ATSPIClient) stateHas(ctx context.Context, conn *dbus.Conn, ref accRef, bit uint) bool {
	states, ok := c.states(ctx, conn, ref)
	if !ok {
		return false
	}

	return atspiStateBit(states, bit)
}

// frameStates reads a frame's ACTIVE and SHOWING state from a single GetState
// call, halving the per-frame state round-trips during frame selection.
// The two results are the ACTIVE and SHOWING states, in that order.
func (c *ATSPIClient) frameStates(ctx context.Context, conn *dbus.Conn, ref accRef) (bool, bool) {
	states, ok := c.states(ctx, conn, ref)
	if !ok {
		return false, false
	}

	return atspiStateBit(states, atspiStateActive), atspiStateBit(states, atspiStateShowing)
}

// extents returns the on-screen rectangle of an accessible.
func (c *ATSPIClient) extents(
	ctx context.Context,
	conn *dbus.Conn,
	ref accRef,
) (image.Rectangle, bool) {
	callCtx, cancel := context.WithTimeout(ctx, atspiCallTimeout)
	defer cancel()

	var ext atspiExtents

	err := conn.Object(ref.Name, ref.Path).
		CallWithContext(callCtx, atspiComponentIfc+".GetExtents", 0, atspiCoordScreen).Store(&ext)
	if err != nil {
		c.noteCallErr(ctx, err)

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
	ctx context.Context,
	conn *dbus.Conn,
	focusedAppID, focusedTitle string,
) (accRef, bool) {
	root := accRef{Name: atspiRegistryDest, Path: atspiRootPath}

	start := time.Now()

	// Fastest path: an event-tracked active window that still matches the
	// compositor's focused toplevel. The background window:activate listener keeps
	// this current, so a match resolves the frame with zero registry scanning. It
	// is validated against the live focus identity, so a stale or missing cache
	// simply falls through to the scan below — never a wrong window.
	if frame, ok := c.activeWindowMatching(focusedAppID, focusedTitle); ok {
		c.logFrameSelection(start, "event-cache", 0, 0)

		return frame, true
	}

	// The title disambiguates multiple windows of the focused application, which
	// share an app_id.
	haveFocused := focusedAppID != ""
	haveFocusedTitle := focusedTitle != ""

	// The AT-SPI ACTIVE state reliably marks the compositor-focused window only
	// on KWin/KDE; on wlroots it is set inconsistently across frames.
	activeStateIdentifiesFocus := platform.DetectLinuxBackend() == platform.BackendWaylandKDE

	apps := c.children(ctx, conn, root)

	// Fast path: when the compositor reports a focused app_id, resolve the window
	// from that one application's frames. The focused app is located with a
	// cancel-on-match name probe (findFocusedApps) that stops the instant a match
	// is seen, so a single wedged background app — common on a desktop with many
	// open windows — can no longer floor the probe at the per-call timeout. Every
	// other app on the bus is skipped. The cross-application ACTIVE/SHOWING
	// fallbacks selectFrame would build from the other apps are never consulted
	// while a focused app_id is present, except the KWin/KDE ACTIVE fallback used
	// when the focused app cannot be uniquely resolved (handled by the full scan
	// below).
	if haveFocused {
		matches := c.findFocusedApps(ctx, conn, apps, focusedAppID)
		if len(matches) > 0 {
			metas := make([]appMeta, len(apps))
			for index := range metas {
				metas[index].skip = true
			}

			for _, index := range matches {
				metas[index] = appMeta{matchesFocused: true}
			}

			scans, scanned := c.scanApps(
				ctx,
				conn,
				apps,
				metas,
				haveFocusedTitle,
				focusedTitle,
				true,
			)
			cand := mergeFrameScan(scans)

			if cand.focusedTitleCount == 1 {
				c.logFrameSelection(start, "focused-title", len(apps), scanned)

				return cand.focusedTitleFrame, true
			}

			if cand.focusedShowingCount == 1 {
				c.logFrameSelection(start, "focused-showing", len(apps), scanned)

				return cand.focusedShowingFrame, true
			}
		}

		if !activeStateIdentifiesFocus {
			// wlroots: the ACTIVE-state fallback is unreliable, so a focused app
			// that cannot be uniquely resolved yields no frame — and there is no
			// reason to scan the rest of the bus.
			c.logFrameSelection(start, "focused-unresolved", len(apps), 0)

			return accRef{}, false
		}

		// KWin/KDE: fall through to the cross-application ACTIVE fallback, which
		// needs every app's frames.
	}

	// Full scan: no focused app_id (X11/GNOME), or KWin/KDE with a focused app
	// that could not be uniquely resolved and needs the cross-app ACTIVE state.
	// This path names every app, so a wedged app can still cost one timeout — but
	// it is reached only without a usable focused app_id, not on the wlroots hot
	// path above.
	names := c.appNames(ctx, conn, apps)

	metas := make([]appMeta, len(apps))

	for index := range apps {
		appName := names[index]

		// Skip surfaces that are never valid hint targets and that steal the
		// ACTIVE state right after a libei cursor move: on-screen virtual
		// keyboards (e.g. the maliit "plasma-keyboard"), the XWayland video
		// bridge, and the portal consent dialog.
		if isVirtualKeyboardApp(appName) || isNonTargetSurfaceApp(appName) {
			metas[index].skip = true

			continue
		}

		metas[index].isShell = isDesktopShellApp(appName)
		metas[index].matchesFocused = haveFocused && !metas[index].isShell &&
			appMatchesFocusedID(appName, focusedAppID)
	}

	scans, scanned := c.scanApps(ctx, conn, apps, metas, haveFocusedTitle, focusedTitle, false)

	cand := mergeFrameScan(scans)
	cand.haveFocused = haveFocused
	cand.activeStateIdentifiesFocus = activeStateIdentifiesFocus

	frame, ok := selectFrame(cand)

	c.logFrameSelection(start, "full-scan", len(apps), scanned)

	return frame, ok
}

// appMeta classifies one AT-SPI application for frame selection: whether it is
// skipped entirely, is the desktop shell, or is the compositor-focused app.
type appMeta struct {
	skip           bool
	isShell        bool
	matchesFocused bool
}

// scannedFrame is one frame of an application captured during the registry scan,
// with the state bits frame selection needs.
type scannedFrame struct {
	ref          accRef
	active       bool
	showing      bool
	titleMatches bool
}

// appFramesScan holds one application's scanned frames plus the classification
// needed to merge them into frameCandidates in registry order.
type appFramesScan struct {
	scanned        bool
	isShell        bool
	matchesFocused bool
	frames         []scannedFrame
}

// appNames reads the accessible Name of every application concurrently. Names
// are latency-bound D-Bus reads that godbus multiplexes over the one connection,
// so overlapping them turns an O(apps) sequential probe into one bounded by the
// slowest single reply. The bounded semaphore caps in-flight calls so a busy
// desktop cannot flood the a11y bus.
func (c *ATSPIClient) appNames(ctx context.Context, conn *dbus.Conn, apps []accRef) []string {
	names := make([]string, len(apps))

	sem := make(chan struct{}, collectionMaxWorkers)

	var waitGroup sync.WaitGroup

	for index, app := range apps {
		waitGroup.Add(1)

		sem <- struct{}{}

		go func(slot int, app accRef) {
			defer waitGroup.Done()
			defer func() { <-sem }()

			names[slot] = c.name(ctx, conn, app)
		}(index, app)
	}

	waitGroup.Wait()

	return names
}

// findFocusedApps returns the indices of the applications whose name matches the
// compositor's focused app_id. It probes names concurrently and stops the moment
// a match is seen (canceling the outstanding probes), so a single wedged
// background app cannot stall the probe at the per-call timeout — the failure
// mode on a desktop with many open windows, where every frame selection would
// otherwise pay ~atspiCallTimeout waiting on the same unresponsive app. Virtual
// keyboards, non-target surfaces, and the desktop shell are never focus matches
// and are excluded. A no-match probe still waits for every app (there is nothing
// to short-circuit on), but that path yields no frame anyway.
func (c *ATSPIClient) findFocusedApps(
	ctx context.Context,
	conn *dbus.Conn,
	apps []accRef,
	focusedAppID string,
) []int {
	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	matchCh := make(chan int, len(apps))

	go func() {
		sem := make(chan struct{}, collectionMaxWorkers)

		var waitGroup sync.WaitGroup

		for index, app := range apps {
			if probeCtx.Err() != nil {
				break
			}

			waitGroup.Add(1)

			sem <- struct{}{}

			go func(index int, app accRef) {
				defer waitGroup.Done()
				defer func() { <-sem }()

				if probeCtx.Err() != nil {
					return
				}

				name := c.name(probeCtx, conn, app)
				if name == "" ||
					isVirtualKeyboardApp(name) ||
					isNonTargetSurfaceApp(name) ||
					isDesktopShellApp(name) {
					return
				}

				if appMatchesFocusedID(name, focusedAppID) {
					select {
					case matchCh <- index:
					case <-probeCtx.Done():
					}
				}
			}(index, app)
		}

		waitGroup.Wait()
		close(matchCh)
	}()

	var matches []int

	// Block for the first match, then drain any others already delivered before
	// canceling. The common case is a single application (its multiple windows
	// are frames *within* one AT-SPI app, counted later by scanApps), so this
	// returns promptly. Draining still catches other co-active apps that share the
	// app_id, so a genuinely ambiguous focus is reported as ambiguous rather than
	// silently resolved to whichever app answered first. A matching app that only
	// responds after the drain is not counted — acceptable, since that requires
	// two distinct apps sharing one app_id with no title to disambiguate them.
	first, ok := <-matchCh
	if !ok {
		return matches
	}

	matches = append(matches, first)

	for drained := false; !drained; {
		select {
		case index, more := <-matchCh:
			if !more {
				drained = true

				break
			}

			matches = append(matches, index)
		default:
			drained = true
		}
	}

	// Stop probing the rest of the bus so an unresponsive app never gates the
	// result; the background goroutine drains and closes matchCh after the
	// canceled probes return.
	cancel()

	return matches
}

// scanApps reads the frames of the selected applications concurrently, returning
// one appFramesScan per app in registry order (unscanned apps keep a zero value)
// and the total number of frames scanned. When onlyMatching is set, only the
// compositor-focused application(s) are scanned — the fast path — otherwise every
// non-skipped app is. Each goroutine writes its own slot, so no lock is needed.
func (c *ATSPIClient) scanApps(
	ctx context.Context,
	conn *dbus.Conn,
	apps []accRef,
	metas []appMeta,
	haveFocusedTitle bool,
	focusedTitle string,
	onlyMatching bool,
) ([]appFramesScan, int) {
	results := make([]appFramesScan, len(apps))

	sem := make(chan struct{}, collectionMaxWorkers)

	var waitGroup sync.WaitGroup

	for index := range apps {
		meta := metas[index]
		if meta.skip || (onlyMatching && !meta.matchesFocused) {
			continue
		}

		waitGroup.Add(1)

		sem <- struct{}{}

		go func(slot int, meta appMeta) {
			defer waitGroup.Done()
			defer func() { <-sem }()

			results[slot] = c.scanOneApp(
				ctx,
				conn,
				apps[slot],
				meta,
				haveFocusedTitle,
				focusedTitle,
			)
		}(index, meta)
	}

	waitGroup.Wait()

	framesScanned := 0
	for i := range results {
		framesScanned += len(results[i].frames)
	}

	return results, framesScanned
}

// scanOneApp reads an application's window/dialog frames and their ACTIVE/SHOWING
// state (one GetState per frame), plus the title of the focused app's showing
// frames so a sibling window can be disambiguated. It is safe for concurrent use:
// it only issues reads over the shared connection, which godbus multiplexes.
func (c *ATSPIClient) scanOneApp(
	ctx context.Context,
	conn *dbus.Conn,
	app accRef,
	meta appMeta,
	haveFocusedTitle bool,
	focusedTitle string,
) appFramesScan {
	res := appFramesScan{scanned: true, isShell: meta.isShell, matchesFocused: meta.matchesFocused}

	for _, frame := range c.children(ctx, conn, app) {
		role := c.roleName(ctx, conn, frame)
		if !isWindowRole(role) {
			continue
		}

		active, showing := c.frameStates(ctx, conn, frame)

		scanned := scannedFrame{ref: frame, active: active, showing: showing}

		// Only the focused app's showing frames need a title read, and only to
		// disambiguate its sibling windows.
		if meta.matchesFocused && showing && haveFocusedTitle {
			scanned.titleMatches = titleMatchesFocused(c.name(ctx, conn, frame), focusedTitle)
		}

		res.frames = append(res.frames, scanned)
	}

	return res
}

// mergeFrameScan folds the per-app scans into frameCandidates in registry order,
// reproducing the original sequential selection exactly: the desktop shell is
// held aside, the focused app's showing/title-matching windows are counted, and
// the first ACTIVE/SHOWING frames become the cross-application fallbacks.
func mergeFrameScan(scans []appFramesScan) frameCandidates {
	var cand frameCandidates

	for _, scan := range scans {
		if !scan.scanned {
			continue
		}

		for _, frame := range scan.frames {
			// The desktop shell never wins the active-frame race; it is kept aside
			// and only used if no real application frame is found.
			if scan.isShell {
				if frame.showing && !cand.haveShell {
					cand.shellShowing = frame.ref
					cand.haveShell = true
				}

				continue
			}

			// Count the focused application's showing windows and how many match
			// the focused toplevel's title, so selectFrame can tell a unique
			// identification from an ambiguous one.
			if scan.matchesFocused && frame.showing {
				cand.focusedShowingCount++

				if cand.focusedShowingCount == 1 {
					cand.focusedShowingFrame = frame.ref
				}

				if frame.titleMatches {
					cand.focusedTitleCount++

					if cand.focusedTitleCount == 1 {
						cand.focusedTitleFrame = frame.ref
					}
				}
			}

			if frame.active && frame.showing && !cand.haveActiveShowing {
				cand.activeShowing = frame.ref
				cand.haveActiveShowing = true
			}

			if frame.active && !cand.haveActiveAny {
				cand.activeAny = frame.ref
				cand.haveActiveAny = true
			}

			if frame.showing && !cand.haveShowingAny {
				cand.showingAny = frame.ref
				cand.haveShowingAny = true
			}
		}
	}

	return cand
}

// logFrameSelection records how frame selection resolved and how much of the bus
// it had to scan, so a slow selection on a busy desktop is visible in the logs.
func (c *ATSPIClient) logFrameSelection(start time.Time, path string, apps, framesScanned int) {
	c.logger.Debug("AT-SPI frame selection complete",
		zap.String("path", path),
		zap.Int("apps", apps),
		zap.Int("framesScanned", framesScanned),
		zap.Duration("elapsed", time.Since(start)))
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

	// Match the AT-SPI role name against the requested set. Both sides are
	// native AT-SPI names: configured roles are resolved into this vocabulary
	// before they reach the client, and the node carries the native name
	// downstream so Adapter.MatchesFilter compares like with like.
	roleName := strings.ToLower(c.roleName(ctx, conn, ref))

	if _, ok := roles[roleName]; ok && c.stateHas(ctx, conn, ref, atspiStateShowing) {
		if rect, valid := c.extents(ctx, conn, ref); valid {
			// AT-SPI reports window-relative coords on Wayland; offset by the
			// focused window's screen origin from the KWin bridge.
			rect = rect.Add(image.Pt(offX, offY))
			*out = append(*out, &atspiNode{
				id:    string(ref.Path) + "@" + ref.Name,
				role:  roleName,
				title: c.name(ctx, conn, ref),
				rect:  rect,
			})
		}
	}

	for _, child := range c.children(ctx, conn, ref) {
		c.walk(ctx, conn, child, roles, depth+1, out, visited, offX, offY)
	}
}

// rolesSet converts the caller's native AT-SPI role list into a lookup set,
// falling back to the default clickable role set when empty. AT-SPI role names
// are canonically lowercase, and both this set and the names read from
// Accessible.GetRoleName are lowercased so a config written with different
// casing still matches.
func rolesSet(roles []string) map[string]struct{} {
	if len(roles) == 0 {
		return defaultClickableRoles
	}

	set := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		if trimmed := strings.TrimSpace(r); trimmed != "" {
			set[strings.ToLower(trimmed)] = struct{}{}
		}
	}

	if len(set) == 0 {
		return defaultClickableRoles
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
