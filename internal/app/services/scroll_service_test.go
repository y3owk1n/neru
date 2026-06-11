package services_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/ports/mocks"
)

func TestScrollService_Scroll(t *testing.T) {
	tests := []struct {
		name         string
		direction    services.ScrollDirection
		amount       services.ScrollAmount
		stepOverride int
		setupMocks   func(*mocks.MockAccessibilityPort)
		setupConfig  func(*config.ScrollConfig)
		wantErr      bool
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
		{
			name:         "step override down",
			direction:    services.ScrollDirectionDown,
			amount:       services.ScrollAmountChar,
			stepOverride: 99,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int) error {
					if deltaY != -99 {
						t.Errorf("Expected deltaY -99 for step override down, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:         "step override up",
			direction:    services.ScrollDirectionUp,
			amount:       services.ScrollAmountChar,
			stepOverride: 77,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int) error {
					if deltaY != 77 {
						t.Errorf("Expected deltaY 77 for step override up, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:         "step override left",
			direction:    services.ScrollDirectionLeft,
			amount:       services.ScrollAmountChar,
			stepOverride: 42,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, _ int) error {
					if deltaX != 42 {
						t.Errorf("Expected deltaX 42 for step override left, got %d", deltaX)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:         "step override right",
			direction:    services.ScrollDirectionRight,
			amount:       services.ScrollAmountChar,
			stepOverride: 33,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, _ int) error {
					if deltaX != -33 {
						t.Errorf("Expected deltaX -33 for step override right, got %d", deltaX)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:         "step override takes precedence over half page config",
			direction:    services.ScrollDirectionDown,
			amount:       services.ScrollAmountHalfPage,
			stepOverride: 5,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int) error {
					// Should use stepOverride (5), not config.ScrollStepHalf (30)
					if deltaY != -5 {
						t.Errorf("Expected deltaY -5 for step override, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:         "step override takes precedence over full page config",
			direction:    services.ScrollDirectionUp,
			amount:       services.ScrollAmountEnd,
			stepOverride: 8,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int) error {
					// Should use stepOverride (8), not config.ScrollStepFull (50)
					if deltaY != 8 {
						t.Errorf("Expected deltaY 8 for step override, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "invert scroll down becomes up",
			direction: services.ScrollDirectionDown,
			amount:    services.ScrollAmountChar,
			setupConfig: func(c *config.ScrollConfig) {
				c.InvertScroll = true
			},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int) error {
					if deltaY <= 0 {
						t.Errorf("Expected positive deltaY (inverted from down), got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "invert scroll up becomes down",
			direction: services.ScrollDirectionUp,
			amount:    services.ScrollAmountChar,
			setupConfig: func(c *config.ScrollConfig) {
				c.InvertScroll = true
			},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int) error {
					if deltaY >= 0 {
						t.Errorf("Expected negative deltaY (inverted from up), got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "invert scroll left becomes right",
			direction: services.ScrollDirectionLeft,
			amount:    services.ScrollAmountChar,
			setupConfig: func(c *config.ScrollConfig) {
				c.InvertScroll = true
			},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, _ int) error {
					if deltaX >= 0 {
						t.Errorf("Expected negative deltaX (inverted from left), got %d", deltaX)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "invert scroll right becomes left",
			direction: services.ScrollDirectionRight,
			amount:    services.ScrollAmountChar,
			setupConfig: func(c *config.ScrollConfig) {
				c.InvertScroll = true
			},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, _ int) error {
					if deltaX <= 0 {
						t.Errorf("Expected positive deltaX (inverted from right), got %d", deltaX)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "scroll down with app override",
			direction: services.ScrollDirectionDown,
			amount:    services.ScrollAmountChar,
			setupConfig: func(c *config.ScrollConfig) {
				step := 25
				half := 200
				full := 1000
				c.AppConfigs = []config.AppConfig{
					{
						BundleID:       "com.apple.Safari",
						ScrollStep:     &step,
						ScrollStepHalf: &half,
						ScrollStepFull: &full,
					},
				}
			},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.FocusedAppBundleIDFunc = func(_ context.Context) (string, error) {
					return "com.apple.Safari", nil
				}
				acc.ScrollFunc = func(_ context.Context, _, deltaY int) error {
					// Safari scroll_step is overridden to 25, so Down is -25
					if deltaY != -25 {
						t.Errorf("Expected deltaY -25 for Safari app override, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			cfg := config.ScrollConfig{
				ScrollStep:     10,
				ScrollStepHalf: 30,
				ScrollStepFull: 50,
			}
			logger := logger.Get()

			if testCase.setupConfig != nil {
				testCase.setupConfig(&cfg)
			}

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockAcc)
			}

			service := services.NewScrollService(
				mockAcc,
				mockOverlay,
				&mocks.MockSystemPort{},
				cfg,
				logger,
			)
			ctx := context.Background()

			scrollErr := service.Scroll(
				ctx,
				testCase.direction,
				testCase.amount,
				testCase.stepOverride,
			)

			if (scrollErr != nil) != testCase.wantErr {
				t.Errorf("Scroll() error = %v, wantErr %v", scrollErr, testCase.wantErr)
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
			name: successfulHide,
			setupMocks: func(ov *mocks.MockOverlayPort) {
				ov.HideFunc = func(_ context.Context) error {
					return nil
				}
			},
			wantErr: false,
		},
		{
			name: overlayHideError,
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

			service := services.NewScrollService(
				mockAcc,
				mockOverlay,
				&mocks.MockSystemPort{},
				config,
				logger,
			)
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
	service := services.NewScrollService(
		mockAcc,
		mockOverlay,
		&mocks.MockSystemPort{},
		initialConfig,
		logger,
	)

	// Update config
	newConfig := config.ScrollConfig{
		ScrollStep:     100,
		ScrollStepFull: 2000,
	}
	service.UpdateConfig(newConfig)

	// Since config is private, we can't directly check, but ensure it doesn't crash.
	// In a real scenario, we could test by calling methods that use config.
}
