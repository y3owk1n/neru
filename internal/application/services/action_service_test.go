package services_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/application/ports/mocks"
	"github.com/y3owk1n/neru/internal/application/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/infra/logger"
)

func TestActionService_PerformAction(t *testing.T) {
	tests := []struct {
		name       string
		action     string
		point      image.Point
		setupMocks func(*mocks.MockAccessibilityPort)
		wantErr    bool
	}{
		{
			name:   "perform left click",
			action: "left_click",
			point:  image.Point{X: 100, Y: 100},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.PerformActionAtPointFunc = func(_ context.Context, actionType action.Type, _ image.Point) error {
					if actionType != action.TypeLeftClick {
						t.Errorf("Expected action TypeLeftClick, got '%v'", actionType)
					}
					return nil
				}
			},
			wantErr: false,
		},
		{
			name:   "perform right click",
			action: "right_click",
			point:  image.Point{X: 100, Y: 100},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.PerformActionAtPointFunc = func(_ context.Context, actionType action.Type, _ image.Point) error {
					if actionType != action.TypeRightClick {
						t.Errorf("Expected action TypeRightClick, got '%v'", actionType)
					}
					return nil
				}
			},
			wantErr: false,
		},
		{
			name:       "unknown action",
			action:     "unknown_action",
			point:      image.Point{X: 100, Y: 100},
			setupMocks: nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			log := logger.Get()

			if tt.setupMocks != nil {
				tt.setupMocks(mockAcc)
			}

			cfg := config.ActionConfig{
				HighlightColor: "#FF0000",
				HighlightWidth: 2,
			}

			service := services.NewActionService(mockAcc, mockOverlay, cfg, log)
			ctx := context.Background()

			err := service.PerformAction(ctx, tt.action, tt.point)

			if (err != nil) != tt.wantErr {
				t.Errorf("PerformAction() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestActionService_IsFocusedAppExcluded(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(*mocks.MockAccessibilityPort)
		want       bool
		wantErr    bool
	}{
		{
			name: "app is excluded",
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.GetFocusedAppBundleIDFunc = func(_ context.Context) (string, error) {
					return "com.excluded.app", nil
				}
				acc.IsAppExcludedFunc = func(_ context.Context, bundleID string) bool {
					return bundleID == "com.excluded.app"
				}
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "app is not excluded",
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.GetFocusedAppBundleIDFunc = func(_ context.Context) (string, error) {
					return "com.normal.app", nil
				}
				acc.IsAppExcludedFunc = func(_ context.Context, _ string) bool {
					return false
				}
			},
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			log := logger.Get()

			if tt.setupMocks != nil {
				tt.setupMocks(mockAcc)
			}

			cfg := config.ActionConfig{}
			service := services.NewActionService(mockAcc, mockOverlay, cfg, log)
			ctx := context.Background()

			got, err := service.IsFocusedAppExcluded(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("IsFocusedAppExcluded() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsFocusedAppExcluded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestActionService_GetFocusedAppBundleID(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(*mocks.MockAccessibilityPort)
		want       string
		wantErr    bool
	}{
		{
			name: "success",
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.GetFocusedAppBundleIDFunc = func(_ context.Context) (string, error) {
					return "com.example.app", nil
				}
			},
			want:    "com.example.app",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			log := logger.Get()

			if tt.setupMocks != nil {
				tt.setupMocks(mockAcc)
			}

			cfg := config.ActionConfig{}
			service := services.NewActionService(mockAcc, mockOverlay, cfg, log)
			ctx := context.Background()

			got, err := service.GetFocusedAppBundleID(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetFocusedAppBundleID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("GetFocusedAppBundleID() = %v, want %v", got, tt.want)
			}
		})
	}
}
