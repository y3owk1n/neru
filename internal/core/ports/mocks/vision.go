package mocks

import (
	"context"
	"image"
	"sync"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
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

	mu       sync.Mutex
	detectN  int
	captureN int
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

// Ensure MockVisionPort implements ports.VisionPort.
var _ ports.VisionPort = (*MockVisionPort)(nil)
