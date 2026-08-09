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

// AT-SPI (D-Bus) accessibility client for Linux: enables assistive-tech mode,
// finds the active window, and walks its tree for clickable elements so hints
// mode works on KDE/Wayland and other AT-SPI desktops.
// It does NOT implement input injection (that stays on the embedded
// InfraAXClient, which routes clicks through wlroots/libei).
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

// atspiRoleFrame is the AT-SPI role of a top-level application window, used
// both as a vocabulary entry and to recognize window elements during frame
// selection.
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

	logger       *zap.Logger
	windowOrigin windowOriginSource

	mu        sync.Mutex
	a11y      *dbus.Conn
	a11yReady bool

	a11yMu    sync.Mutex
	activated bool // true once we enabled AT-SPI this session
	closed    bool // true after Close() runs; prevents re-activation
	savedIsOn bool // original IsEnabled before our first enable
	savedSrOn bool // original ScreenReaderEnabled before our first enable

	// Event-driven active-window cache. A background listener records the AT-SPI
	// window that most recently emitted a window:activate event, so frame
	// selection can skip the registry scan entirely when the cached window still
	// matches the compositor's focused toplevel. See events.go.
	activeMu     sync.Mutex
	activeWindow *atspiActiveWindow // nil until the first window:activate arrives
	eventsReady  bool               // true once the listener is registered on c.a11y
	eventsStop   chan struct{}      // closed to stop the listener goroutine
	eventsClosed bool               // true after Close; blocks starting a new listener
}

// New builds the Linux accessibility client. AT-SPI is not
// activated until ensureA11yEnabled is called (lazily on first hints request),
// so hints-disabled sessions never touch the session-wide a11y status.
//
// AT-SPI itself is the accessibility API on every Linux backend, so this client
// is built on all of them; only the window-origin source is backend-specific,
// and it is chosen here rather than probing for a compositor of its own.
func New(logger *zap.Logger, configProvider config.Provider) *Client {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Client{
		Client:       native.New(logger, configProvider),
		logger:       logger.Named("accessibility.atspi"),
		windowOrigin: newWindowOriginSource(platform.DetectLinuxBackend(), logger),
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

// AllWindows is not used by hints; return empty rather than erroring.
func (c *Client) AllWindows(_ context.Context) ([]ax.Window, error) {
	return nil, nil
}

// ClickableNodes walks the given window for clickable elements.
func (c *Client) ClickableNodes(
	ctx context.Context,
	root ax.Element,
	roles []string,
	_ int,
) ([]ax.Node, error) {
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

	out := make([]ax.Node, 0, atspiClickableNodesCap)
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

// collectViaCollection fetches clickable elements with a single
// org.a11y.atspi.Collection.GetMatches call rooted at the frame, instead of the
// recursive per-node walk. It returns (nodes, true) on success, or (nil, false)
// to signal the caller to fall back to the walk — when Collection is disabled,
// no requested role maps to an AtspiRole, or the toolkit does not implement the
// interface (the GetMatches call errors). Results have identical semantics to
// the walk: SHOWING elements whose role maps (via atspiToAXRole) into roleSet.
func (c *Client) collectViaCollection(
	ctx context.Context,
	conn *dbus.Conn,
	root accRef,
	roleSet map[string]struct{},
	offX, offY int,
) ([]ax.Node, bool) {
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
func (c *Client) materializeMatch(
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
func (c *Client) stateHas(ctx context.Context, conn *dbus.Conn, ref accRef, bit uint) bool {
	states, ok := c.states(ctx, conn, ref)
	if !ok {
		return false
	}

	return atspiStateBit(states, bit)
}

// frameStates reads a frame's ACTIVE and SHOWING state from a single GetState
// call, halving the per-frame state round-trips during frame selection.
// The two results are the ACTIVE and SHOWING states, in that order.
func (c *Client) frameStates(ctx context.Context, conn *dbus.Conn, ref accRef) (bool, bool) {
	states, ok := c.states(ctx, conn, ref)
	if !ok {
		return false, false
	}

	return atspiStateBit(states, atspiStateActive), atspiStateBit(states, atspiStateShowing)
}

// extents returns the on-screen rectangle of an accessible.
func (c *Client) extents(
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

// walk recursively collects clickable, showing nodes under ref.
func (c *Client) walk(
	ctx context.Context,
	conn *dbus.Conn,
	ref accRef,
	roles map[string]struct{},
	depth int,
	out *[]ax.Node,
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
