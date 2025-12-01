//go:build integration

package services_test

import (
	"context"
	"fmt"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/ports"
	"github.com/y3owk1n/neru/internal/core/ports/mocks"
	"go.uber.org/zap"
)

// mockGenerator is a simple mock implementation of hint.Generator for benchmarks
type mockGenerator struct{}

func (m *mockGenerator) Generate(
	ctx context.Context,
	elements []*element.Element,
) ([]*hint.Interface, error) {
	// Return hints for the provided elements to exercise incremental rendering
	hints := make([]*hint.Interface, 0, len(elements))
	for i, elem := range elements {
		label := fmt.Sprintf("%d", i)
		h, _ := hint.NewHint(label, elem, elem.Bounds().Min)
		hints = append(hints, h)
	}
	return hints, nil
}

func (m *mockGenerator) MaxHints() int {
	return 100
}

func (m *mockGenerator) Characters() string {
	return "abcdefghijklmnopqrstuvwxyz"
}

// BenchmarkHintService_ShowHints_Incremental benchmarks the incremental rendering performance
func BenchmarkHintService_ShowHints_Incremental(b *testing.B) {
	// Setup
	logger := zap.NewNop()
	cfg := config.DefaultConfig()

	// Create mock ports that simulate real behavior
	accAdapter := &mocks.MockAccessibilityPort{
		ClickableElementsFunc: func(ctx context.Context, filter ports.ElementFilter) ([]*element.Element, error) {
			// Return a few mock elements to exercise hint generation
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
			return []*element.Element{elem1, elem2, elem3, elem4, elem5}, nil
		},
	}
	overlayAdapter := &mocks.MockOverlayPort{}
	generator := &mockGenerator{}

	hintService := services.NewHintService(accAdapter, overlayAdapter, generator, cfg.Hints, logger)

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// This will exercise the incremental rendering logic
		// The benchmark measures the performance of the hint display with incremental updates
		if _, err := hintService.ShowHints(ctx); err != nil {
			b.Fatalf("ShowHints failed: %v", err)
		}
	}
}

// BenchmarkGridService_ShowGrid_Incremental benchmarks grid incremental rendering performance
func BenchmarkGridService_ShowGrid_Incremental(b *testing.B) {
	// Setup
	logger := zap.NewNop()

	// Create mock overlay port
	overlayAdapter := &mocks.MockOverlayPort{}

	gridService := services.NewGridService(overlayAdapter, logger)

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// This will exercise the grid incremental rendering logic
		// Measures performance of grid display with incremental updates
		if err := gridService.ShowGrid(ctx); err != nil {
			b.Fatalf("ShowGrid failed: %v", err)
		}
	}
}
