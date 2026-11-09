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
// every method. Set the Func fields to model a working Vision backend.
type MockVisionPort struct {
	HealthFunc         func(context.Context) error
	ReadTextFunc       func(context.Context, image.Rectangle) (string, error)
	ReadLinesFunc      func(context.Context, image.Rectangle, ports.TextLevel) ([][]ports.TextPiece, error)
	DetectElementsFunc func(
		context.Context,
		image.Rectangle,
		config.HintsVisionConfig,
		bool,
	) ([]*element.Element, error)
	CaptureScreenFunc  func(context.Context) (*image.RGBA, error)
	DetectContoursFunc func(context.Context, image.Rectangle, config.HintsContourConfig) ([]*element.Element, error)
	DetectSegmentsFunc func(context.Context, image.Rectangle, config.HintsSegmentConfig) ([]*element.Element, error)
	// ExcludesOwnWindows is what CaptureExcludesOwnWindows answers.
	ExcludesOwnWindows bool
	// AccurateReadSame makes AccurateReadDiffers answer false. The zero value
	// has the accurate read differ, as on macOS.
	AccurateReadSame bool
	// Preparing is what PrepareAccurateRead returns. Nil has it return a
	// closed channel, as where nothing compiles.
	Preparing chan struct{}

	mu       sync.Mutex
	detectN  int
	captureN int
	contourN int
	segmentN int
	prepareN int
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

// DetectContours implements ports.VisionPort.
func (m *MockVisionPort) DetectContours(
	ctx context.Context,
	screenBounds image.Rectangle,
	cfg config.HintsContourConfig,
) ([]*element.Element, error) {
	m.mu.Lock()
	m.contourN++
	m.mu.Unlock()

	if m.DetectContoursFunc != nil {
		return m.DetectContoursFunc(ctx, screenBounds, cfg)
	}

	return nil, derrors.New(
		derrors.CodeNotSupported,
		"contour detection is not supported by this mock",
	)
}

// DetectSegments implements ports.VisionPort.
func (m *MockVisionPort) DetectSegments(
	ctx context.Context,
	screenBounds image.Rectangle,
	cfg config.HintsSegmentConfig,
) ([]*element.Element, error) {
	m.mu.Lock()
	m.segmentN++
	m.mu.Unlock()

	if m.DetectSegmentsFunc != nil {
		return m.DetectSegmentsFunc(ctx, screenBounds, cfg)
	}

	return nil, derrors.New(
		derrors.CodeNotSupported,
		"segment detection is not supported by this mock",
	)
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

// DetectContoursCallCount reports how many times DetectContours was called.
func (m *MockVisionPort) DetectContoursCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.contourN
}

// DetectSegmentsAsRead implements ports.VisionPort, as DetectSegments.
func (m *MockVisionPort) DetectSegmentsAsRead(
	ctx context.Context,
	region image.Rectangle,
	cfg config.HintsSegmentConfig,
) ([]*element.Element, error) {
	return m.DetectSegments(ctx, region, cfg)
}

// DetectSegmentsCallCount reports how many times DetectSegments was called.
func (m *MockVisionPort) DetectSegmentsCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.segmentN
}

// CaptureExcludesOwnWindows implements ports.VisionPort.
func (m *MockVisionPort) CaptureExcludesOwnWindows() bool { return m.ExcludesOwnWindows }

// ReadLinesOnceChanged implements ports.VisionPort. It reads as ReadLines
// does at TextFast, through ReadLinesFunc.
func (m *MockVisionPort) ReadLinesOnceChanged(
	ctx context.Context,
	region image.Rectangle,
) ([][]ports.TextPiece, error) {
	return m.ReadLines(ctx, region, ports.TextFast)
}

// AccurateReadDiffers implements ports.VisionPort.
func (m *MockVisionPort) AccurateReadDiffers() bool { return !m.AccurateReadSame }

// PrepareAccurateRead implements ports.VisionPort. It returns Preparing, or
// a closed channel when that is nil.
func (m *MockVisionPort) PrepareAccurateRead() <-chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.prepareN++

	if m.Preparing != nil {
		return m.Preparing
	}

	prepared := make(chan struct{})
	close(prepared)

	return prepared
}

// PrepareCallCount reports how many times PrepareAccurateRead was called.
func (m *MockVisionPort) PrepareCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.prepareN
}

// ForgetLastRead implements ports.VisionPort. It remembers nothing to forget.
func (m *MockVisionPort) ForgetLastRead() {}

// ReadText implements ports.VisionPort. Without ReadTextFunc it reports
// CodeNotSupported, as Linux and Windows do.
func (m *MockVisionPort) ReadText(ctx context.Context, rect image.Rectangle) (string, error) {
	if m.ReadTextFunc != nil {
		return m.ReadTextFunc(ctx, rect)
	}

	return "", derrors.New(derrors.CodeNotSupported, "text reading not supported")
}

// ReadLines implements ports.VisionPort. Without ReadLinesFunc it reports
// CodeNotSupported, as Linux and Windows do.
func (m *MockVisionPort) ReadLines(
	ctx context.Context,
	region image.Rectangle,
	level ports.TextLevel,
) ([][]ports.TextPiece, error) {
	if m.ReadLinesFunc != nil {
		return m.ReadLinesFunc(ctx, region, level)
	}

	return nil, derrors.New(derrors.CodeNotSupported, "reading lines not supported")
}

// Ensure MockVisionPort implements ports.VisionPort.
var _ ports.VisionPort = (*MockVisionPort)(nil)
