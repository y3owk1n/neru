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
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
	"github.com/y3owk1n/neru/internal/ports/mocks"
)

const simFixtureBundleID = "com.example.simfixture"

// --- wait budgets ----------------------------------------------------------
//
// Every journey below waits on the same shape of thing: in-process
// asynchronous work — a hotkey callback, a goroutine the app started, a mode
// transition queued behind the handler lock — that finishes in single-digit
// milliseconds on an idle machine. So a wait's budget is not an estimate of
// how long its step takes. It is headroom against the machine, plus, where the
// awaited thing has a duration of its own, that duration.
//
// Two things move the machine side of it, and only one of them is fixable with
// a number:
//
//   - The race detector instruments every memory access, which is why the
//     -race pass is where a fixed budget runs out first. That is a bounded,
//     knowable factor, so simWaitHeadroom scales by simRaceSlowdown.
//   - A loaded shared runner can deschedule this process for whole seconds
//     regardless of how little work it has left. No budget survives that by
//     being bigger, so waitForWithin does not rely on one: it evaluates the
//     condition and only then looks at the clock, so the last thing a wait
//     does before giving up is ask again. A wait therefore fails only on work
//     that has not happened — never on work that happened while the process
//     was not running. Of the two halves of #1324, that is the half no bigger
//     number could have bought.
//
// What the budget buys after that is the case where the work is genuinely
// still coming, and what it costs is how long a genuinely hung journey takes
// to report — which is why it is seconds and not minutes.
//
// Adding a journey: call waitFor and inherit simWaitHeadroom. Reach for
// waitForWithin only when what you are awaiting has its own duration — a
// configured repeat interval, a sampling window — and write that duration into
// the call as `simWaitHeadroom + <it>`, so the headroom stays one thing and
// the journey's own timing stays visible. Growing simWaitHeadroom to cover one
// slow wait puts every other journey's hang detection behind it.
const (
	// simWaitHeadroomBase is the machine headroom one wait gets on an
	// uninstrumented build. The work these journeys await completes in
	// single-digit milliseconds, so this is roughly three orders of magnitude
	// of slack — sized for a shared runner having a bad minute, not for the
	// work.
	simWaitHeadroomBase = 5 * time.Second

	// simRaceSlowdown is how much further to stretch a wait budget when the
	// binary carries the race detector. The detector's documented cost is a
	// 2-20x slowdown; 4 sits inside that on top of a base that is already
	// mostly slack, and it is applied once, here, so no journey has to know
	// how it was built.
	simRaceSlowdown = 4

	// simPollInterval is how often a wait re-asks its condition. Short enough
	// that a journey is not measuring the poller, long enough that a hundred
	// waits do not busy-spin a constrained CPU away from the app they are
	// waiting on.
	simPollInterval = 2 * time.Millisecond

	// simShutdownBudgetBase is what Stop() gets to unwind every startup phase
	// and return from Run(). It is a budget of its own rather than a reuse of
	// the wait headroom because it covers a different thing: not one
	// asynchronous hop, but the whole application coming down.
	simShutdownBudgetBase = 5 * time.Second
)

// simWaitHeadroom is the default budget for a single waitFor, and
// simShutdownBudget the one the harness gives a stopping app; both are stated
// above in idle-machine terms and scaled here, once.
var (
	simWaitHeadroom   = simScaleForRace(simWaitHeadroomBase)
	simShutdownBudget = simScaleForRace(simShutdownBudgetBase)
)

// simScaleForRace stretches a budget stated for an uninstrumented build to
// what the same wait needs under -race.
func simScaleForRace(budget time.Duration) time.Duration {
	if !simRaceDetectorEnabled {
		return budget
	}

	return budget * simRaceSlowdown
}

// defaultDisplayName is the name of the fixture desktop's single display.
const defaultDisplayName = "SimDisplay"

// simScreen is the fixture display: a single 1920x1080 screen at the global
// origin, so screen-local and global coordinates coincide.
var simScreen = image.Rect(0, 0, 1920, 1080)

// simScreenResized is that same display after a resolution change. Every
// element in the journey fixtures still falls inside it, so what a display
// change does to a mode is not confused with elements dropping off the screen.
var simScreenResized = image.Rect(0, 0, 1280, 720)

// simDisplayResized is the whole fixture desktop after that change.
func simDisplayResized() simDisplay {
	return simDisplay{name: defaultDisplayName, bounds: simScreenResized}
}

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
	// refuseHintSearch stands in for a backend that draws no hint search
	// badge: the draw reports CodeNotSupported, the way the Linux overlay does
	// for a search input no backend there implements.
	refuseHintSearch bool

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

	// gridPointers is the pointer each grid surface was last asked to draw.
	gridPointers map[domain.Mode]ports.GridPointer

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

// DrawHintSearch records the query the search input was asked to show. A
// refusing overlay records the ask too and then reports CodeNotSupported —
// what a journey observes there is that the user was asked for, not that
// anything was drawn.
func (m *simOverlayPort) DrawHintSearch(search ports.HintSearch) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.searchQueries = append(m.searchQueries, search.Query)

	if m.refuseHintSearch {
		return derrors.New(derrors.CodeNotSupported, "no hint search input on this backend")
	}

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

// UpdateGridPointer records the pointer stand-in a grid surface draws where
// the selection is. For a user whose cursor does not follow the selection it
// is the only thing on screen saying where that selection is, so a journey
// reads it to watch one appear and a stale one go away.
func (m *simOverlayPort) UpdateGridPointer(mode domain.Mode, pointer ports.GridPointer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.gridPointers == nil {
		m.gridPointers = make(map[domain.Mode]ports.GridPointer)
	}

	m.gridPointers[mode] = pointer
}

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

// Refresh sizes the overlay to the display that is now active, which brings it
// up: it is the same resize the window sequence performs on its way to putting
// a frame on screen, so a surface that was not on screen is afterwards. That
// is the whole reason the screen-change path refuses to call it while idle
// ("Resizing the overlay when idle would cause it to become visible",
// lifecycle.go), and modeling it as silent success is what would let that
// guard be deleted without a journey noticing.
//
// It puts no content there, so it draws no mode and is not a show.
func (m *simOverlayPort) Refresh(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.visible = true

	return nil
}

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

// searchInputAskCount reports how many times the overlay was asked to show the
// search input. It counts asks rather than draws because a refusing overlay
// records the ask and then declines it, and "search is open" is what a journey
// needs to observe either way.
func (m *simOverlayPort) searchInputAskCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.searchQueries)
}

// monitorDrawCount reports how many times the monitor picker was drawn.
func (m *simOverlayPort) monitorDrawCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.monitorDraws)
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

// lastHintScreen returns the display the most recent hints frame was drawn
// against, and whether one was ever drawn. A screen change is only complete
// when this is the display the user is now looking at.
func (m *simOverlayPort) lastHintScreen() (image.Rectangle, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.hintFrames) == 0 {
		return image.Rectangle{}, false
	}

	return m.hintFrames[len(m.hintFrames)-1].Screen, true
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

// lastGridPointer reports the pointer a grid surface was last asked to draw,
// and whether it was ever asked at all.
func (m *simOverlayPort) lastGridPointer(mode domain.Mode) (ports.GridPointer, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pointer, drawn := m.gridPointers[mode]

	return pointer, drawn
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

	// focusedApp is which application the fixture desktop routes keystrokes
	// to, and focusedAppQueries how many times the app asked for it.
	//
	// Both exist because asking is not free on a real desktop: on macOS it is a
	// message to another process, so an application that is busy or wedged
	// answers slowly or not at all. How many times a single keystroke asks is
	// therefore a property a user can be stalled by, and a journey that counts
	// it is stating what a keystroke costs rather than describing it.
	focusedApp        string
	focusedAppQueries int
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
	a.mu.Lock()
	defer a.mu.Unlock()

	a.focusedAppQueries++

	return a.focusedApp, nil
}

func (a *simAXPort) IsAppExcluded(_ context.Context, _ string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.excluded
}

func (a *simAXPort) PrimeApplication(_ context.Context, _ string) (bool, error) {
	return true, nil
}

// setElements replaces the accessibility tree, the way a display change that
// relaid out or closed windows leaves it.
func (a *simAXPort) setElements(elements []*element.Element) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.elements = elements
}

// setFocusedApp changes which application the fixture desktop reports as
// focused, the way switching applications does on a real one.
func (a *simAXPort) setFocusedApp(bundleID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.focusedApp = bundleID
}

// focusedAppQueryCount reports how many times the app has asked which
// application is focused.
func (a *simAXPort) focusedAppQueryCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.focusedAppQueries
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
	// watcher is the platform application watcher: the thing that tells Neru
	// the user switched applications. A journey drives a focus change through
	// it rather than by poking the accessibility fake alone, because the
	// activation event is the half of a focus change that reaches the app.
	watcher *mocks.MockAppWatcherPort
	desktop *simDesktop
	runDone chan error
}

// focusApp switches the fixture desktop to another application: the desktop
// answers with it from now on, and the platform watcher announces the
// activation the way it does when the user brings an application to the front.
//
// The desktop is switched before the announcement, so everything the app reads
// while handling it sees the application the user is now in, as it would on a
// real desktop.
func (h *simHarness) focusApp(appName, bundleID string) {
	h.ax.setFocusedApp(bundleID)
	h.watcher.EmitActivate(appName, bundleID)
}

// switchToDarkMode flips the fixture desktop's appearance and notifies the app
// the way a platform theme observer does.
func (h *simHarness) switchToDarkMode() {
	h.appearance.Store(true)
	h.app.HandleThemeChange(true)
}

// changeScreen rearranges the fixture desktop to the given displays and
// notifies the app the way a platform screen-parameters observer does — a
// monitor plugged in or unplugged, or a resolution change.
//
// The new arrangement is in place before the notification, so everything the
// app reads while handling it sees the new displays, as it would on a real
// desktop.
func (h *simHarness) changeScreen(displays ...simDisplay) {
	h.t.Helper()

	if len(displays) == 0 {
		h.t.Fatal("changeScreen needs at least one display")
	}

	h.desktop.set(displays)
	h.app.HandleScreenParametersChange()
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
	// The defaults as written rather than as derived, because buildSimHarness
	// derives — a journey that changes a source option at runtime has to start
	// from a configuration where the values derived from it are still empty,
	// which is what the daemon's own defaults are before a load settles them.
	cfg := config.DefaultConfigForDecoding()
	cfg.Hotkeys.Bindings = map[string][]string{
		hintsHotkey:         {"hints"},
		gridHotkey:          {"grid"},
		recursiveGridHotkey: {"recursive_grid"},
		scrollHotkey:        {"scroll"},
	}

	return cfg
}

// unpressedOverrideKey is what a per-app override that exists only to open the
// focused-app path is bound to. Nothing presses it: declaring an override at
// all is what makes a keystroke resolve the focused app, so the binding's key
// is deliberately one no journey types.
const unpressedOverrideKey = "F13"

// perAppHotkeyOverride is one mode's per-app hotkey override: the steps the
// given key runs while that application is focused.
func perAppHotkeyOverride(bundleID, key string, steps ...string) config.AppConfig {
	return config.AppConfig{
		BundleID: bundleID,
		Hotkeys:  map[string]config.StringOrStringArray{key: steps},
	}
}

// firstGridLabelKey returns the lower-case first character of a drawn cell's
// label — the key a user would press to start narrowing to it.
func firstGridLabelKey(sim *simHarness) string {
	grid := sim.overlay.lastGrid()

	cells := grid.Cells()
	if len(cells) == 0 {
		sim.t.Fatal("grid drawn with zero cells")
	}

	label := cells[0].Coordinate()
	if label == "" {
		sim.t.Fatal("grid cell drawn without a label")
	}

	return strings.ToLower(string([]rune(label)[0]))
}

// simDisplay is one monitor of the simulated desktop.
type simDisplay struct {
	name   string
	bounds image.Rectangle
}

// simDesktop is the fixture desktop's display arrangement. A display
// configuration change rearranges it while the app is reading it, so it is
// held behind a mutex rather than closed over as a fixed slice.
type simDesktop struct {
	mu       sync.Mutex
	displays []simDisplay
}

func (d *simDesktop) set(displays []simDisplay) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.displays = slices.Clone(displays)
}

func (d *simDesktop) all() []simDisplay {
	d.mu.Lock()
	defer d.mu.Unlock()

	return slices.Clone(d.displays)
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
		{name: defaultDisplayName, bounds: simScreen},
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

// newSimHarnessRefusingHintSearch builds the app on an overlay that draws
// everything except the hint search badge, which it refuses with
// CodeNotSupported — the Linux overlay's answer for a search input no backend
// there implements. The recorder is kept, because what this stands up is a
// mode that has to go on working without it.
func newSimHarnessRefusingHintSearch(
	tb testing.TB,
	cfg *config.Config,
	elements []*element.Element,
) *simHarness {
	tb.Helper()

	recorder := &simOverlayPort{refuseHintSearch: true}

	return buildSimHarness(tb, cfg, elements, []simDisplay{
		{name: defaultDisplayName, bounds: simScreen},
	}, recorder, recorder)
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

	// The journey's cfg is the configuration as written, and the app is handed
	// both halves of it the way the daemon hands it the two the loader produced.
	// A journey that writes a source option and expects the values derived from
	// it to follow — `neru config set grid.characters` relabelling the grid —
	// only reaches that behavior through this pair.
	written := cfg

	running, deriveErr := loader.DeepCopyConfig(written)
	if deriveErr != nil {
		tb.Fatalf("deep copy config failed: %v", deriveErr)
	}

	loader.ResolveDerived(running)

	cfg = running

	axPort := &simAXPort{elements: elements, focusedApp: simFixtureBundleID}
	appearance := &atomic.Bool{}
	watcher := &mocks.MockAppWatcherPort{}
	cursor := &simCursor{pos: displays[0].bounds.Min.Add(
		image.Point{X: displays[0].bounds.Dx() / 2, Y: displays[0].bounds.Dy() / 2},
	)}
	hotkeys := &simHotkeyPort{}
	tap := &mocks.MockEventTapPort{}
	desktop := &simDesktop{}
	desktop.set(displays)

	// The active screen is wherever the cursor currently is, matching how the
	// real system port resolves it.
	activeBounds := func() image.Rectangle {
		current := desktop.all()

		pos := cursor.position()
		for _, display := range current {
			if pos.In(display.bounds) {
				return display.bounds
			}
		}

		return current[0].bounds
	}

	system := &mocks.MockSystemPort{
		IsDarkModeFunc: func() bool {
			return appearance.Load()
		},
		ScreenBoundsFunc: func(_ context.Context) (image.Rectangle, error) {
			return activeBounds(), nil
		},
		ScreenBoundsByNameFunc: func(_ context.Context, name string) (image.Rectangle, bool, error) {
			for _, display := range desktop.all() {
				if display.name == name {
					return display.bounds, true, nil
				}
			}

			return image.Rectangle{}, false, nil
		},
		ScreenNamesFunc: func(_ context.Context) ([]string, error) {
			current := desktop.all()

			names := make([]string, len(current))
			for idx, display := range current {
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
		app.WithWrittenConfig(written),
		app.WithConfigPath(""),
		app.WithLogger(zap.NewNop()),
		app.WithEventTap(tap),
		app.WithTextInput(&mocks.MockTextInputPort{}),
		app.WithIPCServer(&mocks.MockIPCPort{}),
		app.WithWatcher(watcher),
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
		watcher:    watcher,
		desktop:    desktop,
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
		case <-time.After(simShutdownBudget):
			tb.Errorf("app did not stop within %v", simShutdownBudget)
		}

		application.Cleanup()
	})

	// Wait for the app watcher to be running, not for IsEnabled: the app is
	// enabled before Run is entered at all, so waiting on that waited for
	// nothing and a journey could drive the fixture desktop before the app was
	// listening. Run starts the watcher last of the three steps that have to be
	// in place first (lifecycle.go), so this says exactly that a focus change
	// from here on is one the app hears — and no more than that.
	sim.waitFor("the app watcher running", watcher.Started)

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

// waitFor polls cond until it holds, giving it the default machine headroom.
func (h *simHarness) waitFor(desc string, cond func() bool) {
	h.t.Helper()

	h.waitForWithin(simWaitHeadroom, desc, cond)
}

// waitForWithin polls cond until it holds or budget elapses.
//
// The order of the two checks is the load-bearing part: cond is evaluated
// first and the clock consulted only after it comes back false, so however far
// past the budget a descheduled poll wakes up, the condition is still asked
// once more before the wait gives up. A journey therefore fails on work that
// did not happen, never on work that happened while this process was starved
// of a CPU — the flake #1324 reported.
//
// The budget is therefore when a wait stops trying, not a ceiling on how long
// it takes: a wait can outlast it by one evaluation of cond, which matters
// only for a condition that is itself slow to answer.
func (h *simHarness) waitForWithin(budget time.Duration, desc string, cond func() bool) {
	h.t.Helper()

	deadline := time.Now().Add(budget)

	for {
		if cond() {
			return
		}

		if !time.Now().Before(deadline) {
			h.t.Fatalf("timed out after %v waiting for %s", budget, desc)
		}

		time.Sleep(simPollInterval)
	}
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
//
// Deliberately not scaled by simScaleForRace, unlike every wait budget above.
// A wait is scaled because slowness turns it into a failure; this window's
// failure mode is the opposite one — a starved process watches nothing and
// passes vacuously — and stretching the window does not fix that, because a
// starved four seconds is as empty as a starved 250 milliseconds. It would
// only add real seconds to every -race run. The window a caller writes is the
// window it gets.
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
