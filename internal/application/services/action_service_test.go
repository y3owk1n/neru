package services_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/application/ports/mocks"
	"github.com/y3owk1n/neru/internal/application/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/action"
	derrors "github.com/y3owk1n/neru/internal/errors"
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
			name:   "perform middle click",
			action: "middle_click",
			point:  image.Point{X: 100, Y: 100},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.PerformActionAtPointFunc = func(_ context.Context, actionType action.Type, _ image.Point) error {
					if actionType != action.TypeMiddleClick {
						t.Errorf("Expected action TypeMiddleClick, got '%v'", actionType)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:   "perform mouse down",
			action: "mouse_down",
			point:  image.Point{X: 100, Y: 100},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.PerformActionAtPointFunc = func(_ context.Context, actionType action.Type, _ image.Point) error {
					if actionType != action.TypeMouseDown {
						t.Errorf("Expected action TypeMouseDown, got '%v'", actionType)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:   "perform mouse up",
			action: "mouse_up",
			point:  image.Point{X: 100, Y: 100},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.PerformActionAtPointFunc = func(_ context.Context, actionType action.Type, _ image.Point) error {
					if actionType != action.TypeMouseUp {
						t.Errorf("Expected action TypeMouseUp, got '%v'", actionType)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:   "perform scroll",
			action: "scroll",
			point:  image.Point{X: 100, Y: 100},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.PerformActionAtPointFunc = func(_ context.Context, actionType action.Type, _ image.Point) error {
					if actionType != action.TypeScroll {
						t.Errorf("Expected action TypeScroll, got '%v'", actionType)
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
		{
			name:   "accessibility error",
			action: "left_click",
			point:  image.Point{X: 100, Y: 100},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.PerformActionAtPointFunc = func(_ context.Context, _ action.Type, _ image.Point) error {
					return derrors.New(
						derrors.CodeAccessibilityFailed,
						"accessibility permission denied",
					)
				}
			},
			wantErr: true,
		},
		{
			name:   "negative coordinates",
			action: "left_click",
			point:  image.Point{X: -100, Y: -100},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.PerformActionAtPointFunc = func(_ context.Context, _ action.Type, pt image.Point) error {
					// Service should still call the port, validation happens elsewhere
					if pt.X != -100 || pt.Y != -100 {
						t.Errorf("Expected point (-100, -100), got (%d, %d)", pt.X, pt.Y)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:   "extreme coordinates",
			action: "left_click",
			point:  image.Point{X: 99999, Y: 99999},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.PerformActionAtPointFunc = func(_ context.Context, _ action.Type, pt image.Point) error {
					if pt.X != 99999 || pt.Y != 99999 {
						t.Errorf("Expected point (99999, 99999), got (%d, %d)", pt.X, pt.Y)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:   "zero coordinates",
			action: "left_click",
			point:  image.Point{X: 0, Y: 0},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.PerformActionAtPointFunc = func(_ context.Context, _ action.Type, pt image.Point) error {
					if pt.X != 0 || pt.Y != 0 {
						t.Errorf("Expected point (0, 0), got (%d, %d)", pt.X, pt.Y)
					}

					return nil
				}
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			logger := logger.Get()

			if test.setupMocks != nil {
				test.setupMocks(mockAcc)
			}

			config := config.ActionConfig{
				HighlightColor: "#FF0000",
				HighlightWidth: 2,
			}

			service := services.NewActionService(mockAcc, mockOverlay, config, logger)
			context := context.Background()

			performActionErr := service.PerformAction(context, test.action, test.point)

			if (performActionErr != nil) != test.wantErr {
				t.Errorf("PerformAction() error = %v, wantErr %v", performActionErr, test.wantErr)
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			logger := logger.Get()

			if test.setupMocks != nil {
				test.setupMocks(mockAcc)
			}

			config := config.ActionConfig{}
			service := services.NewActionService(mockAcc, mockOverlay, config, logger)
			context := context.Background()

			isExcluded, isExcludedErr := service.IsFocusedAppExcluded(context)

			if (isExcludedErr != nil) != test.wantErr {
				t.Errorf(
					"IsFocusedAppExcluded() error = %v, wantErr %v",
					isExcludedErr,
					test.wantErr,
				)
			}

			if isExcluded != test.want {
				t.Errorf("IsFocusedAppExcluded() = %v, want %v", isExcluded, test.want)
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
		{
			name: "error getting bundle ID",
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.GetFocusedAppBundleIDFunc = func(_ context.Context) (string, error) {
					return "", derrors.New(
						derrors.CodeAccessibilityFailed,
						"failed to get focused app",
					)
				}
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "empty bundle ID",
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.GetFocusedAppBundleIDFunc = func(_ context.Context) (string, error) {
					return "", nil
				}
			},
			want:    "",
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			logger := logger.Get()

			if test.setupMocks != nil {
				test.setupMocks(mockAcc)
			}

			config := config.ActionConfig{}
			service := services.NewActionService(mockAcc, mockOverlay, config, logger)
			context := context.Background()

			focusedApp, focusedAppErr := service.GetFocusedAppBundleID(context)

			if (focusedAppErr != nil) != test.wantErr {
				t.Errorf(
					"GetFocusedAppBundleID() error = %v, wantErr %v",
					focusedAppErr,
					test.wantErr,
				)
			}

			if focusedApp != test.want {
				t.Errorf("GetFocusedAppBundleID() = %v, want %v", focusedApp, test.want)
			}
		})
	}
}
