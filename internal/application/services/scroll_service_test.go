package services_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/application/ports/mocks"
	"github.com/y3owk1n/neru/internal/application/services"
	"github.com/y3owk1n/neru/internal/config"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			cfg := config.ScrollConfig{
				ScrollStep:     10,
				ScrollStepFull: 50,
			}
			log := logger.Get()

			if tt.setupMocks != nil {
				tt.setupMocks(mockAcc)
			}

			service := services.NewScrollService(mockAcc, mockOverlay, cfg, log)
			ctx := context.Background()

			err := service.Scroll(ctx, tt.direction, tt.amount)

			if (err != nil) != tt.wantErr {
				t.Errorf("Scroll() error = %v, wantErr %v", err, tt.wantErr)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			cfg := config.ScrollConfig{
				HighlightColor: "#ff0000",
				HighlightWidth: 5,
			}
			log := logger.Get()

			if tt.setupMocks != nil {
				tt.setupMocks(mockAcc, mockOverlay)
			}

			service := services.NewScrollService(mockAcc, mockOverlay, cfg, log)
			ctx := context.Background()

			err := service.ShowScrollOverlay(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("ShowScrollOverlay() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
