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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	overlaymanager "github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	rendergrid "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	renderhints "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	renderrecursivegrid "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
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
	t *testing.T,
	elementID string,
	bounds image.Rectangle,
	title string,
) *element.Element {
	t.Helper()

	elem, err := element.NewElement(
		element.ID(elementID),
		bounds,
		element.Role("button"),
		element.WithClickable(true),
		element.WithTitle(title),
	)
	if err != nil {
		t.Fatalf("failed to build fixture element %q: %v", elementID, err)
	}

	return elem
}

// --- recording overlay manager -------------------------------------------

// simOverlayManager records what the app draws. It embeds the headless
// NoOpManager so it satisfies the full manager contract and only overrides
// what the journeys assert on. It declares itself headless, which keeps the
// component factory on its headless path (no native overlay construction).
type simOverlayManager struct {
	overlay.NoOpManager

	mu                 sync.Mutex
	mode               overlay.Mode
	visible            bool
	hintDraws          [][]*renderhints.Hint
	hintStyles         []renderhints.StyleMode
	gridDraws          []*domainGrid.Grid
	recursiveGridDraws []image.Rectangle
	searchQueries      []string
	monitorDraws       [][]overlay.MonitorSelectTarget
	monitorHides       int
	stickySymbols      []string
	// indicatorVisible is the visibility each indicator was last asked for.
	// A backend with no surface draws nothing, so this — not a draw — is what
	// a journey can observe about an indicator being on screen.
	indicatorVisible map[ports.Indicator]bool
}

// Ensure the recorder implements the optional monitor-select extension the
// same way the darwin and Linux backends do, and declares itself headless the
// way a backend with no surface does.
var (
	_ overlaymanager.MonitorSelector  = (*simOverlayManager)(nil)
	_ overlaymanager.HeadlessReporter = (*simOverlayManager)(nil)
)

// Headless states outright what the journeys rely on: there is no native
// surface here, so the component factory must not build render overlays.
func (m *simOverlayManager) Headless() bool { return true }

func (m *simOverlayManager) DrawMonitorSelect(
	targets []overlay.MonitorSelectTarget,
	_ overlay.MonitorSelectStyle,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	drawn := make([]overlay.MonitorSelectTarget, len(targets))
	copy(drawn, targets)
	m.monitorDraws = append(m.monitorDraws, drawn)

	return nil
}

func (m *simOverlayManager) HideMonitorSelect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.monitorHides++
}

func (m *simOverlayManager) DrawStickyModifiersIndicator(_, _ int, symbols string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stickySymbols = append(m.stickySymbols, symbols)
}

func (m *simOverlayManager) ShowIndicator(indicator ports.Indicator) {
	m.setIndicatorVisible(indicator, true)
}

func (m *simOverlayManager) HideIndicator(indicator ports.Indicator) {
	m.setIndicatorVisible(indicator, false)
}

func (m *simOverlayManager) Show() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.visible = true
}

func (m *simOverlayManager) Hide() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.visible = false
}

func (m *simOverlayManager) SwitchTo(next overlay.Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mode = next
}

func (m *simOverlayManager) Mode() overlay.Mode {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.mode
}

func (m *simOverlayManager) DrawHintsWithStyle(
	hintsSlice []*renderhints.Hint,
	style renderhints.StyleMode,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	drawn := make([]*renderhints.Hint, len(hintsSlice))
	copy(drawn, hintsSlice)
	m.hintDraws = append(m.hintDraws, drawn)
	m.hintStyles = append(m.hintStyles, style)

	return nil
}

func (m *simOverlayManager) DrawGrid(
	grid *domainGrid.Grid,
	_ string,
	_ rendergrid.Style,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.gridDraws = append(m.gridDraws, grid)

	return nil
}

func (m *simOverlayManager) DrawRecursiveGrid(
	bounds image.Rectangle,
	_ int,
	_ string,
	_ int,
	_ int,
	_ string,
	_ int,
	_ int,
	_ renderrecursivegrid.Style,
	_ renderrecursivegrid.VirtualPointerState,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.recursiveGridDraws = append(m.recursiveGridDraws, bounds)

	return nil
}

func (m *simOverlayManager) DrawHintSearchInput(
	query string,
	_ int,
	_ renderhints.SearchInputFrame,
	_ renderhints.SearchInputStyle,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.searchQueries = append(m.searchQueries, query)

	return nil
}

func (m *simOverlayManager) setIndicatorVisible(indicator ports.Indicator, visible bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.indicatorVisible == nil {
		m.indicatorVisible = make(map[ports.Indicator]bool)
	}

	m.indicatorVisible[indicator] = visible
}

// indicatorVisibility reports the visibility an indicator was last asked for,
// and whether it was ever asked at all.
func (m *simOverlayManager) indicatorVisibility(indicator ports.Indicator) (bool, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	visible, asked := m.indicatorVisible[indicator]

	return visible, asked
}

func (m *simOverlayManager) searchInputDrawCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.searchQueries)
}

func (m *simOverlayManager) lastMonitorTargets() []overlay.MonitorSelectTarget {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.monitorDraws) == 0 {
		return nil
	}

	return m.monitorDraws[len(m.monitorDraws)-1]
}

func (m *simOverlayManager) stickyIndicatorDrawCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.stickySymbols)
}

func (m *simOverlayManager) monitorHideCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.monitorHides
}

func (m *simOverlayManager) lastRecursiveGridBounds() (image.Rectangle, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.recursiveGridDraws) == 0 {
		return image.Rectangle{}, false
	}

	return m.recursiveGridDraws[len(m.recursiveGridDraws)-1], true
}

func (m *simOverlayManager) isVisible() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.visible
}

func (m *simOverlayManager) hintDrawCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.hintDraws)
}

// lastHintStyle returns the style the most recent hint draw was given, which
// is what a user actually sees on screen.
func (m *simOverlayManager) lastHintStyle() (renderhints.StyleMode, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.hintStyles) == 0 {
		return renderhints.StyleMode{}, false
	}

	return m.hintStyles[len(m.hintStyles)-1], true
}

// lastHintLabels returns the labels of the most recent hint draw.
func (m *simOverlayManager) lastHintLabels() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.hintDraws) == 0 {
		return nil
	}

	last := m.hintDraws[len(m.hintDraws)-1]

	labels := make([]string, len(last))
	for i, h := range last {
		labels[i] = h.Label()
	}

	return labels
}

func (m *simOverlayManager) lastGrid() *domainGrid.Grid {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.gridDraws) == 0 {
		return nil
	}

	return m.gridDraws[len(m.gridDraws)-1]
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
	t   *testing.T
	app *app.App
	// appearance is the fixture desktop's light/dark state, read by the
	// system port the app resolves its theme through.
	appearance *atomic.Bool
	overlay    *simOverlayManager
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
	t *testing.T,
	cfg *config.Config,
	elements []*element.Element,
) *simHarness {
	t.Helper()

	return newSimHarnessWithDisplays(t, cfg, elements, []simDisplay{
		{name: "SimDisplay", bounds: simScreen},
	})
}

// simHeadlessOverlayManager is a manager WITHOUT the MonitorSelector
// extension, standing in for backends that cannot draw the monitor picker.
type simHeadlessOverlayManager struct {
	overlay.NoOpManager
}

// newSimHarnessWithDisplays builds and starts the real app on a simulated
// desktop with the given displays, and registers cleanup that stops it and
// fails the test on unclean shutdown. The cursor starts on the first display.
func newSimHarnessWithDisplays(
	t *testing.T,
	cfg *config.Config,
	elements []*element.Element,
	displays []simDisplay,
) *simHarness {
	t.Helper()

	recorder := &simOverlayManager{}

	return buildSimHarness(t, cfg, elements, displays, recorder, recorder)
}

// newSimHarnessHeadlessOverlay builds the app on an overlay manager without
// any optional extensions. The returned harness has a nil overlay recorder —
// only for journeys asserting that a capability is refused.
func newSimHarnessHeadlessOverlay(
	t *testing.T,
	cfg *config.Config,
	displays []simDisplay,
) *simHarness {
	t.Helper()

	return buildSimHarness(t, cfg, nil, displays, &simHeadlessOverlayManager{}, nil)
}

// buildSimHarness wires the app with the given overlay manager; recorder is
// that same manager when it records, nil otherwise.
func buildSimHarness(
	t *testing.T,
	cfg *config.Config,
	elements []*element.Element,
	displays []simDisplay,
	overlayManager app.OverlayManager,
	recorder *simOverlayManager,
) *simHarness {
	t.Helper()

	if len(displays) == 0 {
		t.Fatal("buildSimHarness needs at least one display")
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
		app.WithOverlayManager(overlayManager),
		app.WithSystemPort(system),
		app.WithAccessibility(axPort),
	)
	if err != nil {
		t.Fatalf("app.New failed: %v", err)
	}

	sim := &simHarness{
		t:          t,
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

	t.Cleanup(func() {
		application.Stop()

		select {
		case runErr := <-sim.runDone:
			if runErr != nil && !errors.Is(runErr, context.Canceled) &&
				!derrors.IsCode(runErr, derrors.CodeContextCanceled) {
				t.Errorf("Run() returned an unexpected error after Stop(): %v", runErr)
			}
		case <-time.After(simWaitTimeout):
			t.Errorf("app did not stop within %v", simWaitTimeout)
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
