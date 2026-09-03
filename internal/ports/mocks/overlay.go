package mocks

import (
	"context"
	"image"
	"slices"
	"sync"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

// MockOverlayPort is a mock implementation of ports.OverlayPort.
type MockOverlayPort struct {
	modeIndicatorMu sync.Mutex
	modeIndicatorX  int
	modeIndicatorY  int

	indicatorMu sync.Mutex
	// indicatorVisible records the last visibility a caller asked for, per
	// indicator: true after ShowIndicator, false after HideIndicator, absent
	// while a caller has never touched it.
	indicatorVisible map[ports.Indicator]bool
	indicatorResizes map[ports.Indicator]int

	virtualPointerMu sync.Mutex
	virtualPointerX  int
	virtualPointerY  int
	virtualPointerN  int

	// ShowFrameFunc mocks ShowFrame.
	ShowFrameFunc func(context.Context, ports.Frame) error
	// RedrawFrameFunc mocks RedrawFrame.
	RedrawFrameFunc func(context.Context, ports.Frame) error
	// ClearFrameFunc mocks ClearFrame.
	ClearFrameFunc func(context.Context) error
	// DrawHintSearchFunc mocks DrawHintSearch.
	DrawHintSearchFunc func(ports.HintSearch) error
	// HideHintSearchFunc mocks HideHintSearch.
	HideHintSearchFunc func()
	// HintSearchBoundsFunc mocks HintSearchBounds.
	HintSearchBoundsFunc func(image.Rectangle) image.Rectangle
	// DrawModeIndicatorFunc mocks DrawModeIndicator.
	DrawModeIndicatorFunc func(x, y int)
	// DrawStickyModifiersIndicatorFunc mocks DrawStickyModifiersIndicator.
	DrawStickyModifiersIndicatorFunc func(x, y int, symbols string)
	// DrawVirtualPointerFunc mocks DrawVirtualPointer.
	DrawVirtualPointerFunc func(x, y int)
	// ShowIndicatorFunc mocks ShowIndicator.
	ShowIndicatorFunc func(indicator ports.Indicator)
	// HideIndicatorFunc mocks HideIndicator.
	HideIndicatorFunc            func(indicator ports.Indicator)
	DrawMouseActionIndicatorFunc func(point image.Point, style ports.MouseActionIndicatorStyle)
	// FlushFunc mocks Flush.
	FlushFunc     func()
	IsVisibleFunc func() bool
	RefreshFunc   func(context.Context) error
	HealthFunc    func(context.Context) error

	frameMu sync.Mutex
	// frames records every Frame the caller handed over, in order, so a test
	// asserts on the domain values a user would have seen rather than on
	// which method the adapter would have used to draw them.
	frames []ports.Frame
	// hintSearches records every search input draw, in order.
	hintSearches []ports.HintSearch

	gridMu sync.Mutex
	// gridPrefixes records every prefix the grid was narrowed to, in order —
	// the per-keystroke path in grid mode, which by ADR 0003 never builds a
	// Frame and so is not visible in frames above.
	gridPrefixes []string
	// gridHideUnmatched records the last visibility asked of unmatched cells.
	gridHideUnmatched bool
	// gridSubgrids counts the subgrids opened inside a cell.
	gridSubgrids int
	// gridPointers records the last pointer asked for, per grid mode.
	gridPointers map[domain.Mode]ports.GridPointer

	screenMu sync.Mutex
	// activeScreen is the display the overlay was last told its screen-local
	// content belongs to.
	activeScreen image.Rectangle
	// flushes counts how many times the caller committed what it had drawn.
	flushes int

	styleMu sync.Mutex
	// appliedConfigs records every configuration the overlay was handed, in
	// order, and styleRefreshes how many times it was asked to re-resolve
	// against the one it already held.
	appliedConfigs  []*config.Config
	styleRefreshes  int
	screenShareHide bool
	keyboardCapture bool
	destroys        int

	// State tracking for tests
	visible bool
}

// ShowFrame implements ports.OverlayPort.
func (m *MockOverlayPort) ShowFrame(ctx context.Context, frame ports.Frame) error {
	m.recordFrame(frame)

	if m.ShowFrameFunc != nil {
		return m.ShowFrameFunc(ctx, frame)
	}

	m.visible = true

	return nil
}

// RedrawFrame implements ports.OverlayPort.
func (m *MockOverlayPort) RedrawFrame(ctx context.Context, frame ports.Frame) error {
	m.recordFrame(frame)

	if m.RedrawFrameFunc != nil {
		return m.RedrawFrameFunc(ctx, frame)
	}

	return nil
}

// ClearFrame implements ports.OverlayPort.
func (m *MockOverlayPort) ClearFrame(ctx context.Context) error {
	if m.ClearFrameFunc != nil {
		return m.ClearFrameFunc(ctx)
	}

	m.visible = false

	return nil
}

// SetActiveScreen implements ports.OverlayPort.
func (m *MockOverlayPort) SetActiveScreen(screen image.Rectangle) {
	m.screenMu.Lock()
	defer m.screenMu.Unlock()

	m.activeScreen = screen
}

// ActiveScreen returns the display the overlay was last told about.
func (m *MockOverlayPort) ActiveScreen() image.Rectangle {
	m.screenMu.Lock()
	defer m.screenMu.Unlock()

	return m.activeScreen
}

// Flush implements ports.OverlayPort.
func (m *MockOverlayPort) Flush() {
	m.screenMu.Lock()
	m.flushes++
	m.screenMu.Unlock()

	if m.FlushFunc != nil {
		m.FlushFunc()
	}
}

// FlushCount returns how many times the overlay was asked to commit its draws.
func (m *MockOverlayPort) FlushCount() int {
	m.screenMu.Lock()
	defer m.screenMu.Unlock()

	return m.flushes
}

// UpdateGridMatches implements ports.OverlayPort.
func (m *MockOverlayPort) UpdateGridMatches(prefix string) {
	m.gridMu.Lock()
	defer m.gridMu.Unlock()

	m.gridPrefixes = append(m.gridPrefixes, prefix)
}

// SetGridHideUnmatched implements ports.OverlayPort.
func (m *MockOverlayPort) SetGridHideUnmatched(hide bool) {
	m.gridMu.Lock()
	defer m.gridMu.Unlock()

	m.gridHideUnmatched = hide
}

// ShowGridSubgrid implements ports.OverlayPort. The pointer it carries is
// recorded where UpdateGridPointer records one, because from a caller's side it
// is the same statement about the same surface — made in the same call so the
// surface is painted once (#1492).
func (m *MockOverlayPort) ShowGridSubgrid(_ *grid.Cell, pointer ports.GridPointer) {
	m.gridMu.Lock()
	defer m.gridMu.Unlock()

	m.gridSubgrids++
	m.recordGridPointerLocked(domain.ModeGrid, pointer)
}

// UpdateGridPointer implements ports.OverlayPort.
func (m *MockOverlayPort) UpdateGridPointer(mode domain.Mode, pointer ports.GridPointer) {
	m.gridMu.Lock()
	defer m.gridMu.Unlock()

	m.recordGridPointerLocked(mode, pointer)
}

// GridPrefixes returns every prefix the grid was narrowed to, in order.
func (m *MockOverlayPort) GridPrefixes() []string {
	m.gridMu.Lock()
	defer m.gridMu.Unlock()

	out := make([]string, len(m.gridPrefixes))
	copy(out, m.gridPrefixes)

	return out
}

// GridHideUnmatched reports the last visibility asked of unmatched cells.
func (m *MockOverlayPort) GridHideUnmatched() bool {
	m.gridMu.Lock()
	defer m.gridMu.Unlock()

	return m.gridHideUnmatched
}

// GridSubgridCount reports how many subgrids were opened.
func (m *MockOverlayPort) GridSubgridCount() int {
	m.gridMu.Lock()
	defer m.gridMu.Unlock()

	return m.gridSubgrids
}

// GridPointer reports the pointer a grid mode's surface was last asked for,
// and whether it was ever asked at all.
func (m *MockOverlayPort) GridPointer(mode domain.Mode) (ports.GridPointer, bool) {
	m.gridMu.Lock()
	defer m.gridMu.Unlock()

	pointer, asked := m.gridPointers[mode]

	return pointer, asked
}

// Frames returns every Frame handed to ShowFrame or RedrawFrame, in order.
func (m *MockOverlayPort) Frames() []ports.Frame {
	m.frameMu.Lock()
	defer m.frameMu.Unlock()

	out := make([]ports.Frame, len(m.frames))
	copy(out, m.frames)

	return out
}

// LastHintLabels returns the labels of the most recent hints Frame, and
// whether one was ever handed over.
func (m *MockOverlayPort) LastHintLabels() ([]string, bool) {
	m.frameMu.Lock()
	defer m.frameMu.Unlock()

	for _, v := range slices.Backward(m.frames) {
		hints, ok := v.(ports.HintsFrame)
		if !ok {
			continue
		}

		labels := make([]string, len(hints.Hints))
		for labelIndex, drawn := range hints.Hints {
			labels[labelIndex] = drawn.Label()
		}

		return labels, true
	}

	return nil, false
}

// LastModeIndicatorPosition returns the coordinates of the most recent
// DrawModeIndicator call.
func (m *MockOverlayPort) LastModeIndicatorPosition() (int, int) {
	m.modeIndicatorMu.Lock()
	defer m.modeIndicatorMu.Unlock()

	return m.modeIndicatorX, m.modeIndicatorY
}

// DrawModeIndicator implements ports.OverlayPort.
func (m *MockOverlayPort) DrawModeIndicator(posX, posY int) {
	m.modeIndicatorMu.Lock()
	m.modeIndicatorX, m.modeIndicatorY = posX, posY
	m.modeIndicatorMu.Unlock()

	if m.DrawModeIndicatorFunc != nil {
		m.DrawModeIndicatorFunc(posX, posY)
	}
}

// DrawStickyModifiersIndicator implements ports.OverlayPort.
func (m *MockOverlayPort) DrawStickyModifiersIndicator(x, y int, symbols string) {
	if m.DrawStickyModifiersIndicatorFunc != nil {
		m.DrawStickyModifiersIndicatorFunc(x, y, symbols)
	}
}

// DrawVirtualPointer implements ports.OverlayPort.
func (m *MockOverlayPort) DrawVirtualPointer(posX, posY int) {
	m.virtualPointerMu.Lock()
	m.virtualPointerX, m.virtualPointerY = posX, posY
	m.virtualPointerN++
	m.virtualPointerMu.Unlock()

	if m.DrawVirtualPointerFunc != nil {
		m.DrawVirtualPointerFunc(posX, posY)
	}
}

// LastVirtualPointerPosition returns the coordinates of the most recent
// DrawVirtualPointer call and how many there have been.
func (m *MockOverlayPort) LastVirtualPointerPosition() (int, int, int) {
	m.virtualPointerMu.Lock()
	defer m.virtualPointerMu.Unlock()

	return m.virtualPointerX, m.virtualPointerY, m.virtualPointerN
}

// ShowIndicator implements ports.OverlayPort.
func (m *MockOverlayPort) ShowIndicator(indicator ports.Indicator) {
	m.setIndicatorVisible(indicator, true)

	if m.ShowIndicatorFunc != nil {
		m.ShowIndicatorFunc(indicator)
	}
}

// HideIndicator implements ports.OverlayPort.
func (m *MockOverlayPort) HideIndicator(indicator ports.Indicator) {
	m.setIndicatorVisible(indicator, false)

	if m.HideIndicatorFunc != nil {
		m.HideIndicatorFunc(indicator)
	}
}

// ResizeIndicatorToActiveScreen implements ports.OverlayPort.
func (m *MockOverlayPort) ResizeIndicatorToActiveScreen(indicator ports.Indicator) {
	m.indicatorMu.Lock()
	defer m.indicatorMu.Unlock()

	if m.indicatorResizes == nil {
		m.indicatorResizes = make(map[ports.Indicator]int)
	}

	m.indicatorResizes[indicator]++
}

// IndicatorVisible reports the visibility last asked of an indicator, and
// whether it was ever asked at all.
func (m *MockOverlayPort) IndicatorVisible(indicator ports.Indicator) (bool, bool) {
	m.indicatorMu.Lock()
	defer m.indicatorMu.Unlock()

	visible, asked := m.indicatorVisible[indicator]

	return visible, asked
}

// IndicatorResizeCount returns how often an indicator was sized to the active
// screen.
func (m *MockOverlayPort) IndicatorResizeCount(indicator ports.Indicator) int {
	m.indicatorMu.Lock()
	defer m.indicatorMu.Unlock()

	return m.indicatorResizes[indicator]
}

// DrawMouseActionIndicator implements ports.OverlayPort.
func (m *MockOverlayPort) DrawMouseActionIndicator(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
) {
	if m.DrawMouseActionIndicatorFunc != nil {
		m.DrawMouseActionIndicatorFunc(point, style)
	}
}

// DrawHintSearch implements ports.OverlayPort.
func (m *MockOverlayPort) DrawHintSearch(search ports.HintSearch) error {
	m.frameMu.Lock()
	m.hintSearches = append(m.hintSearches, search)
	m.frameMu.Unlock()

	if m.DrawHintSearchFunc != nil {
		return m.DrawHintSearchFunc(search)
	}

	return nil
}

// HideHintSearch implements ports.OverlayPort.
func (m *MockOverlayPort) HideHintSearch() {
	if m.HideHintSearchFunc != nil {
		m.HideHintSearchFunc()
	}
}

// HintSearchBounds implements ports.OverlayPort.
func (m *MockOverlayPort) HintSearchBounds(screen image.Rectangle) image.Rectangle {
	if m.HintSearchBoundsFunc != nil {
		return m.HintSearchBoundsFunc(screen)
	}

	return image.Rectangle{}
}

// HintSearchDrawCount returns how many times the search input was drawn.
func (m *MockOverlayPort) HintSearchDrawCount() int {
	m.frameMu.Lock()
	defer m.frameMu.Unlock()

	return len(m.hintSearches)
}

// IsVisible implements ports.OverlayPort.
func (m *MockOverlayPort) IsVisible() bool {
	if m.IsVisibleFunc != nil {
		return m.IsVisibleFunc()
	}

	return m.visible
}

// Refresh implements ports.OverlayPort.
func (m *MockOverlayPort) Refresh(ctx context.Context) error {
	if m.RefreshFunc != nil {
		return m.RefreshFunc(ctx)
	}

	return nil
}

// ApplyConfig implements ports.OverlayPort.
func (m *MockOverlayPort) ApplyConfig(cfg *config.Config) {
	m.styleMu.Lock()
	defer m.styleMu.Unlock()

	m.appliedConfigs = append(m.appliedConfigs, cfg)
}

// AppliedConfigs returns every configuration the overlay was handed, in order.
func (m *MockOverlayPort) AppliedConfigs() []*config.Config {
	m.styleMu.Lock()
	defer m.styleMu.Unlock()

	return slices.Clone(m.appliedConfigs)
}

// RefreshStyles implements ports.OverlayPort.
func (m *MockOverlayPort) RefreshStyles() {
	m.styleMu.Lock()
	defer m.styleMu.Unlock()

	m.styleRefreshes++
}

// StyleRefreshCount returns how many times the overlay was asked to re-resolve
// its Styles against the configuration it already held.
func (m *MockOverlayPort) StyleRefreshCount() int {
	m.styleMu.Lock()
	defer m.styleMu.Unlock()

	return m.styleRefreshes
}

// SetHiddenInScreenShare implements ports.OverlayPort.
func (m *MockOverlayPort) SetHiddenInScreenShare(hidden bool) {
	m.styleMu.Lock()
	defer m.styleMu.Unlock()

	m.screenShareHide = hidden
}

// HiddenInScreenShare returns the last screen-share visibility asked for.
func (m *MockOverlayPort) HiddenInScreenShare() bool {
	m.styleMu.Lock()
	defer m.styleMu.Unlock()

	return m.screenShareHide
}

// SetKeyboardCaptureEnabled implements ports.OverlayPort.
func (m *MockOverlayPort) SetKeyboardCaptureEnabled(enabled bool) {
	m.styleMu.Lock()
	defer m.styleMu.Unlock()

	m.keyboardCapture = enabled
}

// KeyboardCaptureEnabled returns the last keyboard-capture state asked for.
func (m *MockOverlayPort) KeyboardCaptureEnabled() bool {
	m.styleMu.Lock()
	defer m.styleMu.Unlock()

	return m.keyboardCapture
}

// Destroy implements ports.OverlayPort.
func (m *MockOverlayPort) Destroy() {
	m.styleMu.Lock()
	defer m.styleMu.Unlock()

	m.destroys++
}

// DestroyCount returns how many times the overlay was torn down.
func (m *MockOverlayPort) DestroyCount() int {
	m.styleMu.Lock()
	defer m.styleMu.Unlock()

	return m.destroys
}

// Health checks if the overlay manager is responsive.
func (m *MockOverlayPort) Health(ctx context.Context) error {
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}

	return nil
}

// recordGridPointerLocked stores the pointer a grid surface was last asked for.
// Caller must hold gridMu.
func (m *MockOverlayPort) recordGridPointerLocked(mode domain.Mode, pointer ports.GridPointer) {
	if m.gridPointers == nil {
		m.gridPointers = make(map[domain.Mode]ports.GridPointer)
	}

	m.gridPointers[mode] = pointer
}

func (m *MockOverlayPort) setIndicatorVisible(indicator ports.Indicator, visible bool) {
	m.indicatorMu.Lock()
	defer m.indicatorMu.Unlock()

	if m.indicatorVisible == nil {
		m.indicatorVisible = make(map[ports.Indicator]bool)
	}

	m.indicatorVisible[indicator] = visible
}

func (m *MockOverlayPort) recordFrame(frame ports.Frame) {
	m.frameMu.Lock()
	defer m.frameMu.Unlock()

	m.frames = append(m.frames, frame)
}

// Ensure MockOverlayPort implements ports.OverlayPort.
var _ ports.OverlayPort = (*MockOverlayPort)(nil)
