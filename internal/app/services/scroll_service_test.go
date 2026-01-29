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

// scrollStubAccessibilityPort is a stub for AccessibilityPort used in scroll tests.
type scrollStubAccessibilityPort struct {
	screenBounds        image.Rectangle
	scrollErr           error
	screenBoundsErr     error
	scrollDeltaRecorder func(deltaX, deltaY int)
}

func (s *scrollStubAccessibilityPort) Health(_ context.Context) error { return nil }

func (s *scrollStubAccessibilityPort) ClickableElements(
	_ context.Context,
	_ ports.ElementFilter,
) ([]*element.Element, error) {
	return nil, nil
}

func (s *scrollStubAccessibilityPort) PerformAction(
	_ context.Context,
	_ *element.Element,
	_ action.Type,
) error {
	return nil
}

func (s *scrollStubAccessibilityPort) PerformActionAtPoint(
	_ context.Context,
	_ action.Type,
	_ image.Point,
) error {
	return nil
}

func (s *scrollStubAccessibilityPort) Scroll(_ context.Context, deltaX, deltaY int) error {
	if s.scrollDeltaRecorder != nil {
		s.scrollDeltaRecorder(deltaX, deltaY)
	}

	return s.scrollErr
}

func (s *scrollStubAccessibilityPort) FocusedAppBundleID(_ context.Context) (string, error) {
	return "", nil
}

func (s *scrollStubAccessibilityPort) IsAppExcluded(
	_ context.Context,
	_ string,
) bool {
	return false
}

func (s *scrollStubAccessibilityPort) ScreenBounds(_ context.Context) (image.Rectangle, error) {
	if s.screenBounds.Empty() {
		return image.Rect(0, 0, 1920, 1080), s.screenBoundsErr
	}

	return s.screenBounds, s.screenBoundsErr
}

func (s *scrollStubAccessibilityPort) MoveCursorToPoint(
	_ context.Context,
	_ image.Point,
	_ bool,
) error {
	return nil
}

func (s *scrollStubAccessibilityPort) CursorPosition(_ context.Context) (image.Point, error) {
	return image.Point{}, nil
}
func (s *scrollStubAccessibilityPort) CheckPermissions(_ context.Context) error { return nil }

// scrollStubOverlayPort is a stub for OverlayPort used in scroll tests.
type scrollStubOverlayPort struct {
	visible             bool
	hideErr             error
	drawScrollErr       error
	drawnRect           image.Rectangle
	drawScrollHighlight func(ctx context.Context, rect image.Rectangle, color string, width int) error
}

func (s *scrollStubOverlayPort) Health(_ context.Context) error { return nil }
func (s *scrollStubOverlayPort) ShowHints(_ context.Context, _ []*hint.Interface) error {
	return nil
}
func (s *scrollStubOverlayPort) ShowGrid(_ context.Context) error { return nil }

func (s *scrollStubOverlayPort) DrawScrollHighlight(
	ctx context.Context,
	rect image.Rectangle,
	color string,
	width int,
) error {
	if s.drawScrollHighlight != nil {
		return s.drawScrollHighlight(ctx, rect, color, width)
	}

	s.drawnRect = rect

	return s.drawScrollErr
}

func (s *scrollStubOverlayPort) Hide(_ context.Context) error {
	s.visible = false

	return s.hideErr
}
func (s *scrollStubOverlayPort) IsVisible() bool { return s.visible }
func (s *scrollStubOverlayPort) Refresh(_ context.Context) error {
	return nil
}

func TestScrollService_Scroll(t *testing.T) {
	tests := []struct {
		name       string
		direction  services.ScrollDirection
		amount     services.ScrollAmount
		scrollErr  error
		wantErr    bool
		checkDelta func(t *testing.T, deltaX, deltaY int)
	}{
		{
			name:      "scroll down char",
			direction: services.ScrollDirectionDown,
			amount:    services.ScrollAmountChar,
			wantErr:   false,
			checkDelta: func(t *testing.T, deltaX, deltaY int) {
				t.Helper()

				if deltaY >= 0 {
					t.Errorf("Expected negative deltaY for scroll down, got %d", deltaY)
				}
			},
		},
		{
			name:      "scroll up char",
			direction: services.ScrollDirectionUp,
			amount:    services.ScrollAmountChar,
			wantErr:   false,
			checkDelta: func(t *testing.T, deltaX, deltaY int) {
				t.Helper()

				if deltaY <= 0 {
					t.Errorf("Expected positive deltaY for scroll up, got %d", deltaY)
				}
			},
		},
		{
			name:      "scroll left char",
			direction: services.ScrollDirectionLeft,
			amount:    services.ScrollAmountChar,
			wantErr:   false,
			checkDelta: func(t *testing.T, deltaX, deltaY int) {
				t.Helper()

				if deltaX <= 0 {
					t.Errorf("Expected positive deltaX for scroll left, got %d", deltaX)
				}
			},
		},
		{
			name:      "scroll right char",
			direction: services.ScrollDirectionRight,
			amount:    services.ScrollAmountChar,
			wantErr:   false,
			checkDelta: func(t *testing.T, deltaX, deltaY int) {
				t.Helper()

				if deltaX >= 0 {
					t.Errorf("Expected negative deltaX for scroll right, got %d", deltaX)
				}
			},
		},
		{
			name:      "accessibility error",
			direction: services.ScrollDirectionDown,
			amount:    services.ScrollAmountChar,
			scrollErr: derrors.New(derrors.CodeAccessibilityFailed, "scroll permission denied"),
			wantErr:   true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var recordedDeltaX, recordedDeltaY int

			stubAcc := &scrollStubAccessibilityPort{
				scrollErr: testCase.scrollErr,
				scrollDeltaRecorder: func(deltaX, deltaY int) {
					recordedDeltaX = deltaX
					recordedDeltaY = deltaY
				},
			}
			stubOv := &scrollStubOverlayPort{}
			cfg := config.ScrollConfig{
				ScrollStep:     10,
				ScrollStepHalf: 30,
				ScrollStepFull: 50,
			}
			logger := logger.Get()

			service := services.NewScrollService(stubAcc, stubOv, cfg, logger)
			ctx := context.Background()

			scrollErr := service.Scroll(ctx, testCase.direction, testCase.amount)

			if (scrollErr != nil) != testCase.wantErr {
				t.Errorf("Scroll() error = %v, wantErr %v", scrollErr, testCase.wantErr)
			}

			if testCase.checkDelta != nil && !testCase.wantErr {
				testCase.checkDelta(t, recordedDeltaX, recordedDeltaY)
			}
		})
	}
}

func TestScrollService_ShowScrollOverlay(t *testing.T) {
	tests := []struct {
		name            string
		screenBoundsErr error
		drawErr         error
		wantErr         bool
	}{
		{
			name:    "successful show",
			wantErr: false,
		},
		{
			name: "screen bounds error",
			screenBoundsErr: derrors.New(
				derrors.CodeAccessibilityFailed,
				"failed to get screen bounds",
			),
			wantErr: true,
		},
		{
			name:    "overlay draw error",
			drawErr: derrors.New(derrors.CodeOverlayFailed, "failed to draw scroll highlight"),
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			stubAcc := &scrollStubAccessibilityPort{
				screenBoundsErr: testCase.screenBoundsErr,
			}
			stubOv := &scrollStubOverlayPort{
				drawScrollErr: testCase.drawErr,
			}
			cfg := config.ScrollConfig{
				HighlightColor: "#ff0000",
				HighlightWidth: 5,
			}
			logger := logger.Get()

			service := services.NewScrollService(stubAcc, stubOv, cfg, logger)
			ctx := context.Background()

			showScrollOverlayErr := service.Show(ctx)

			if (showScrollOverlayErr != nil) != testCase.wantErr {
				t.Errorf("Show() error = %v, wantErr %v", showScrollOverlayErr, testCase.wantErr)
			}
		})
	}
}

func TestScrollService_Hide(t *testing.T) {
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
			stubAcc := &scrollStubAccessibilityPort{}
			stubOv := &scrollStubOverlayPort{hideErr: testCase.hideErr}
			cfg := config.ScrollConfig{}
			logger := logger.Get()

			service := services.NewScrollService(stubAcc, stubOv, cfg, logger)
			ctx := context.Background()

			hideScrollOverlayErr := service.Hide(ctx)

			if (hideScrollOverlayErr != nil) != testCase.wantErr {
				t.Errorf("Hide() error = %v, wantErr %v", hideScrollOverlayErr, testCase.wantErr)
			}
		})
	}
}

func TestScrollService_UpdateConfig(t *testing.T) {
	stubAcc := &scrollStubAccessibilityPort{}
	stubOv := &scrollStubOverlayPort{}
	logger := logger.Get()

	initialConfig := config.ScrollConfig{
		ScrollStep:     50,
		ScrollStepFull: 1000,
	}
	service := services.NewScrollService(stubAcc, stubOv, initialConfig, logger)

	newConfig := config.ScrollConfig{
		ScrollStep:     100,
		ScrollStepFull: 2000,
	}
	ctx := context.Background()
	service.UpdateConfig(ctx, newConfig)

	// Ensure it doesn't crash - config is private
}
