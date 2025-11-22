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
			setupMocks: func(acc *mocks.MockAccessibilityPort, ov *mocks.MockOverlayPort) {
				acc.GetClickableElementsFunc = func(ctx context.Context, filter ports.ElementFilter) ([]*element.Element, error) {
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
				if !ov.IsVisible() {
					t.Error("Overlay should be visible after ShowHints")
				}
			},
		},
		{
			name: "no elements found",
			setupMocks: func(acc *mocks.MockAccessibilityPort, ov *mocks.MockOverlayPort) {
				acc.GetClickableElementsFunc = func(ctx context.Context, filter ports.ElementFilter) ([]*element.Element, error) {
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
				if len(hints) != 0 {
					t.Errorf("Expected 0 hints, got %d", len(hints))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}

			if tt.setupMocks != nil {
				tt.setupMocks(mockAcc, mockOverlay)
			}

			gen := tt.setupGen()
			log := logger.Get()

			service := services.NewHintService(mockAcc, mockOverlay, gen, log)

			ctx := context.Background()
			filter := ports.DefaultElementFilter()

			// Act
			hints, err := service.ShowHints(ctx, filter)

			// Assert
			if tt.wantErr && err == nil {
				t.Error("ShowHints() expected error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("ShowHints() unexpected error: %v", err)
			}

			if tt.checkHints != nil {
				tt.checkHints(t, hints)
			}

			if tt.checkOverlay != nil {
				tt.checkOverlay(t, mockOverlay)
			}
		})
	}
}

func TestHintService_HideHints(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	gen, _ := hint.NewAlphabetGenerator("asdf")
	log := logger.Get()

	service := services.NewHintService(mockAcc, mockOverlay, gen, log)

	ctx := context.Background()
	err := service.HideHints(ctx)
	if err != nil {
		t.Errorf("HideHints() unexpected error: %v", err)
	}

	if mockOverlay.IsVisible() {
		t.Error("Overlay should not be visible after HideHints")
	}
}

func TestHintService_RefreshHints(t *testing.T) {
	tests := []struct {
		name           string
		overlayVisible bool
		expectRefresh  bool
	}{
		{
			name:           "refresh when visible",
			overlayVisible: true,
			expectRefresh:  true,
		},
		{
			name:           "skip refresh when not visible",
			overlayVisible: false,
			expectRefresh:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}

			refreshCalled := false
			mockOverlay.IsVisibleFunc = func() bool {
				return tt.overlayVisible
			}
			mockOverlay.RefreshFunc = func(ctx context.Context) error {
				refreshCalled = true
				return nil
			}

			gen, _ := hint.NewAlphabetGenerator("asdf")
			log := logger.Get()

			service := services.NewHintService(mockAcc, mockOverlay, gen, log)

			ctx := context.Background()
			err := service.RefreshHints(ctx)
			if err != nil {
				t.Errorf("RefreshHints() unexpected error: %v", err)
			}

			if refreshCalled != tt.expectRefresh {
				t.Errorf("Refresh called = %v, want %v", refreshCalled, tt.expectRefresh)
			}
		})
	}
}

// Helper functions
func mustNewElement(id string, bounds image.Rectangle) *element.Element {
	elem, err := element.NewElement(element.ID(id), bounds, element.RoleButton)
	if err != nil {
		panic(err)
	}
	return elem
}

func mustNewHint(label string, elem *element.Element) *hint.Hint {
	h, err := hint.NewHint(label, elem, image.Point{})
	if err != nil {
		panic(err)
	}
	return h
}
