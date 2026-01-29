//go:build integration

package services_test

import (
	"context"
	"image"
	"strconv"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// benchStubAccessibilityPort is a stub for benchmarking.
type benchStubAccessibilityPort struct {
	elements []*element.Element
}

func (s *benchStubAccessibilityPort) Health(_ context.Context) error { return nil }

func (s *benchStubAccessibilityPort) ClickableElements(
	_ context.Context,
	_ ports.ElementFilter,
) ([]*element.Element, error) {
	return s.elements, nil
}

func (s *benchStubAccessibilityPort) PerformAction(
	_ context.Context,
	_ *element.Element,
	_ action.Type,
) error {
	return nil
}

func (s *benchStubAccessibilityPort) PerformActionAtPoint(
	_ context.Context,
	_ action.Type,
	_ image.Point,
) error {
	return nil
}
func (s *benchStubAccessibilityPort) Scroll(_ context.Context, _, _ int) error { return nil }
func (s *benchStubAccessibilityPort) FocusedAppBundleID(_ context.Context) (string, error) {
	return "", nil
}

func (s *benchStubAccessibilityPort) IsAppExcluded(
	_ context.Context,
	_ string,
) bool {
	return false
}

func (s *benchStubAccessibilityPort) ScreenBounds(_ context.Context) (image.Rectangle, error) {
	return image.Rect(0, 0, 1920, 1080), nil
}

func (s *benchStubAccessibilityPort) MoveCursorToPoint(
	_ context.Context,
	_ image.Point,
	_ bool,
) error {
	return nil
}

func (s *benchStubAccessibilityPort) CursorPosition(_ context.Context) (image.Point, error) {
	return image.Point{}, nil
}
func (s *benchStubAccessibilityPort) CheckPermissions(_ context.Context) error { return nil }

// benchStubOverlayPort is a stub for benchmarking.
type benchStubOverlayPort struct{}

func (s *benchStubOverlayPort) Health(
	_ context.Context,
) error {
	return nil
}

func (s *benchStubOverlayPort) ShowHints(
	_ context.Context,
	_ []*hint.Interface,
) error {
	return nil
}

func (s *benchStubOverlayPort) ShowGrid(
	_ context.Context,
) error {
	return nil
}

func (s *benchStubOverlayPort) DrawScrollHighlight(
	_ context.Context,
	_ image.Rectangle,
	_ string,
	_ int,
) error {
	return nil
}
func (s *benchStubOverlayPort) Hide(_ context.Context) error    { return nil }
func (s *benchStubOverlayPort) IsVisible() bool                 { return false }
func (s *benchStubOverlayPort) Refresh(_ context.Context) error { return nil }

// mockGenerator is a simple mock implementation of hint.Generator for benchmarks.
type mockGenerator struct{}

func (m *mockGenerator) Generate(
	ctx context.Context,
	elements []*element.Element,
) ([]*hint.Interface, error) {
	hints := make([]*hint.Interface, 0, len(elements))
	for i, elem := range elements {
		label := strconv.Itoa(i)

		h, err := hint.NewHint(label, elem, elem.Bounds().Min)
		if err == nil {
			hints = append(hints, h)
		}
	}

	return hints, nil
}

func (m *mockGenerator) MaxHints() int {
	return 100
}

func (m *mockGenerator) Characters() string {
	return "abcdefghijklmnopqrstuvwxyz"
}

// BenchmarkHintService_ShowHints_Incremental benchmarks the incremental rendering performance.
func BenchmarkHintService_ShowHints_Incremental(b *testing.B) {
	logger := zap.NewNop()
	cfg := config.DefaultConfig()

	// Create test elements
	elem1, _ := element.NewElement(
		"btn1",
		image.Rect(0, 0, 50, 50),
		element.RoleButton,
		element.WithTitle("Button1"),
	)
	elem2, _ := element.NewElement(
		"btn2",
		image.Rect(50, 0, 100, 50),
		element.RoleButton,
		element.WithTitle("Button2"),
	)
	elem3, _ := element.NewElement(
		"lnk1",
		image.Rect(0, 50, 50, 100),
		element.RoleLink,
		element.WithTitle("Link1"),
	)
	elem4, _ := element.NewElement(
		"lnk2",
		image.Rect(50, 50, 100, 100),
		element.RoleLink,
		element.WithTitle("Link2"),
	)
	elem5, _ := element.NewElement(
		"inp1",
		image.Rect(0, 100, 100, 150),
		element.RoleTextField,
		element.WithTitle("Input1"),
	)

	accAdapter := &benchStubAccessibilityPort{
		elements: []*element.Element{elem1, elem2, elem3, elem4, elem5},
	}
	overlayAdapter := &benchStubOverlayPort{}
	generator := &mockGenerator{}

	hintService := services.NewHintService(accAdapter, overlayAdapter, generator, cfg.Hints, logger)

	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		_, err := hintService.ShowHints(ctx)
		if err != nil {
			b.Fatalf("ShowHints failed: %v", err)
		}
	}
}

// BenchmarkGridService_ShowGrid_Incremental benchmarks grid incremental rendering performance.
func BenchmarkGridService_ShowGrid_Incremental(b *testing.B) {
	logger := zap.NewNop()

	overlayAdapter := &benchStubOverlayPort{}

	gridService := services.NewGridService(overlayAdapter, logger)

	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		err := gridService.ShowGrid(ctx)
		if err != nil {
			b.Fatalf("ShowGrid failed: %v", err)
		}
	}
}
