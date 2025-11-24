package services_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/application/ports/mocks"
	"github.com/y3owk1n/neru/internal/application/services"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/errors"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

func TestHintService_ShowHints(t *testing.T) {
	// Create test elements
	testElements := []*element.Element{
		mustNewElement("elem1", image.Rect(10, 10, 50, 50)),
		mustNewElement("elem2", image.Rect(60, 10, 100, 50)),
		mustNewElement("elem3", image.Rect(10, 60, 50, 100)),
	}

	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockAccessibilityPort, *mocks.MockOverlayPort)
		setupGen      func() hint.Generator
		wantErr       bool
		wantHintCount int
		checkHints    func(*testing.T, []*hint.Hint)
		checkOverlay  func(*testing.T, *mocks.MockOverlayPort)
	}{
		{
			name: "successful hint display",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.GetClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
					return testElements, nil
				}
			},
			setupGen: func() hint.Generator {
				gen, _ := hint.NewAlphabetGenerator("asdf")

				return gen
			},
			wantErr:       false,
			wantHintCount: 3, // We have 3 test elements
			checkHints: func(t *testing.T, hints []*hint.Hint) {
				t.Helper()

				if len(hints) != 3 {
					t.Errorf("Expected 3 hints, got %d", len(hints))

					return
				}
				// Check that hints have labels (exact labels depend on generator)
				if hints[0].Label() == "" || hints[1].Label() == "" || hints[2].Label() == "" {
					t.Error("Hints should have non-empty labels")
				}
			},
			checkOverlay: func(t *testing.T, ov *mocks.MockOverlayPort) {
				t.Helper()

				if !ov.IsVisible() {
					t.Error("Overlay should be visible after ShowHints")
				}
			},
		},
		{
			name: "no elements found",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.GetClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
					return []*element.Element{}, nil
				}
			},
			setupGen: func() hint.Generator {
				gen, _ := hint.NewAlphabetGenerator("asdf")

				return gen
			},
			wantErr:       false,
			wantHintCount: 0, // No elements means no hints
			checkHints: func(t *testing.T, hints []*hint.Hint) {
				t.Helper()

				if len(hints) != 0 {
					t.Errorf("Expected 0 hints, got %d", len(hints))
				}
			},
		},
		{
			name: "accessibility error",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.GetClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
					return nil, derrors.New(
						derrors.CodeAccessibilityFailed,
						"accessibility permission denied",
					)
				}
			},
			setupGen: func() hint.Generator {
				gen, _ := hint.NewAlphabetGenerator("asdf")

				return gen
			},
			wantErr:       true,
			wantHintCount: 0,
		},
		{
			name: "large element set",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.GetClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
					// Create 100 elements
					elements := make([]*element.Element, 100)

					for index := range 100 {
						elem, _ := element.NewElement(
							element.ID("elem"+string(rune(index))),
							image.Rect(index*10, index*10, index*10+40, index*10+40),
							element.RoleButton,
						)
						elements[index] = elem
					}

					return elements, nil
				}
			},
			setupGen: func() hint.Generator {
				// Use larger character set for more hints
				gen, _ := hint.NewAlphabetGenerator("abcdefghijklmnopqrstuvwxyz")

				return gen
			},
			wantErr:       false,
			wantHintCount: 100,
			checkHints: func(t *testing.T, hints []*hint.Hint) {
				t.Helper()

				if len(hints) != 100 {
					t.Errorf("Expected 100 hints, got %d", len(hints))
				}
			},
		},
		{
			name: "single element",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.GetClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
					return []*element.Element{testElements[0]}, nil
				}
			},
			setupGen: func() hint.Generator {
				gen, _ := hint.NewAlphabetGenerator("ab")

				return gen
			},
			wantErr:       false,
			wantHintCount: 1,
			checkHints: func(t *testing.T, hints []*hint.Hint) {
				t.Helper()

				if len(hints) != 1 {
					t.Errorf("Expected 1 hint, got %d", len(hints))
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}

			if test.setupMocks != nil {
				test.setupMocks(mockAcc, mockOverlay)
			}

			generator := test.setupGen()
			logger := logger.Get()

			service := services.NewHintService(mockAcc, mockOverlay, generator, logger)

			context := context.Background()
			filter := ports.DefaultElementFilter()

			// Act
			hints, hintsErr := service.ShowHints(context, filter)

			// Assert
			if test.wantErr && hintsErr == nil {
				t.Error("ShowHints() expected error, got nil")
			}

			if !test.wantErr && hintsErr != nil {
				t.Errorf("ShowHints() unexpected error: %v", hintsErr)
			}

			if test.checkHints != nil {
				test.checkHints(t, hints)
			}

			if test.checkOverlay != nil {
				test.checkOverlay(t, mockOverlay)
			}
		})
	}
}

func TestHintService_HideHints(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(*mocks.MockOverlayPort)
		wantErr    bool
	}{
		{
			name: "successful hide",
			setupMocks: func(ov *mocks.MockOverlayPort) {
				ov.HideFunc = func(_ context.Context) error {
					return nil
				}
			},
			wantErr: false,
		},
		{
			name: "overlay hide error",
			setupMocks: func(ov *mocks.MockOverlayPort) {
				ov.HideFunc = func(_ context.Context) error {
					return derrors.New(
						derrors.CodeOverlayFailed,
						"failed to hide overlay",
					)
				}
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			generator, _ := hint.NewAlphabetGenerator("asdf")
			logger := logger.Get()

			if test.setupMocks != nil {
				test.setupMocks(mockOverlay)
			}

			service := services.NewHintService(mockAcc, mockOverlay, generator, logger)

			context := context.Background()
			hideHintsErr := service.HideHints(context)

			if (hideHintsErr != nil) != test.wantErr {
				t.Errorf("HideHints() error = %v, wantErr %v", hideHintsErr, test.wantErr)
			}

			// Only check visibility for successful hide
			if !test.wantErr && mockOverlay.IsVisible() {
				t.Error("Overlay should not be visible after successful HideHints")
			}
		})
	}
}

func TestHintService_RefreshHints(t *testing.T) {
	tests := []struct {
		name           string
		overlayVisible bool
		expectRefresh  bool
		refreshError   error
		wantErr        bool
	}{
		{
			name:           "refresh when visible",
			overlayVisible: true,
			expectRefresh:  true,
			refreshError:   nil,
			wantErr:        false,
		},
		{
			name:           "skip refresh when not visible",
			overlayVisible: false,
			expectRefresh:  false,
			refreshError:   nil,
			wantErr:        false,
		},
		{
			name:           "refresh error when visible",
			overlayVisible: true,
			expectRefresh:  true,
			refreshError:   derrors.New(derrors.CodeOverlayFailed, "overlay refresh failed"),
			wantErr:        true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}

			refreshCalled := false
			mockOverlay.IsVisibleFunc = func() bool {
				return test.overlayVisible
			}
			mockOverlay.RefreshFunc = func(_ context.Context) error {
				refreshCalled = true

				return test.refreshError
			}

			generator, _ := hint.NewAlphabetGenerator("asdf")
			logger := logger.Get()

			service := services.NewHintService(mockAcc, mockOverlay, generator, logger)

			context := context.Background()
			refreshHintsErr := service.RefreshHints(context)

			if (refreshHintsErr != nil) != test.wantErr {
				t.Errorf("RefreshHints() error = %v, wantErr %v", refreshHintsErr, test.wantErr)
			}

			if refreshCalled != test.expectRefresh {
				t.Errorf("Refresh called = %v, want %v", refreshCalled, test.expectRefresh)
			}
		})
	}
}

// Helper functions.
func mustNewElement(id string, bounds image.Rectangle) *element.Element {
	element, elementErr := element.NewElement(element.ID(id), bounds, element.RoleButton)
	if elementErr != nil {
		panic(elementErr)
	}

	return element
}
