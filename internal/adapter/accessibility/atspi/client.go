//go:build linux

package atspi

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

	"github.com/y3owk1n/neru/internal/adapter/accessibility/ax"
	"github.com/y3owk1n/neru/internal/adapter/accessibility/native"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/config"
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
	atspiTextIfc       = "org.a11y.atspi.Text"

	// ATSPI_COORD_TYPE_SCREEN and ATSPI_COORD_TYPE_WINDOW: extents relative to
	// the screen origin and to the top-level window.
	atspiCoordScreen = uint32(0)
	atspiCoordWindow = uint32(1)

	// AT-SPI state bit indices (atspi-constants.h). All < 32, so they live in
	// the first uint32 of the state bitfield array.
	atspiStateActive    = 1
	atspiStateEditable  = 7
	atspiStateEnabled   = 8
	atspiStateFocused   = 12
	atspiStateSensitive = 24
	atspiStateShowing   = 25

	// Bit width of the int32 words AT-SPI packs its state and role sets into
	// (one bit per state/role index).
	atspiBitsPerWord = 32

	// Walk bounds that keep D-Bus traffic down on large trees. hints.max_depth
	// bounds the depth. When it is 0 (unlimited), atspiWalkDepthCeiling bounds
	// it instead, so the walk ends in an app whose accessibility tree loops.
	atspiMaxNodes         = 2000
	atspiWalkDepthCeiling = 256

	// atspiCallTimeout bounds one AT-SPI D-Bus round trip. A healthy toolkit
	// answers in milliseconds, so it trips only on a wedged app, and the scan
	// drops that node and continues instead of spending its whole budget.
	atspiCallTimeout = 500 * time.Millisecond

	// atspiPropertiesGet is called with a context so a wedged read cannot block,
	// since godbus GetProperty has no context-aware variant.
	atspiPropertiesGet = "org.freedesktop.DBus.Properties.Get"

	// Capacity hint for the clickable-node slice.
	atspiClickableNodesCap = 128

	// org.a11y.Status D-Bus property names.
	a11yPropIsEnabled    = "IsEnabled"
	a11yPropScreenReader = "ScreenReaderEnabled"

	// Collection.GetMatches fetches every matching descendant in one call.
	atspiCollectionIfc = "org.a11y.atspi.Collection"

	// AtspiCollectionMatchType values (atspi-constants.h). MATCH_ALL requires
	// every listed item, so an empty set matches anything. MATCH_ANY requires one.
	atspiMatchAll = int32(1)
	atspiMatchAny = int32(2)

	// AtspiCollectionSortOrder CANONICAL. Order is irrelevant but must be valid.
	atspiSortCanonical = uint32(1)

	// Fixed word counts libatspi marshals the role/state sets into (see
	// _atspi_match_rule_marshal in at-spi2-core/atspi/atspi-matchrule.c). The
	// role bitfield is always 5 int32 words, the state bitset always 2.
	atspiRoleWords  = 5
	atspiStateWords = 2

	// collectionMaxWorkers caps the goroutines materializing GetMatches results,
	// so a large page cannot flood the a11y bus.
	collectionMaxWorkers = 16
)

// atspiRoleFrame is the AT-SPI role of a top-level application window.
const atspiRoleFrame = "frame"

// Role names that appear both in the ordered list below and in the alias map.
const (
	atspiRolePushButton     = "push button"
	atspiRolePushButtonMenu = "push button menu"
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

// Client is the Linux ax.Client. It walks the AT-SPI tree for hints and
// delegates everything else (input injection, focused-app identity) to the
// embedded InfraAXClient.
type Client struct {
	*native.Client

	logger         *zap.Logger
	configProvider config.Provider
	windowOrigin   windowOriginSource

	// originFailure is the origin source's last failure reason, so a
	// session-long failure warns once. Any scanning goroutine may touch it.
	originFailure atomic.Pointer[string]

	mu        sync.Mutex
	a11y      *dbus.Conn
	a11yReady bool

	a11yMu    sync.Mutex
	activated bool // true once we enabled AT-SPI this session
	closed    bool // true after Close() runs; prevents re-activation
	savedIsOn bool // original IsEnabled before our first enable
	savedSrOn bool // original ScreenReaderEnabled before our first enable

	// titleMatched maps a focused app_id that no AT-SPI application name
	// answers to the bus name whose window last matched it by title, and
	// titleMissed records title scans that found nothing. Both spare
	// findFocusedFrameByTitle from reading every window on every activation.
	titleMatchedMu sync.Mutex
	titleMatched   map[string]string
	titleMissed    map[string]titleMiss

	// Active window as last reported by window:activate, so frame selection can
	// skip the registry scan. See events.go.
	activeMu     sync.Mutex
	activeWindow *atspiActiveWindow // nil until the first window:activate arrives
	eventsReady  bool               // true once the listener is registered on c.a11y
	eventsStop   chan struct{}      // closed to stop the listener goroutine
	eventsClosed bool               // true after Close; blocks starting a new listener
}

// New builds the Linux accessibility client. AT-SPI is activated lazily on
// the first hints request, so hints-disabled sessions never touch the
// session-wide a11y status. Only the window-origin source is backend-specific.
func New(logger *zap.Logger, configProvider config.Provider) *Client {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Client{
		Client:         native.New(logger, configProvider),
		logger:         logger.Named("accessibility.atspi"),
		configProvider: configProvider,
		windowOrigin:   newWindowOriginSource(platform.DetectLinuxBackend(), logger),
	}
}

// FrontmostWindow returns the active top-level window via AT-SPI.
func (c *Client) FrontmostWindow(ctx context.Context) (ax.Window, error) {
	err := c.ensureA11yEnabled()
	if err != nil {
		c.logger.Warn("Failed to enable AT-SPI accessibility", zap.Error(err))
	}

	conn, err := c.ensureA11yConn()
	if err != nil {
		return nil, err
	}

	// Start the window:activate listener once. See events.go.
	c.ensureA11yEvents(conn)

	// The diagnostic lets frame selection report a fatal AT-SPI failure instead
	// of a registry that stopped responding looking like "no active frame".
	diag := &atspiScanDiag{}
	ctx = context.WithValue(ctx, atspiScanDiagKey{}, diag)

	// Read the focused app_id and title once, as one snapshot, for both selection
	// and the window record. A second read could pair a newer window's identity
	// with the old frame and defeat the ClickableNodes stability check.
	focusedAppID, focusedTitle, _ := linux.FocusedAppIdentity(
		platform.DetectLinuxBackend().String(),
	)

	frame, ok := c.findActiveFrame(ctx, conn, focusedAppID, focusedTitle)
	if !ok {
		// Tell "nothing focused" from an AT-SPI failure during selection. A frame
		// that was found is returned even if another app errored mid-scan.
		ferr := diag.fatalErr()
		if ferr != nil {
			return nil, scanFailureError(ferr)
		}

		c.logger.Debug("AT-SPI found no active frame")

		// Nothing focused. An empty window yields no hints rather than an error.
		return &atspiWindow{}, nil
	}

	// The app name is a D-Bus round trip, so it is read only when debug is on.
	if ce := c.logger.Check(zap.DebugLevel, "AT-SPI selected active frame"); ce != nil {
		ce.Write(
			zap.String("bus", frame.Name),
			zap.String("app", c.name(ctx, conn, accRef{Name: frame.Name, Path: atspiRootPath})),
		)
	}

	return &atspiWindow{
		ref:          frame,
		valid:        true,
		focusedAppID: focusedAppID,
		focusedTitle: focusedTitle,
	}, nil
}

// FrontmostAndPopoverWindows returns the active window (popovers are part of
// the same AT-SPI tree, so the single-window walk already covers them).
func (c *Client) FrontmostAndPopoverWindows(ctx context.Context) ([]ax.Window, error) {
	win, err := c.FrontmostWindow(ctx)
	if err != nil {
		return nil, err
	}

	w, ok := win.(*atspiWindow)
	if !ok || !w.valid {
		return nil, nil
	}

	return []ax.Window{win}, nil
}

// AllWindows is not used by hints, so it returns empty rather than erroring.
func (c *Client) AllWindows(_ context.Context) ([]ax.Window, error) {
	return nil, nil
}

// ClickableNodes walks the given window for clickable elements.
func (c *Client) ClickableNodes(
	ctx context.Context,
	root ax.Element,
	roles []string,
	maxDepth int,
	readState bool,
) ([]ax.Node, error) {
	win, ok := root.(*atspiWindow)
	if !ok || !win.valid {
		return nil, nil
	}

	conn, err := c.ensureA11yConn()
	if err != nil {
		return nil, err
	}

	// The diagnostic lets an unresponsive app surface below as a timeout instead
	// of a window with no elements.
	diag := &atspiScanDiag{}
	ctx = context.WithValue(ctx, atspiScanDiagKey{}, diag)

	start := time.Now()

	// If focus changed since selection the frame is stale, so skip the
	// subprocess-spawning geometry query. With no focused app_id (GNOME without
	// the extension) there is nothing to compare and the walk proceeds.
	if !c.focusStableSince(win.focusedAppID, win.focusedTitle) {
		return nil, nil
	}

	place, haveOrigin := c.placementFor(ctx, conn, win)

	// originFor reads the current focused window's geometry. Recheck so a focus
	// change in between cannot pair this frame with another window's origin.
	if !c.focusStableSince(win.focusedAppID, win.focusedTitle) {
		return nil, nil
	}

	depth := c.treeDepth(maxDepth)
	out, path, visited := c.scan(ctx, conn, win.ref, rolesSet(roles),
		c.ignoresClickableCheck(win.focusedAppID), depth, place, readState)

	if c.visibleCheckFor(win.focusedAppID) {
		out = c.visibleNodes(ctx, conn, win.ref, out, place)
	}

	c.logger.Debug("AT-SPI clickable scan complete",
		zap.String("path", path),
		zap.Int("count", len(out)),
		zap.Int("visited", visited),
		zap.Int("max_depth", depth),
		zap.Int("offset_x", place.offset.X),
		zap.Int("offset_y", place.offset.Y),
		zap.Bool("have_origin", haveOrigin),
		zap.Duration("elapsed", time.Since(start)))

	if path == scanPathCollection {
		return out, nil
	}

	// An empty result after fatal read failures (the app timed out, exited, or
	// dropped its a11y connection) is reported, not shown as no hints. A partial
	// result is returned as-is.
	if len(out) == 0 {
		ferr := diag.fatalErr()
		if ferr != nil {
			return nil, scanFailureError(ferr)
		}
	}

	return out, nil
}

// atspiMatchRule mirrors the AT-SPI Collection MatchRule D-Bus struct
// (aiia{ss}iaiiasib), and godbus marshals the exported fields in declaration
// order.
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

// collectionStates returns the states a Collection match requires: SHOWING,
// and ENABLED unless anyState keeps disabled elements too.
func collectionStates(anyState bool) []int32 {
	if anyState {
		return stateBitset(atspiStateShowing)
	}

	return stateBitset(atspiStateShowing, atspiStateEnabled)
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
func (c *Client) Close() error {
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

// placement is how a scan turns an element's AT-SPI extents into screen
// pixels: the coordinate type it reads them in and the offset it adds.
type placement struct {
	coordType uint32
	offset    image.Point
}

// placementFor returns how to place the elements of win, and whether a
// compositor origin was applied.
//
// Without an origin, the scan uses screen coordinates as they are. Those are
// right on X11 and for XWayland, and window-relative for a native Wayland
// client, which is the best Neru can do there.
//
// With an origin, the scan reads window coordinates and adds the origin less
// the frame's own window position, which a client-side-decoration margin makes
// non-zero. GTK 4 needs this because it reports a zero screen origin for every
// element.
//
// The origin is checked against this frame by size and by focused identity
// (KWin compares identities), because another window's origin would misplace
// every hint. Without frame extents it applies none.
func (c *Client) placementFor(
	ctx context.Context,
	conn *dbus.Conn,
	win *atspiWindow,
) (placement, bool) {
	screen := placement{coordType: atspiCoordScreen}

	frameRect, frameOK := c.extents(ctx, conn, win.ref, atspiCoordScreen)
	if !frameOK {
		return screen, false
	}

	origin, originKnown, originErr := c.windowOrigin.originFor(windowFrame{
		Width:        frameRect.Dx(),
		Height:       frameRect.Dy(),
		FocusedAppID: win.focusedAppID,
		FocusedTitle: win.focusedTitle,
	})
	if originErr != nil {
		// A compositor with no origin to give is routine. A source that could not be
		// asked misplaces every hint in this window, so it is reported.
		c.reportOriginFailure(originErr)

		return screen, false
	}

	// The source answered, even with no position to give (a tiled niri window, an
	// unfocused desktop), so a later failure is news again.
	c.clearOriginFailure()

	if !originKnown {
		return screen, false
	}

	frameInWindow, ok := c.extents(ctx, conn, win.ref, atspiCoordWindow)
	if !ok {
		// A toolkit that cannot answer in window coordinates keeps the
		// screen-coordinate placement Neru used before GTK 4 needed this.
		return placement{coordType: atspiCoordScreen, offset: origin.Sub(frameRect.Min)}, true
	}

	return placement{coordType: atspiCoordWindow, offset: origin.Sub(frameInWindow.Min)}, true
}

// The two ways a scan reads the window, as the scan-complete log names them.
const (
	scanPathCollection = "collection"
	scanPathWalk       = "walk"
)

// scan returns the nodes under root whose role is in roles, read no deeper
// than depth (0 for unlimited), and which path read them. visited counts the
// nodes the walk read, and is 0 on the Collection path. anyState keeps an
// element whatever its enabled state, for hints.ignore_clickable_check.
func (c *Client) scan(
	ctx context.Context,
	conn *dbus.Conn,
	root accRef,
	roles map[string]struct{},
	anyState bool,
	depth int,
	place placement,
	readState bool,
) ([]ax.Node, string, int) {
	// Fast path: one Collection.GetMatches call, falling back to the walk when the
	// toolkit lacks Collection. Collection cannot stop at a depth, so a limit below
	// the default goes to the walk.
	if depth == 0 || depth >= config.DefaultMaxDepth {
		nodes, ok := c.collectViaCollection(ctx, conn, root, roles, anyState, place, readState)
		if ok {
			return nodes, scanPathCollection, 0
		}
	}

	limit := depth
	if limit == 0 {
		limit = atspiWalkDepthCeiling
	}

	out := make([]ax.Node, 0, atspiClickableNodesCap)
	visited := 0
	c.walk(ctx, conn, root, roles, anyState, 0, limit, &out, &visited, place, readState)

	return out, scanPathWalk, visited
}

// ignoresClickableCheck reports whether hints.ignore_clickable_check is set
// for appID. A scan then keeps elements of the clickable roles that the app
// reports as disabled.
func (c *Client) ignoresClickableCheck(appID string) bool {
	if c.configProvider == nil {
		return false
	}

	cfg := c.configProvider.Get()

	return cfg != nil && cfg.ShouldIgnoreClickableCheckForApp(appID)
}

// treeDepth returns the deepest level a scan reads, or 0 for unlimited. That
// is maxDepth when the caller sets one, and hints.max_depth otherwise.
func (c *Client) treeDepth(maxDepth int) int {
	if maxDepth > 0 {
		return maxDepth
	}

	if c.configProvider == nil {
		return config.DefaultMaxDepth
	}

	cfg := c.configProvider.Get()
	if cfg == nil {
		return config.DefaultMaxDepth
	}

	return cfg.Hints.MaxDepth
}

// collectViaCollection fetches clickable elements with one
// Collection.GetMatches call rooted at the frame. It returns false to send the
// caller to the walk when Collection is disabled, no requested role maps to an
// AtspiRole, or the toolkit does not implement it. Results match the walk's,
// SHOWING elements (ENABLED too unless anyState) whose role maps into roleSet.
func (c *Client) collectViaCollection(
	ctx context.Context,
	conn *dbus.Conn,
	root accRef,
	roleSet map[string]struct{},
	anyState bool,
	place placement,
	readState bool,
) ([]ax.Node, bool) {
	if os.Getenv("NERU_ATSPI_NO_COLLECTION") != "" {
		return nil, false
	}

	ids := deriveTargetRoleIDs(roleSet)
	if len(ids) == 0 {
		// No requested role has an AtspiRole equivalent, so the walk would also
		// emit nothing. Fall back rather than issue a match-nothing query.
		return nil, false
	}

	rule := atspiMatchRule{
		States:     collectionStates(anyState),
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

		// GetMatches runs on the full scan budget, outside the leaf helpers, so record
		// a scan-fatal error here for an empty fallback walk to surface. A benign
		// "not implemented" error is ignored.
		c.noteCallErr(ctx, err)

		return nil, false
	}

	// An empty result is not authoritative, since a toolkit's Collection may be
	// incomplete or scope the subtree differently. The walk is the safe baseline
	// and cheap on an empty window.
	if len(matches) == 0 {
		return nil, false
	}

	// Materialize matches concurrently over the one multiplexed connection. The
	// semaphore caps workers at collectionMaxWorkers, and each writes its own
	// slot, so no lock is needed.
	results := make([]*atspiNode, len(matches))

	sem := make(chan struct{}, collectionMaxWorkers)

	var (
		waitGroup  sync.WaitGroup
		validCount atomic.Int64
	)

	for index, ref := range matches {
		// Cap on valid nodes, not raw candidates, so invalid matches cannot crowd out
		// valid ones. validCount lags under concurrency, and the output loop trims any
		// excess.
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

			node := c.materializeMatch(ctx, conn, ref, roleSet, place, readState)
			if node != nil {
				results[slot] = node

				validCount.Add(1)
			}
		}(index, ref)
	}

	waitGroup.Wait()

	out := make([]ax.Node, 0, len(results))

	for _, node := range results {
		if node == nil {
			continue
		}

		out = append(out, node)
		if len(out) >= atspiMaxNodes {
			break
		}
	}

	// Every match failed to materialize (a dynamic UI invalidated them), so fall
	// back to the walk rather than report an empty success.
	if len(out) == 0 {
		return nil, false
	}

	return out, true
}

// materializeMatch turns a GetMatches result into an atspiNode, or nil where
// the walk's filter would drop it. It only reads over the shared multiplexed
// connection, so it is safe for concurrent use.
func (c *Client) materializeMatch(
	ctx context.Context,
	conn *dbus.Conn,
	ref accRef,
	roleSet map[string]struct{},
	place placement,
	readState bool,
) *atspiNode {
	// A toolkit may return a role we did not ask for.
	roleName := strings.ToLower(c.roleName(ctx, conn, ref))
	if !candidateRole(roleName, roleSet) {
		return nil
	}

	// The match rule cannot tell editable text from read-only text, so read
	// the element's state to tell them apart.
	if roleName == atspiRoleText {
		states, ok := c.states(ctx, conn, ref)
		if !ok {
			return nil
		}

		roleName = effectiveRole(roleName, states)
		if _, ok := roleSet[roleName]; !ok {
			return nil
		}
	}

	return c.readNode(ctx, conn, ref, roleName, place, readState)
}

// D-Bus errors meaning the scan cannot progress, because the app left the bus
// or its connection is gone. UnknownMethod (GetChildren has a fallback) and
// stale-node errors are excluded so a healthy or empty window is never
// misreported as a failure.
const (
	atspiErrServiceUnknown = "org.freedesktop.DBus.Error.ServiceUnknown"
	atspiErrNoReply        = "org.freedesktop.DBus.Error.NoReply"
	atspiErrDisconnected   = "org.freedesktop.DBus.Error.Disconnected"
)

// children returns the AT-SPI children of an accessible. It prefers the bulk
// GetChildren method and falls back to ChildCount + GetChildAtIndex for older
// toolkits that do not expose GetChildren.
func (c *Client) children(ctx context.Context, conn *dbus.Conn, ref accRef) []accRef {
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

// name returns the accessible Name property (used as the element title).
func (c *Client) name(ctx context.Context, conn *dbus.Conn, ref accRef) string {
	callCtx, cancel := context.WithTimeout(ctx, atspiCallTimeout)
	defer cancel()

	return c.propertyString(ctx, conn.Object(ref.Name, ref.Path).
		GoWithContext(callCtx, atspiPropertiesGet, 0, nil, atspiAccessibleIfc, "Name"))
}

// atspiValueRoles are the roles whose text becomes the element's value, as
// AXValue does on macOS. effectiveRole maps GTK 3 editable text, a multi-line
// view included, to one of them. Password text is left out so a password
// never leaves the app.
var atspiValueRoles = map[string]struct{}{atspiRoleEntry: {}, "spin button": {}}

// readNode reads a matched element's bounds as place says, its title,
// description and, for a role in atspiValueRoles, its value. readState adds
// the identifier and focused state, which only `neru query hints` reports.
// It returns nil for an element with no on-screen size. The reads go out
// together and godbus multiplexes them, so they cost one round trip.
func (c *Client) readNode(
	ctx context.Context,
	conn *dbus.Conn,
	ref accRef,
	roleName string,
	place placement,
	readState bool,
) *atspiNode {
	callCtx, cancel := context.WithTimeout(ctx, atspiCallTimeout)
	defer cancel()

	obj := conn.Object(ref.Name, ref.Path)
	property := func(name string) *dbus.Call {
		return obj.GoWithContext(callCtx, atspiPropertiesGet, 0, nil, atspiAccessibleIfc, name)
	}

	extentsCall := obj.GoWithContext(
		callCtx,
		atspiComponentIfc+".GetExtents",
		0,
		nil,
		place.coordType,
	)
	nameCall := property("Name")
	descriptionCall := property("Description")

	var valueCall, identifierCall, stateCall *dbus.Call
	if _, ok := atspiValueRoles[roleName]; ok {
		valueCall = obj.GoWithContext(callCtx, atspiTextIfc+".GetText", 0, nil, int32(0), int32(-1))
	}

	if readState {
		identifierCall = property("AccessibleId")
		stateCall = obj.GoWithContext(callCtx, atspiAccessibleIfc+".GetState", 0, nil)
	}

	rect, valid := c.extentsFrom(ctx, extentsCall)

	node := &atspiNode{
		ref:         ref,
		id:          string(ref.Path) + "@" + ref.Name,
		role:        roleName,
		rect:        rect.Add(place.offset),
		title:       c.propertyString(ctx, nameCall),
		description: c.propertyString(ctx, descriptionCall),
	}

	if valueCall != nil {
		c.callStore(ctx, valueCall, &node.value)
	}

	if readState {
		node.identifier = c.propertyString(ctx, identifierCall)

		var states []uint32
		if c.callStore(ctx, stateCall, &states) {
			node.focused = atspiStateBit(states, atspiStateFocused)
		}
	}

	if !valid {
		return nil
	}

	return node
}

// callStore waits for call and stores its reply in out, reporting whether it
// could.
func (c *Client) callStore(ctx context.Context, call *dbus.Call, out any) bool {
	<-call.Done

	err := call.Store(out)
	if err != nil {
		c.noteCallErr(ctx, err)

		return false
	}

	return true
}

// propertyString waits for a Properties.Get call and returns its string, or
// "" when the read failed or the property is not a string.
func (c *Client) propertyString(ctx context.Context, call *dbus.Call) string {
	var val dbus.Variant
	if !c.callStore(ctx, call, &val) {
		return ""
	}

	s, _ := val.Value().(string)

	return s
}

// states reads the AT-SPI state bitfield of an accessible in one GetState
// round trip. Callers pull individual bits with atspiStateBit.
func (c *Client) states(ctx context.Context, conn *dbus.Conn, ref accRef) ([]uint32, bool) {
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

// showingRole returns the element's role read through effectiveRole, and
// whether the element is showing, enabled and has that role in roles. A
// disabled element gets no hint, as on macOS, since clicking it does nothing.
// anyState skips the enabled test, for hints.ignore_clickable_check.
func (c *Client) showingRole(
	ctx context.Context,
	conn *dbus.Conn,
	ref accRef,
	roles map[string]struct{},
	anyState bool,
	roleName string,
) (string, bool) {
	if !candidateRole(roleName, roles) {
		return roleName, false
	}

	states, read := c.states(ctx, conn, ref)
	if !read || !atspiStateBit(states, atspiStateShowing) ||
		(!anyState && !atspiEnabled(states)) {
		return roleName, false
	}

	roleName = effectiveRole(roleName, states)
	_, wanted := roles[roleName]

	return roleName, wanted
}

// atspiEnabled reports whether an element takes input. GTK 4 sets SENSITIVE
// and never ENABLED, where other toolkits set both, so either one counts.
// Collection asks for ENABLED alone, which is safe because GTK 4 does not
// implement Collection and always reaches the walk.
func atspiEnabled(states []uint32) bool {
	return atspiStateBit(states, atspiStateEnabled) || atspiStateBit(states, atspiStateSensitive)
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

// frameStates reads a frame's ACTIVE and SHOWING state, in that order, from
// one GetState call.
func (c *Client) frameStates(ctx context.Context, conn *dbus.Conn, ref accRef) (bool, bool) {
	states, ok := c.states(ctx, conn, ref)
	if !ok {
		return false, false
	}

	return atspiStateBit(states, atspiStateActive), atspiStateBit(states, atspiStateShowing)
}

// extents returns the rectangle of an accessible in coordType.
func (c *Client) extents(
	ctx context.Context,
	conn *dbus.Conn,
	ref accRef,
	coordType uint32,
) (image.Rectangle, bool) {
	callCtx, cancel := context.WithTimeout(ctx, atspiCallTimeout)
	defer cancel()

	return c.extentsFrom(ctx, conn.Object(ref.Name, ref.Path).
		GoWithContext(callCtx, atspiComponentIfc+".GetExtents", 0, nil, coordType))
}

// extentsFrom waits for a GetExtents call and returns the rectangle, or false
// when the read failed or the element has no size.
func (c *Client) extentsFrom(ctx context.Context, call *dbus.Call) (image.Rectangle, bool) {
	var ext atspiExtents
	if !c.callStore(ctx, call, &ext) || ext.W <= 0 || ext.H <= 0 {
		return image.Rectangle{}, false
	}

	return image.Rect(int(ext.X), int(ext.Y), int(ext.X+ext.W), int(ext.Y+ext.H)), true
}

// walk recursively collects clickable, showing nodes under ref.
func (c *Client) walk(
	ctx context.Context,
	conn *dbus.Conn,
	ref accRef,
	roles map[string]struct{},
	anyState bool,
	depth int,
	maxDepth int,
	out *[]ax.Node,
	visited *int,
	place placement,
	readState bool,
) {
	if depth > maxDepth || len(*out) >= atspiMaxNodes {
		return
	}

	if ctx.Err() != nil {
		return
	}

	*visited++

	// Both sides are native AT-SPI role names, since configured roles are resolved
	// before they reach the client. One state read covers showing and, for text,
	// editable.
	roleName, matched := c.showingRole(ctx, conn, ref, roles, anyState,
		strings.ToLower(c.roleName(ctx, conn, ref)))

	if matched {
		node := c.readNode(ctx, conn, ref, roleName, place, readState)
		if node != nil {
			*out = append(*out, node)
		}
	}

	for _, child := range c.children(ctx, conn, ref) {
		c.walk(ctx, conn, child, roles, anyState, depth+1, maxDepth, out, visited, place, readState)
	}
}
