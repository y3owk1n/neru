package mocks

import (
	"context"
	"image"
	"sync"

	"github.com/y3owk1n/neru/internal/domain/hint"
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

	ShowHintsFunc func(context.Context, []*hint.Interface) error
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
	HideFunc                     func(context.Context) error
	IsVisibleFunc                func() bool
	RefreshFunc                  func(context.Context) error
	HealthFunc                   func(context.Context) error

	// State tracking for tests
	visible bool
}

// ShowHints implements ports.OverlayPort.
func (m *MockOverlayPort) ShowHints(ctx context.Context, hints []*hint.Interface) error {
	if m.ShowHintsFunc != nil {
		return m.ShowHintsFunc(ctx, hints)
	}

	m.visible = true

	return nil
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

// Hide implements ports.OverlayPort.
func (m *MockOverlayPort) Hide(ctx context.Context) error {
	if m.HideFunc != nil {
		return m.HideFunc(ctx)
	}

	m.visible = false

	return nil
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

// Health checks if the overlay manager is responsive.
func (m *MockOverlayPort) Health(ctx context.Context) error {
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}

	return nil
}

func (m *MockOverlayPort) setIndicatorVisible(indicator ports.Indicator, visible bool) {
	m.indicatorMu.Lock()
	defer m.indicatorMu.Unlock()

	if m.indicatorVisible == nil {
		m.indicatorVisible = make(map[ports.Indicator]bool)
	}

	m.indicatorVisible[indicator] = visible
}

// Ensure MockOverlayPort implements ports.OverlayPort.
var _ ports.OverlayPort = (*MockOverlayPort)(nil)
