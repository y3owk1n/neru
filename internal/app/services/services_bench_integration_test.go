//go:build integration

package services_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/ports/mocks"
	"go.uber.org/zap"
)

// BenchmarkHintService_ShowHints_Incremental benchmarks the incremental rendering performance
func BenchmarkHintService_ShowHints_Incremental(b *testing.B) {
	// Setup
	logger := zap.NewNop()
	cfg := config.DefaultConfig()

	// Create mock ports that simulate real behavior
	accAdapter := &mocks.MockAccessibilityPort{}
	overlayAdapter := &mocks.MockOverlayPort{}

	hintService := services.NewHintService(accAdapter, overlayAdapter, nil, cfg.Hints, logger)

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// This will exercise the incremental rendering logic
		// The benchmark measures the performance of the hint display with incremental updates
		_, _ = hintService.ShowHints(ctx)
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
		gridService.ShowGrid(ctx)
	}
}

// BenchmarkHintService_IncrementalUpdateFiltering benchmarks the element filtering logic
func BenchmarkHintService_IncrementalUpdateFiltering(b *testing.B) {
	// This benchmark specifically targets the incremental update filtering
	// which is a key performance optimization in the hints system

	logger := zap.NewNop()
	cfg := config.DefaultConfig()

	accAdapter := &mocks.MockAccessibilityPort{}
	overlayAdapter := &mocks.MockOverlayPort{}

	hintService := services.NewHintService(accAdapter, overlayAdapter, nil, cfg.Hints, logger)

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Exercise the filtering logic that determines what needs incremental updates
		_, _ = hintService.ShowHints(ctx)
	}
}
