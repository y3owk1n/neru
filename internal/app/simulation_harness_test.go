package app_test

// Full-loop simulation harness: builds the real application via app.New with
// every OS port replaced by an in-process fake, then drives it the way the
// native event tap would — one key string at a time through HandleKeyPress —
// and observes what a user would observe: hints and grids drawn on the
// (recording) overlay manager, cursor moves on the system port, and clicks,
// scrolls and element actions on the accessibility port.
//
// Everything between those edges — hotkey resolution, mode transitions, hint
// generation and matching, grid math, services, locking — is the production
// code path. Nothing here touches native APIs, so these tests are
// deterministic and platform-neutral.

import (
	"context"
	"errors"
	"image"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
	"github.com/y3owk1n/neru/internal/ports/mocks"
)

const (
	simFixtureBundleID = "com.example.simfixture"
	simWaitTimeout     = 5 * time.Second
	simPollInterval    = 2 * time.Millisecond
)

// simScreen is the fixture display: a single 1920x1080 screen at the global
// origin, so screen-local and global coordinates coincide.
var simScreen = image.Rect(0, 0, 1920, 1080)

// simElement builds a clickable fixture element or fails the test.
func simElement(
	tb testing.TB,
	elementID string,
	bounds image.Rectangle,
	title string,
) *element.Element {
	tb.Helper()

	elem, err := element.NewElement(
		element.ID(elementID),
		bounds,
		element.Role("button"),
		element.WithClickable(true),
		element.WithTitle(title),
	)
	if err != nil {
		tb.Fatalf("failed to build fixture element %q: %v", elementID, err)
	}

	return elem
}

// --- recording overlay port -----------------------------------------------

// simGridDraw is one grid drawn on the overlay: the grid itself and the input
// it was drawn narrowed to.
type simGridDraw struct {
	grid  *domainGrid.Grid
	input string
}

// simOverlayPort is the screen the journeys observe. It implements
// ports.OverlayPort outright — no embedding, no inherited silent success — so
// a call production makes that this fake forgot is a compile error rather than
// a journey that passes without it.
//
// It records domain values, never call sequences: which calls an adapter would
// use to realize a Frame is exactly what the Frame port exists to be free to
// change. What it models instead is what a user would see — which mode has
// content on screen, whether the overlay is up, what was drawn on it.
type simOverlayPort struct {
	// refuseMonitorSelect stands in for a backend with no surface for the
	// monitor picker: showing that frame reports CodeNotSupported, the way an
	// adapter over a backend without the capability does.
	refuseMonitorSelect bool

	mu sync.Mutex
	// visible is whether the overlay is on screen; shows counts how many times
	// a frame was put there. A journey needs both: entering a mode shows the
	// overlay once, and every keystroke after it must redraw without showing
	// again (ADR 0003).
	visible bool
	shows   int
	// drawnModes is which modes have content on screen right now, tracked the
	// way a display would: showing a frame replaces whatever was on screen with
	// that frame's content, redrawing repaints it, and clearing empties the
	// screen. It is what lets a journey say what a user would see rather than
	// which method was called.
	drawnModes map[domain.Mode]bool

	hintFrames         []ports.HintsFrame
	gridDraws          []simGridDraw
	recursiveGridDraws []image.Rectangle
	monitorDraws       [][]ports.MonitorSelectTarget

	matchPrefixes []string
	hideUnmatched []bool
	subgridCells  []*domainGrid.Cell
	searchQueries []string
	stickySymbols []string

	// indicatorVisible is the visibility each indicator was last asked for.
	// An indicator draws through its own call rather than a frame, so this —
	// not a frame — is what a journey can observe about one being on screen.
	indicatorVisible map[ports.Indicator]bool

	// appliedConfigs are the configurations a reload handed the overlay, and
	// styleRefreshes how many times a theme change asked it to re-resolve
	// against the one it already held. The overlay owns config + theme ->
	// Style, so at this seam that notification is what "the change reached the
	// overlay" means.
	appliedConfigs []*config.Config
	styleRefreshes int
}

var _ ports.OverlayPort = (*simOverlayPort)(nil)

func (m *simOverlayPort) Health(_ context.Context) error { return nil }

// ShowFrame puts a frame on screen: the overlay comes up and the frame's
// content replaces whatever was there.
func (m *simOverlayPort) ShowFrame(_ context.Context, frame ports.Frame) error {
	if _, isPicker := frame.(ports.MonitorSelectFrame); isPicker && m.refuseMonitorSelect {
		return derrors.New(derrors.CodeNotSupported, "no monitor picker on this backend")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.visible = true
	m.shows++
	clear(m.drawnModes)
	m.recordLocked(frame)

	return nil
}

// RedrawFrame repaints a frame already on screen, without the window sequence.
func (m *simOverlayPort) RedrawFrame(_ context.Context, frame ports.Frame) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.recordLocked(frame)

	return nil
}

// ClearFrame empties the screen and takes the overlay down.
func (m *simOverlayPort) ClearFrame(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.visible = false
	clear(m.drawnModes)

	return nil
}

func (m *simOverlayPort) SetActiveScreen(_ image.Rectangle) {}

func (m *simOverlayPort) DrawHintSearch(search ports.HintSearch) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.searchQueries = append(m.searchQueries, search.Query)

	return nil
}

func (m *simOverlayPort) HideHintSearch() {}

func (m *simOverlayPort) HintSearchBounds(_ image.Rectangle) image.Rectangle {
	return image.Rectangle{}
}

// UpdateGridMatches records the prefix the grid was narrowed to. This is the
// per-keystroke path in grid mode: a journey asserts the user's typing reached
// the overlay here, not that the whole grid was drawn again.
func (m *simOverlayPort) UpdateGridMatches(prefix string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.matchPrefixes = append(m.matchPrefixes, prefix)
}

// SetGridHideUnmatched records whether unmatched cells were asked to disappear.
func (m *simOverlayPort) SetGridHideUnmatched(hide bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.hideUnmatched = append(m.hideUnmatched, hide)
}

// ShowGridSubgrid records the cell a subgrid was opened inside.
func (m *simOverlayPort) ShowGridSubgrid(cell *domainGrid.Cell) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.subgridCells = append(m.subgridCells, cell)
}

func (m *simOverlayPort) UpdateGridPointer(_ domain.Mode, _ ports.GridPointer) {}

func (m *simOverlayPort) DrawModeIndicator(_, _ int) {}

func (m *simOverlayPort) DrawStickyModifiersIndicator(_, _ int, symbols string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stickySymbols = append(m.stickySymbols, symbols)
}

func (m *simOverlayPort) DrawVirtualPointer(_, _ int) {}

func (m *simOverlayPort) DrawMouseActionIndicator(
	_ image.Point,
	_ ports.MouseActionIndicatorStyle,
) {
}

func (m *simOverlayPort) ShowIndicator(indicator ports.Indicator) {
	m.setIndicatorVisible(indicator, true)
}

func (m *simOverlayPort) HideIndicator(indicator ports.Indicator) {
	m.setIndicatorVisible(indicator, false)
}

func (m *simOverlayPort) ResizeIndicatorToActiveScreen(_ ports.Indicator) {}

func (m *simOverlayPort) Flush() {}

func (m *simOverlayPort) IsVisible() bool { return m.isVisible() }

func (m *simOverlayPort) Refresh(_ context.Context) error { return nil }

func (m *simOverlayPort) ApplyConfig(cfg *config.Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.appliedConfigs = append(m.appliedConfigs, cfg)
}

func (m *simOverlayPort) RefreshStyles() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.styleRefreshes++
}

func (m *simOverlayPort) SetHiddenInScreenShare(_ bool) {}

// SetKeyboardCaptureEnabled does nothing, and no journey can make it fire: the
// caller is gated on the event tap reporting that the overlay holds the
// keyboard, which only the Linux evdev tap does and the simulated tap never
// does. It is spelled out rather than inherited so that stays a stated fact
// about the fixture rather than a method silently swallowing a call.
func (m *simOverlayPort) SetKeyboardCaptureEnabled(_ bool) {}

func (m *simOverlayPort) Destroy() {}

// recordLocked stores what a frame put on screen. The caller holds m.mu.
func (m *simOverlayPort) recordLocked(frame ports.Frame) {
	switch drawn := frame.(type) {
	case ports.HintsFrame:
		m.hintFrames = append(m.hintFrames, drawn)
	case ports.GridFrame:
		m.gridDraws = append(m.gridDraws, simGridDraw{grid: drawn.Grid, input: drawn.Input})
	case ports.RecursiveGridFrame:
		m.recursiveGridDraws = append(m.recursiveGridDraws, drawn.Bounds)
	case ports.MonitorSelectFrame:
		m.monitorDraws = append(m.monitorDraws, slices.Clone(drawn.Targets))
	case ports.ScrollFrame:
		// Scroll draws nothing: it is a mode the indicators name rather than a
		// surface with content, so it leaves the screen empty.
		return
	}

	if m.drawnModes == nil {
		m.drawnModes = make(map[domain.Mode]bool)
	}

	m.drawnModes[frame.Mode()] = true
}

func (m *simOverlayPort) setIndicatorVisible(indicator ports.Indicator, visible bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.indicatorVisible == nil {
		m.indicatorVisible = make(map[ports.Indicator]bool)
	}

	m.indicatorVisible[indicator] = visible
}

// drawnModeNames reports every mode with content on screen right now, sorted
// so a failure message reads the same way twice.
func (m *simOverlayPort) drawnModeNames() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	names := make([]string, 0, len(m.drawnModes))
	for mode := range m.drawnModes {
		names = append(names, domain.ModeString(mode))
	}

	sort.Strings(names)

	return names
}

// indicatorVisibility reports the visibility an indicator was last asked for,
// and whether it was ever asked at all.
func (m *simOverlayPort) indicatorVisibility(indicator ports.Indicator) (bool, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	visible, asked := m.indicatorVisible[indicator]

	return visible, asked
}

func (m *simOverlayPort) searchInputDrawCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.searchQueries)
}

func (m *simOverlayPort) lastMonitorTargets() []ports.MonitorSelectTarget {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.monitorDraws) == 0 {
		return nil
	}

	return m.monitorDraws[len(m.monitorDraws)-1]
}

func (m *simOverlayPort) stickyIndicatorDrawCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.stickySymbols)
}

func (m *simOverlayPort) lastRecursiveGridBounds() (image.Rectangle, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.recursiveGridDraws) == 0 {
		return image.Rectangle{}, false
	}

	return m.recursiveGridDraws[len(m.recursiveGridDraws)-1], true
}

// showCount reports how many times a frame was put on screen with the window
// sequence, which is what a keystroke must not pay for.
func (m *simOverlayPort) showCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.shows
}

func (m *simOverlayPort) isVisible() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.visible
}

func (m *simOverlayPort) hintDrawCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.hintFrames)
}

// lastHintLabels returns the labels of the most recent hints frame.
func (m *simOverlayPort) lastHintLabels() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.hintFrames) == 0 {
		return nil
	}

	last := m.hintFrames[len(m.hintFrames)-1]

	labels := make([]string, len(last.Hints))
	for i, h := range last.Hints {
		labels[i] = h.Label()
	}

	return labels
}

func (m *simOverlayPort) lastGrid() *domainGrid.Grid {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.gridDraws) == 0 {
		return nil
	}

	return m.gridDraws[len(m.gridDraws)-1].grid
}

// gridDrawCount reports how many times the whole grid was drawn. A journey
// uses it to pin what a keystroke costs: narrowing must not redraw the grid.
func (m *simOverlayPort) gridDrawCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.gridDraws)
}

// lastMatchPrefix reports the prefix the grid was last narrowed to, and
// whether it was ever narrowed at all.
func (m *simOverlayPort) lastMatchPrefix() (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.matchPrefixes) == 0 {
		return "", false
	}

	return m.matchPrefixes[len(m.matchPrefixes)-1], true
}

// lastHideUnmatched reports whether unmatched cells were last asked to hide.
func (m *simOverlayPort) lastHideUnmatched() (bool, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.hideUnmatched) == 0 {
		return false, false
	}

	return m.hideUnmatched[len(m.hideUnmatched)-1], true
}

// subgridCount reports how many subgrids were opened.
func (m *simOverlayPort) subgridCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.subgridCells)
}

// recursiveGridDrawCount reports how many times the recursive grid was drawn.
func (m *simOverlayPort) recursiveGridDrawCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.recursiveGridDraws)
}

// lastAppliedConfig returns the configuration a reload last handed the
// overlay, and whether one ever reached it.
func (m *simOverlayPort) lastAppliedConfig() (*config.Config, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.appliedConfigs) == 0 {
		return nil, false
	}

	return m.appliedConfigs[len(m.appliedConfigs)-1], true
}

// styleRefreshCount reports how many times a theme change asked the overlay to
// re-resolve its Styles.
func (m *simOverlayPort) styleRefreshCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.styleRefreshes
}

// --- scripted accessibility port ------------------------------------------

// simClick is one recorded point action (click, mouse down, ...).
type simClick struct {
	action    action.Type
	point     image.Point
	modifiers action.Modifiers
}

// simAXPort plays the role of the OS accessibility tree: it serves a fixed
// set of fixture elements and records every action the app performs.
type simAXPort struct {
	mu             sync.Mutex
	elements       []*element.Element
	excluded       bool
	clicks         []simClick
	elementActions []action.Type
	scrolls        []image.Point
	releases       int
}

var _ ports.AccessibilityPort = (*simAXPort)(nil)

func (a *simAXPort) Health(_ context.Context) error { return nil }

func (a *simAXPort) ClickableElements(
	_ context.Context,
	_ ports.ElementFilter,
) ([]*element.Element, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make([]*element.Element, len(a.elements))
	copy(out, a.elements)

	return out, nil
}

func (a *simAXPort) UpdateClickableRoles(_ []string) {}

func (a *simAXPort) PerformAction(
	_ context.Context,
	_ *element.Element,
	actionType action.Type,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.elementActions = append(a.elementActions, actionType)

	return nil
}

func (a *simAXPort) PerformActionAtPoint(
	_ context.Context,
	actionType action.Type,
	point image.Point,
	modifiers action.Modifiers,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.clicks = append(a.clicks, simClick{action: actionType, point: point, modifiers: modifiers})

	return nil
}

func (a *simAXPort) Scroll(_ context.Context, deltaX, deltaY int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.scrolls = append(a.scrolls, image.Point{X: deltaX, Y: deltaY})

	return nil
}

func (a *simAXPort) ReleaseHeldButtons(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.releases++

	return nil
}

func (a *simAXPort) FocusedAppBundleID(_ context.Context) (string, error) {
	return simFixtureBundleID, nil
}

func (a *simAXPort) IsAppExcluded(_ context.Context, _ string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.excluded
}

func (a *simAXPort) PrimeApplication(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (a *simAXPort) setExcluded(excluded bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.excluded = excluded
}

func (a *simAXPort) recordedClicks() []simClick {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make([]simClick, len(a.clicks))
	copy(out, a.clicks)

	return out
}

func (a *simAXPort) recordedScrolls() []image.Point {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make([]image.Point, len(a.scrolls))
	copy(out, a.scrolls)

	return out
}

func (a *simAXPort) releaseCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.releases
}

// --- hotkey backend fake ---------------------------------------------------

// simHotkeyPort stands in for the OS global-hotkey backend. The app registers
// chord -> callback through it; the harness "presses" a chord by invoking the
// stored callback, exactly as the native backend does.
type simHotkeyPort struct {
	mu        sync.Mutex
	nextID    ports.HotkeyID
	callbacks map[ports.HotkeyID]ports.HotkeyCallback
	keys      map[ports.HotkeyID]string
}

var _ ports.HotkeyPort = (*simHotkeyPort)(nil)

func (p *simHotkeyPort) Register(
	keyString string,
	callback ports.HotkeyCallback,
) (ports.HotkeyID, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.callbacks == nil {
		p.callbacks = make(map[ports.HotkeyID]ports.HotkeyCallback)
		p.keys = make(map[ports.HotkeyID]string)
	}

	p.nextID++
	p.callbacks[p.nextID] = callback
	p.keys[p.nextID] = keyString

	return p.nextID, nil
}

func (p *simHotkeyPort) Unregister(hotkeyID ports.HotkeyID) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.callbacks, hotkeyID)
	delete(p.keys, hotkeyID)
}

func (p *simHotkeyPort) UnregisterAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.callbacks = nil
	p.keys = nil
}

// callbackFor returns the callback registered for the given canonical chord.
func (p *simHotkeyPort) callbackFor(keyString string) ports.HotkeyCallback {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, key := range p.keys {
		if key == keyString {
			return p.callbacks[id]
		}
	}

	return nil
}

// --- cursor recorder -------------------------------------------------------

// simCursor tracks the virtual cursor the system port exposes.
type simCursor struct {
	mu    sync.Mutex
	pos   image.Point
	moves []image.Point
}

func (c *simCursor) moveTo(p image.Point) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.pos = p
	c.moves = append(c.moves, p)
}

func (c *simCursor) position() image.Point {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.pos
}

func (c *simCursor) moveCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.moves)
}

// --- harness ---------------------------------------------------------------

type simHarness struct {
	t   testing.TB
	app *app.App
	// appearance is the fixture desktop's light/dark state, read by the
	// system port the app resolves its theme through.
	appearance *atomic.Bool
	overlay    *simOverlayPort
	ax         *simAXPort
	cursor     *simCursor
	hotkeys    *simHotkeyPort
	tap        *mocks.MockEventTapPort
	runDone    chan error
}

// switchToDarkMode flips the fixture desktop's appearance and notifies the app
// the way a platform theme observer does.
func (h *simHarness) switchToDarkMode() {
	h.appearance.Store(true)
	h.app.HandleThemeChange(true)
}

// simConfig returns the default config with the standard mode bindings set
// explicitly; tests mutate it before newSimHarness.
//
// The bindings cannot come from platform defaults: Linux deliberately ships
// with an empty [hotkeys] table (config_linux.go) because chords like
// Ctrl+Shift+C collide with common application shortcuts there. The journeys
// test the binding machinery, not each platform's default binding content, so
// they declare their own.
func simConfig() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Hotkeys.Bindings = map[string][]string{
		hintsHotkey:         {"hints"},
		gridHotkey:          {"grid"},
		recursiveGridHotkey: {"recursive_grid"},
		scrollHotkey:        {"scroll"},
	}

	return cfg
}

// simDisplay is one monitor of the simulated desktop.
type simDisplay struct {
	name   string
	bounds image.Rectangle
}

// newSimHarness builds and starts the real app wired to the simulation fakes
// on a single default display.
func newSimHarness(
	tb testing.TB,
	cfg *config.Config,
	elements []*element.Element,
) *simHarness {
	tb.Helper()

	return newSimHarnessWithDisplays(tb, cfg, elements, []simDisplay{
		{name: "SimDisplay", bounds: simScreen},
	})
}

// newSimHarnessWithDisplays builds and starts the real app on a simulated
// desktop with the given displays, and registers cleanup that stops it and
// fails the test on unclean shutdown. The cursor starts on the first display.
func newSimHarnessWithDisplays(
	tb testing.TB,
	cfg *config.Config,
	elements []*element.Element,
	displays []simDisplay,
) *simHarness {
	tb.Helper()

	recorder := &simOverlayPort{}

	return buildSimHarness(tb, cfg, elements, displays, recorder, recorder)
}

// newSimHarnessHeadlessOverlay builds the app on an overlay with no surface
// for the monitor picker, standing in for a backend that cannot draw it. The
// returned harness has a nil overlay recorder — only for journeys asserting
// that a capability is refused.
func newSimHarnessHeadlessOverlay(
	tb testing.TB,
	cfg *config.Config,
	displays []simDisplay,
) *simHarness {
	tb.Helper()

	return buildSimHarness(
		tb,
		cfg,
		nil,
		displays,
		&simOverlayPort{refuseMonitorSelect: true},
		nil,
	)
}

// buildSimHarness wires the app with the given overlay port; recorder is that
// same port when the journey reads from it, nil otherwise.
func buildSimHarness(
	tb testing.TB,
	cfg *config.Config,
	elements []*element.Element,
	displays []simDisplay,
	overlayPort ports.OverlayPort,
	recorder *simOverlayPort,
) *simHarness {
	tb.Helper()

	if len(displays) == 0 {
		tb.Fatal("buildSimHarness needs at least one display")
	}

	axPort := &simAXPort{elements: elements}
	appearance := &atomic.Bool{}
	cursor := &simCursor{pos: displays[0].bounds.Min.Add(
		image.Point{X: displays[0].bounds.Dx() / 2, Y: displays[0].bounds.Dy() / 2},
	)}
	hotkeys := &simHotkeyPort{}
	tap := &mocks.MockEventTapPort{}

	// The active screen is wherever the cursor currently is, matching how the
	// real system port resolves it.
	activeBounds := func() image.Rectangle {
		pos := cursor.position()
		for _, display := range displays {
			if pos.In(display.bounds) {
				return display.bounds
			}
		}

		return displays[0].bounds
	}

	system := &mocks.MockSystemPort{
		IsDarkModeFunc: func() bool {
			return appearance.Load()
		},
		ScreenBoundsFunc: func(_ context.Context) (image.Rectangle, error) {
			return activeBounds(), nil
		},
		ScreenBoundsByNameFunc: func(_ context.Context, name string) (image.Rectangle, bool, error) {
			for _, display := range displays {
				if display.name == name {
					return display.bounds, true, nil
				}
			}

			return image.Rectangle{}, false, nil
		},
		ScreenNamesFunc: func(_ context.Context) ([]string, error) {
			names := make([]string, len(displays))
			for idx, display := range displays {
				names[idx] = display.name
			}

			return names, nil
		},
		FocusedWindowBoundsFunc: func(_ context.Context) (image.Rectangle, bool, error) {
			return activeBounds(), true, nil
		},
		MoveCursorToPointFunc: func(_ context.Context, point image.Point, _ bool) error {
			cursor.moveTo(point)

			return nil
		},
		CursorPositionFunc: func(_ context.Context) (image.Point, error) {
			return cursor.position(), nil
		},
	}

	application, err := app.New(
		app.WithConfig(cfg),
		app.WithConfigPath(""),
		app.WithLogger(zap.NewNop()),
		app.WithEventTap(tap),
		app.WithTextInput(&mocks.MockTextInputPort{}),
		app.WithIPCServer(&mocks.MockIPCPort{}),
		app.WithWatcher(&mocks.MockAppWatcherPort{}),
		app.WithHotkeyService(hotkeys),
		app.WithOverlayPort(overlayPort),
		app.WithSystemPort(system),
		app.WithAccessibility(axPort),
	)
	if err != nil {
		tb.Fatalf("app.New failed: %v", err)
	}

	sim := &simHarness{
		t:          tb,
		app:        application,
		appearance: appearance,
		overlay:    recorder,
		ax:         axPort,
		cursor:     cursor,
		hotkeys:    hotkeys,
		tap:        tap,
		runDone:    make(chan error, 1),
	}

	go func() {
		sim.runDone <- application.Run()
	}()

	tb.Cleanup(func() {
		application.Stop()

		select {
		case runErr := <-sim.runDone:
			if runErr != nil && !errors.Is(runErr, context.Canceled) &&
				!derrors.IsCode(runErr, derrors.CodeContextCanceled) {
				tb.Errorf("Run() returned an unexpected error after Stop(): %v", runErr)
			}
		case <-time.After(simWaitTimeout):
			tb.Errorf("app did not stop within %v", simWaitTimeout)
		}

		application.Cleanup()
	})

	sim.waitFor("app running", application.IsEnabled)

	return sim
}

// press feeds key strings to the app exactly as the native event tap would.
func (h *simHarness) press(keys ...string) {
	for _, key := range keys {
		h.app.HandleKeyPress(key)
	}
}

// pressHotkey fires a global hotkey chord the way the OS backend does: it
// waits for the app to register the (platform-canonicalized) chord on the
// hotkey port, then invokes the registered callback.
func (h *simHarness) pressHotkey(binding string) {
	h.t.Helper()

	canonical := config.CanonicalHotkeyForPlatform(binding)

	var callback ports.HotkeyCallback

	h.waitFor("hotkey "+canonical+" registered", func() bool {
		callback = h.hotkeys.callbackFor(canonical)

		return callback != nil
	})

	callback()
}

// typeLabel presses a hint or grid label one character at a time.
func (h *simHarness) typeLabel(label string) {
	for _, r := range strings.ToLower(label) {
		h.press(string(r))
	}
}

// waitFor polls cond until it holds or the harness timeout elapses.
func (h *simHarness) waitFor(desc string, cond func() bool) {
	h.t.Helper()

	deadline := time.Now().Add(simWaitTimeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(simPollInterval)
	}

	h.t.Fatalf("timed out after %v waiting for %s", simWaitTimeout, desc)
}

// waitMode waits for the app to reach the given mode.
func (h *simHarness) waitMode(mode domain.Mode) {
	h.t.Helper()

	h.waitFor("mode "+domain.ModeString(mode), func() bool {
		return h.app.CurrentMode() == mode
	})
}

// neverMode asserts the app does not enter the given mode within the window.
// Absence can only be checked for a bounded time; the window is generous
// relative to the (in-process, no-IO) activation path it guards against.
func (h *simHarness) neverMode(mode domain.Mode, window time.Duration) {
	h.t.Helper()

	deadline := time.Now().Add(window)
	for time.Now().Before(deadline) {
		if h.app.CurrentMode() == mode {
			h.t.Fatalf("app unexpectedly entered mode %s", domain.ModeString(mode))
		}

		time.Sleep(simPollInterval)
	}
}
