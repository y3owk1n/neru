package services_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/ports"
	"github.com/y3owk1n/neru/internal/core/ports/mocks"
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
		checkHints    func(*testing.T, []*hint.Interface)
		checkOverlay  func(*testing.T, *mocks.MockOverlayPort)
	}{
		{
			name: "successful hint display",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.ClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
					return testElements, nil
				}
			},
			setupGen: func() hint.Generator {
				gen, _ := hint.NewAlphabetGenerator("asdf")

				return gen
			},
			wantErr:       false,
			wantHintCount: 3, // We have 3 test elements
			checkHints: func(t *testing.T, hints []*hint.Interface) {
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
				acc.ClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
					return []*element.Element{}, nil
				}
			},
			setupGen: func() hint.Generator {
				gen, _ := hint.NewAlphabetGenerator("asdf")

				return gen
			},
			wantErr:       false,
			wantHintCount: 0, // No elements means no hints
			checkHints: func(t *testing.T, hints []*hint.Interface) {
				t.Helper()

				if len(hints) != 0 {
					t.Errorf("Expected 0 hints, got %d", len(hints))
				}
			},
		},
		{
			name: "accessibility error",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.ClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
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
				acc.ClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
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
			checkHints: func(t *testing.T, hints []*hint.Interface) {
				t.Helper()

				if len(hints) != 100 {
					t.Errorf("Expected 100 hints, got %d", len(hints))
				}
			},
		},
		{
			name: "single element",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.ClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
					return []*element.Element{testElements[0]}, nil
				}
			},
			setupGen: func() hint.Generator {
				gen, _ := hint.NewAlphabetGenerator("ab")

				return gen
			},
			wantErr:       false,
			wantHintCount: 1,
			checkHints: func(t *testing.T, hints []*hint.Interface) {
				t.Helper()

				if len(hints) != 1 {
					t.Errorf("Expected 1 hint, got %d", len(hints))
				}
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockAcc, mockOverlay)
			}

			generator := testCase.setupGen()
			logger := logger.Get()

			service := services.NewHintService(
				mockAcc,
				mockOverlay,
				generator,
				config.HintsConfig{},
				logger,
			)

			ctx := context.Background()

			// Act
			hints, hintsErr := service.ShowHints(ctx)

			// Assert
			if testCase.wantErr && hintsErr == nil {
				t.Error("ShowHints() expected error, got nil")
			}

			if !testCase.wantErr && hintsErr != nil {
				t.Errorf("ShowHints() unexpected error: %v", hintsErr)
			}

			if testCase.checkHints != nil {
				testCase.checkHints(t, hints)
			}

			if testCase.checkOverlay != nil {
				testCase.checkOverlay(t, mockOverlay)
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			generator, _ := hint.NewAlphabetGenerator("asdf")
			logger := logger.Get()

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockOverlay)
			}

			service := services.NewHintService(
				mockAcc,
				mockOverlay,
				generator,
				config.HintsConfig{},
				logger,
			)

			ctx := context.Background()
			hideHintsErr := service.HideHints(ctx)

			if (hideHintsErr != nil) != testCase.wantErr {
				t.Errorf("HideHints() error = %v, wantErr %v", hideHintsErr, testCase.wantErr)
			}

			// Only check visibility for successful hide
			if !testCase.wantErr && mockOverlay.IsVisible() {
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}

			refreshCalled := false
			mockOverlay.IsVisibleFunc = func() bool {
				return testCase.overlayVisible
			}
			mockOverlay.RefreshFunc = func(_ context.Context) error {
				refreshCalled = true

				return testCase.refreshError
			}

			generator, _ := hint.NewAlphabetGenerator("asdf")
			logger := logger.Get()

			service := services.NewHintService(
				mockAcc,
				mockOverlay,
				generator,
				config.HintsConfig{},
				logger,
			)

			ctx := context.Background()
			refreshHintsErr := service.RefreshHints(ctx)

			if (refreshHintsErr != nil) != testCase.wantErr {
				t.Errorf("RefreshHints() error = %v, wantErr %v", refreshHintsErr, testCase.wantErr)
			}

			if refreshCalled != testCase.expectRefresh {
				t.Errorf("Refresh called = %v, want %v", refreshCalled, testCase.expectRefresh)
			}
		})
	}
}

func TestHintService_UpdateGenerator(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	logger := logger.Get()

	// Initial generator
	initialGen, _ := hint.NewAlphabetGenerator("abcd")
	service := services.NewHintService(
		mockAcc,
		mockOverlay,
		initialGen,
		config.HintsConfig{},
		logger,
	)

	// Update with new generator
	newGen, _ := hint.NewAlphabetGenerator("efgh")
	ctx := context.Background()
	service.UpdateGenerator(ctx, newGen)

	// Test with nil generator (should not crash)
	service.UpdateGenerator(ctx, nil)
}

func TestHintService_Health(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	generator, _ := hint.NewAlphabetGenerator("abcd")
	logger := logger.Get()

	service := services.NewHintService(
		mockAcc,
		mockOverlay,
		generator,
		config.HintsConfig{},
		logger,
	)

	// Setup mocks
	mockAcc.HealthFunc = func(_ context.Context) error {
		return nil
	}
	mockOverlay.HealthFunc = func(_ context.Context) error {
		return derrors.New(derrors.CodeOverlayFailed, "overlay unhealthy")
	}

	ctx := context.Background()
	health := service.Health(ctx)

	// Check that health map has both keys
	if len(health) != 2 {
		t.Errorf("Health() returned %d entries, want 2", len(health))
	}

	if _, ok := health["accessibility"]; !ok {
		t.Error("Health() missing 'accessibility' key")
	}

	if _, ok := health["overlay"]; !ok {
		t.Error("Health() missing 'overlay' key")
	}

	// Check that overlay has error
	if health["overlay"] == nil {
		t.Error("Health() overlay should have error")
	}

	if health["accessibility"] != nil {
		t.Error("Health() accessibility should not have error")
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
