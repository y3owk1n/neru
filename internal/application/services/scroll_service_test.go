package services_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/application/ports/mocks"
	"github.com/y3owk1n/neru/internal/application/services"
	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/errors"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

func TestScrollService_Scroll(t *testing.T) {
	tests := []struct {
		name       string
		direction  services.ScrollDirection
		amount     services.ScrollAmount
		setupMocks func(*mocks.MockAccessibilityPort)
		wantErr    bool
	}{
		{
			name:      "scroll down char",
			direction: services.ScrollDirectionDown,
			amount:    services.ScrollAmountChar,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int) error {
					if deltaY >= 0 {
						t.Errorf("Expected negative deltaY for scroll down, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "scroll up char",
			direction: services.ScrollDirectionUp,
			amount:    services.ScrollAmountChar,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int) error {
					if deltaY <= 0 {
						t.Errorf("Expected positive deltaY for scroll up, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "scroll down full",
			direction: services.ScrollDirectionDown,
			amount:    services.ScrollAmountHalfPage,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int) error {
					// Down = negative deltaY
					if deltaY >= 0 {
						t.Errorf("Expected negative deltaY for scroll down, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "scroll up full",
			direction: services.ScrollDirectionUp,
			amount:    services.ScrollAmountHalfPage,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int) error {
					// Up = positive deltaY
					if deltaY <= 0 {
						t.Errorf("Expected positive deltaY for scroll up, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "scroll left char",
			direction: services.ScrollDirectionLeft,
			amount:    services.ScrollAmountChar,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, _ int) error {
					// Left = positive deltaX
					if deltaX <= 0 {
						t.Errorf("Expected positive deltaX for scroll left, got %d", deltaX)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "scroll right char",
			direction: services.ScrollDirectionRight,
			amount:    services.ScrollAmountChar,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, _ int) error {
					// Right = negative deltaX
					if deltaX >= 0 {
						t.Errorf("Expected negative deltaX for scroll right, got %d", deltaX)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "accessibility error",
			direction: services.ScrollDirectionDown,
			amount:    services.ScrollAmountChar,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, _ int) error {
					return derrors.New(
						derrors.CodeAccessibilityFailed,
						"scroll permission denied",
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
			config := config.ScrollConfig{
				ScrollStep:     10,
				ScrollStepHalf: 30,
				ScrollStepFull: 50,
			}
			logger := logger.Get()

			if test.setupMocks != nil {
				test.setupMocks(mockAcc)
			}

			service := services.NewScrollService(mockAcc, mockOverlay, config, logger)
			context := context.Background()

			scrollErr := service.Scroll(context, test.direction, test.amount)

			if (scrollErr != nil) != test.wantErr {
				t.Errorf("Scroll() error = %v, wantErr %v", scrollErr, test.wantErr)
			}
		})
	}
}

func TestScrollService_ShowScrollOverlay(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(*mocks.MockAccessibilityPort, *mocks.MockOverlayPort)
		wantErr    bool
	}{
		{
			name: "successful show",
			setupMocks: func(acc *mocks.MockAccessibilityPort, ov *mocks.MockOverlayPort) {
				acc.GetScreenBoundsFunc = func(_ context.Context) (image.Rectangle, error) {
					return image.Rect(0, 0, 1920, 1080), nil
				}
				ov.DrawScrollHighlightFunc = func(_ context.Context, rect image.Rectangle, _ string, _ int) error {
					if rect.Dx() != 1920 || rect.Dy() != 1080 {
						t.Errorf("Unexpected rect dimensions: %v", rect)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name: "screen bounds error",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.GetScreenBoundsFunc = func(_ context.Context) (image.Rectangle, error) {
					return image.Rectangle{}, derrors.New(
						derrors.CodeAccessibilityFailed,
						"failed to get screen bounds",
					)
				}
			},
			wantErr: true,
		},
		{
			name: "overlay draw error",
			setupMocks: func(acc *mocks.MockAccessibilityPort, ov *mocks.MockOverlayPort) {
				acc.GetScreenBoundsFunc = func(_ context.Context) (image.Rectangle, error) {
					return image.Rect(0, 0, 1920, 1080), nil
				}
				ov.DrawScrollHighlightFunc = func(_ context.Context, _ image.Rectangle, _ string, _ int) error {
					return derrors.New(
						derrors.CodeOverlayFailed,
						"failed to draw scroll highlight",
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
			config := config.ScrollConfig{
				HighlightColor: "#ff0000",
				HighlightWidth: 5,
			}
			logger := logger.Get()

			if test.setupMocks != nil {
				test.setupMocks(mockAcc, mockOverlay)
			}

			service := services.NewScrollService(mockAcc, mockOverlay, config, logger)
			context := context.Background()

			showScrollOverlayErr := service.ShowScrollOverlay(context)

			if (showScrollOverlayErr != nil) != test.wantErr {
				t.Errorf(
					"ShowScrollOverlay() error = %v, wantErr %v",
					showScrollOverlayErr,
					test.wantErr,
				)
			}
		})
	}
}
