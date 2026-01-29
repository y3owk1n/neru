package services_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// stubAccessibilityPort is a simple stub for AccessibilityPort.
type stubAccessibilityPort struct {
	elements []*element.Element
	err      error
}

func (s *stubAccessibilityPort) Health(_ context.Context) error { return nil }

func (s *stubAccessibilityPort) ClickableElements(
	_ context.Context,
	_ ports.ElementFilter,
) ([]*element.Element, error) {
	return s.elements, s.err
}

func (s *stubAccessibilityPort) PerformAction(
	_ context.Context,
	_ *element.Element,
	_ action.Type,
) error {
	return nil
}

func (s *stubAccessibilityPort) PerformActionAtPoint(
	_ context.Context,
	_ action.Type,
	_ image.Point,
) error {
	return nil
}
func (s *stubAccessibilityPort) Scroll(_ context.Context, _, _ int) error { return nil }
func (s *stubAccessibilityPort) FocusedAppBundleID(_ context.Context) (string, error) {
	return "", nil
}
func (s *stubAccessibilityPort) IsAppExcluded(_ context.Context, _ string) bool { return false }
func (s *stubAccessibilityPort) ScreenBounds(_ context.Context) (image.Rectangle, error) {
	return image.Rect(0, 0, 1920, 1080), nil
}

func (s *stubAccessibilityPort) MoveCursorToPoint(_ context.Context, _ image.Point, _ bool) error {
	return nil
}

func (s *stubAccessibilityPort) CursorPosition(_ context.Context) (image.Point, error) {
	return image.Point{}, nil
}
func (s *stubAccessibilityPort) CheckPermissions(_ context.Context) error { return nil }

// stubOverlayPort is a simple stub for OverlayPort.
type stubOverlayPort struct {
	visible     bool
	hideErr     error
	refreshErr  error
	healthErr   error
	hideFunc    func(context.Context) error
	refreshFunc func(context.Context) error
	visibleFunc func() bool
}

func (s *stubOverlayPort) Health(_ context.Context) error { return s.healthErr }
func (s *stubOverlayPort) ShowHints(_ context.Context, _ []*hint.Interface) error {
	s.visible = true

	return nil
}
func (s *stubOverlayPort) ShowGrid(_ context.Context) error { return nil }

func (s *stubOverlayPort) DrawScrollHighlight(
	_ context.Context,
	_ image.Rectangle,
	_ string,
	_ int,
) error {
	return nil
}

func (s *stubOverlayPort) Hide(ctx context.Context) error {
	if s.hideFunc != nil {
		return s.hideFunc(ctx)
	}

	s.visible = false

	return s.hideErr
}

func (s *stubOverlayPort) IsVisible() bool {
	if s.visibleFunc != nil {
		return s.visibleFunc()
	}

	return s.visible
}

func (s *stubOverlayPort) Refresh(ctx context.Context) error {
	if s.refreshFunc != nil {
		return s.refreshFunc(ctx)
	}

	return s.refreshErr
}

func TestHintService_ShowHints(t *testing.T) {
	// Create test elements
	testElements := []*element.Element{
		mustNewElement("elem1", image.Rect(10, 10, 50, 50)),
		mustNewElement("elem2", image.Rect(60, 10, 100, 50)),
		mustNewElement("elem3", image.Rect(10, 60, 50, 100)),
	}

	tests := []struct {
		name          string
		elements      []*element.Element
		accErr        error
		config        config.HintsConfig
		wantErr       bool
		wantHintCount int
	}{
		{
			name:          "successful hint display",
			elements:      testElements,
			wantErr:       false,
			wantHintCount: 3,
		},
		{
			name:          "no elements found",
			elements:      []*element.Element{},
			wantErr:       false,
			wantHintCount: 0,
		},
		{
			name:     "accessibility error",
			elements: nil,
			accErr: derrors.New(
				derrors.CodeAccessibilityFailed,
				"accessibility permission denied",
			),
			wantErr:       true,
			wantHintCount: 0,
		},
		{
			name:          "single element",
			elements:      []*element.Element{testElements[0]},
			wantErr:       false,
			wantHintCount: 1,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			stubAcc := &stubAccessibilityPort{
				elements: testCase.elements,
				err:      testCase.accErr,
			}
			stubOv := &stubOverlayPort{}

			generator, _ := hint.NewAlphabetGenerator("asdf")
			logger := logger.Get()

			service := services.NewHintService(
				stubAcc,
				stubOv,
				generator,
				testCase.config,
				logger,
			)

			ctx := context.Background()
			hints, hintsErr := service.ShowHints(ctx)

			if testCase.wantErr && hintsErr == nil {
				t.Error("ShowHints() expected error, got nil")
			}

			if !testCase.wantErr && hintsErr != nil {
				t.Errorf("ShowHints() unexpected error: %v", hintsErr)
			}

			if len(hints) != testCase.wantHintCount {
				t.Errorf("Expected %d hints, got %d", testCase.wantHintCount, len(hints))
			}
		})
	}
}

func TestHintService_HideHints(t *testing.T) {
	tests := []struct {
		name    string
		hideErr error
		wantErr bool
	}{
		{
			name:    "successful hide",
			hideErr: nil,
			wantErr: false,
		},
		{
			name:    "overlay hide error",
			hideErr: derrors.New(derrors.CodeOverlayFailed, "failed to hide overlay"),
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			stubAcc := &stubAccessibilityPort{}
			stubOv := &stubOverlayPort{hideErr: testCase.hideErr}
			generator, _ := hint.NewAlphabetGenerator("asdf")
			logger := logger.Get()

			service := services.NewHintService(
				stubAcc,
				stubOv,
				generator,
				config.HintsConfig{},
				logger,
			)

			ctx := context.Background()
			hideHintsErr := service.HideHints(ctx)

			if (hideHintsErr != nil) != testCase.wantErr {
				t.Errorf("HideHints() error = %v, wantErr %v", hideHintsErr, testCase.wantErr)
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
			stubAcc := &stubAccessibilityPort{}

			refreshCalled := false
			stubOv := &stubOverlayPort{
				visibleFunc: func() bool { return testCase.overlayVisible },
				refreshFunc: func(_ context.Context) error {
					refreshCalled = true

					return testCase.refreshError
				},
			}

			generator, _ := hint.NewAlphabetGenerator("asdf")
			logger := logger.Get()

			service := services.NewHintService(
				stubAcc,
				stubOv,
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
	stubAcc := &stubAccessibilityPort{}
	stubOv := &stubOverlayPort{}
	logger := logger.Get()

	initialGen, _ := hint.NewAlphabetGenerator("abcd")

	service := services.NewHintService(
		stubAcc,
		stubOv,
		initialGen,
		config.HintsConfig{},
		logger,
	)

	newGen, _ := hint.NewAlphabetGenerator("efgh")
	ctx := context.Background()
	service.UpdateGenerator(ctx, newGen)

	// Test with nil generator (should not crash)
	service.UpdateGenerator(ctx, nil)
}

func TestHintService_Health(t *testing.T) {
	stubAcc := &stubAccessibilityPort{}
	stubOv := &stubOverlayPort{
		healthErr: derrors.New(derrors.CodeOverlayFailed, "overlay unhealthy"),
	}
	generator, _ := hint.NewAlphabetGenerator("abcd")
	logger := logger.Get()

	service := services.NewHintService(
		stubAcc,
		stubOv,
		generator,
		config.HintsConfig{},
		logger,
	)

	ctx := context.Background()
	health := service.Health(ctx)

	if len(health) != 2 {
		t.Errorf("Health() returned %d entries, want 2", len(health))
	}

	if _, ok := health["accessibility"]; !ok {
		t.Error("Health() missing 'accessibility' key")
	}

	if _, ok := health["overlay"]; !ok {
		t.Error("Health() missing 'overlay' key")
	}

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
