package mocks

import (
	"context"
	"image"
	"sync"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/ports"
)

// MockVisionPort is a mock implementation of ports.VisionPort.
//
// Its zero value behaves like every non-macOS platform: CodeNotSupported from
// all three methods. Set the Func fields to model a working Vision backend.
type MockVisionPort struct {
	HealthFunc         func(context.Context) error
	DetectElementsFunc func(
		context.Context,
		image.Rectangle,
		config.HintsVisionConfig,
		bool,
	) ([]*element.Element, error)
	CaptureScreenFunc func(context.Context) (*image.RGBA, error)
	DetectWLKBPTRFunc func(context.Context, image.Rectangle) ([]*element.Element, error)

	mu       sync.Mutex
	detectN  int
	captureN int
	wlkbptrN int
}

// Health implements ports.VisionPort.
func (m *MockVisionPort) Health(ctx context.Context) error {
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}

	return derrors.New(derrors.CodeNotSupported, "vision framework is only available on macOS")
}

// DetectElements implements ports.VisionPort.
func (m *MockVisionPort) DetectElements(
	ctx context.Context,
	screenBounds image.Rectangle,
	cfg config.HintsVisionConfig,
	splitWord bool,
) ([]*element.Element, error) {
	m.mu.Lock()
	m.detectN++
	m.mu.Unlock()

	if m.DetectElementsFunc != nil {
		return m.DetectElementsFunc(ctx, screenBounds, cfg, splitWord)
	}

	return nil, derrors.New(derrors.CodeNotSupported, "vision detection is only supported on macOS")
}

// CaptureScreen implements ports.VisionPort.
func (m *MockVisionPort) CaptureScreen(ctx context.Context) (*image.RGBA, error) {
	m.mu.Lock()
	m.captureN++
	m.mu.Unlock()

	if m.CaptureScreenFunc != nil {
		return m.CaptureScreenFunc(ctx)
	}

	return nil, derrors.New(derrors.CodeNotSupported, "screen capture is only supported on macOS")
}

// DetectWLKBPTR implements ports.VisionPort.
func (m *MockVisionPort) DetectWLKBPTR(
	ctx context.Context,
	screenBounds image.Rectangle,
) ([]*element.Element, error) {
	m.mu.Lock()
	m.wlkbptrN++
	m.mu.Unlock()

	if m.DetectWLKBPTRFunc != nil {
		return m.DetectWLKBPTRFunc(ctx, screenBounds)
	}

	return nil, derrors.New(derrors.CodeNotSupported, "wl-kbptr is only supported on Linux")
}

// DetectCallCount reports how many times DetectElements was called.
func (m *MockVisionPort) DetectCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.detectN
}

// CaptureCallCount reports how many times CaptureScreen was called.
func (m *MockVisionPort) CaptureCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.captureN
}

// DetectWLKBPTRCallCount reports how many times DetectWLKBPTR was called.
func (m *MockVisionPort) DetectWLKBPTRCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.wlkbptrN
}

// Ensure MockVisionPort implements ports.VisionPort.
var _ ports.VisionPort = (*MockVisionPort)(nil)
