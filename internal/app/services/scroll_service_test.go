package services_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/ports/mocks"
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			config := config.ScrollConfig{
				ScrollStep:     10,
				ScrollStepHalf: 30,
				ScrollStepFull: 50,
			}
			logger := logger.Get()

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockAcc)
			}

			service := services.NewScrollService(mockAcc, mockOverlay, config, logger)
			ctx := context.Background()

			scrollErr := service.Scroll(ctx, testCase.direction, testCase.amount)

			if (scrollErr != nil) != testCase.wantErr {
				t.Errorf("Scroll() error = %v, wantErr %v", scrollErr, testCase.wantErr)
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
				acc.CursorPositionFunc = func(_ context.Context) (image.Point, error) {
					return image.Point{X: 100, Y: 100}, nil
				}
				ov.DrawScrollIndicatorFunc = func(x, y int) {
					if x != 100 || y != 100 {
						t.Errorf("Unexpected scroll indicator position: (%d, %d)", x, y)
					}
				}
			},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			config := config.ScrollConfig{
				ScrollStep: 10,
			}
			logger := logger.Get()

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockAcc, mockOverlay)
			}

			service := services.NewScrollService(mockAcc, mockOverlay, config, logger)
			ctx := context.Background()

			showScrollOverlayErr := service.Show(ctx)

			if (showScrollOverlayErr != nil) != testCase.wantErr {
				t.Errorf(
					"Show() error = %v, wantErr %v",
					showScrollOverlayErr,
					testCase.wantErr,
				)
			}
		})
	}
}

func TestScrollService_Hide(t *testing.T) {
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
			config := config.ScrollConfig{}
			logger := logger.Get()

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockOverlay)
			}

			service := services.NewScrollService(mockAcc, mockOverlay, config, logger)
			ctx := context.Background()

			hideScrollOverlayErr := service.Hide(ctx)

			if (hideScrollOverlayErr != nil) != testCase.wantErr {
				t.Errorf(
					"Hide() error = %v, wantErr %v",
					hideScrollOverlayErr,
					testCase.wantErr,
				)
			}
		})
	}
}

func TestScrollService_UpdateConfig(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	logger := logger.Get()

	initialConfig := config.ScrollConfig{
		ScrollStep:     50,
		ScrollStepFull: 1000,
	}
	service := services.NewScrollService(mockAcc, mockOverlay, initialConfig, logger)

	// Update config
	newConfig := config.ScrollConfig{
		ScrollStep:     100,
		ScrollStepFull: 2000,
	}
	ctx := context.Background()
	service.UpdateConfig(ctx, newConfig)

	// Since config is private, we can't directly check, but ensure it doesn't crash.
	// In a real scenario, we could test by calling methods that use config.
}
