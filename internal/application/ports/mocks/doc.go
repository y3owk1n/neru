// Package mocks provides mock implementations of port interfaces for testing.
//
// These mocks use function fields to allow tests to customize behavior.
// Each mock provides sensible defaults when function fields are nil.
//
// # Usage
//
//	mockAccessibility := &mocks.MockAccessibilityPort{
//		GetClickableElementsFunc: func(ctx context.Context, filter ports.ElementFilter) ([]*element.Element, error) {
//			return testElements, nil
//		},
//	}
//
//	service := services.NewHintService(mockAccessibility, mockOverlay, generator, logger)
//	err := service.ShowHints(ctx, filter)
//
// # Design Principles
//
//   - Function Fields: Each method has a corresponding function field
//   - Sensible Defaults: Nil function fields return zero values or no-ops
//   - State Tracking: Some mocks track state for verification (e.g., MockOverlayPort.visible)
//   - Interface Compliance: All mocks verify interface implementation with var _ checks
package mocks
