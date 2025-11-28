package services_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/ports/mocks"
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			logger := logger.Get()

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockAcc)
			}

			config := config.ActionConfig{
				HighlightColor: "#FF0000",
				HighlightWidth: 2,
			}

			service := services.NewActionService(mockAcc, mockOverlay, config, logger)
			ctx := context.Background()

			performActionErr := service.PerformAction(ctx, testCase.action, testCase.point)

			if (performActionErr != nil) != testCase.wantErr {
				t.Errorf(
					"PerformAction() error = %v, wantErr %v",
					performActionErr,
					testCase.wantErr,
				)
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
				acc.FocusedAppBundleIDFunc = func(_ context.Context) (string, error) {
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
				acc.FocusedAppBundleIDFunc = func(_ context.Context) (string, error) {
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			logger := logger.Get()

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockAcc)
			}

			config := config.ActionConfig{}
			service := services.NewActionService(mockAcc, mockOverlay, config, logger)
			ctx := context.Background()

			isExcluded, isExcludedErr := service.IsFocusedAppExcluded(ctx)

			if (isExcludedErr != nil) != testCase.wantErr {
				t.Errorf(
					"IsFocusedAppExcluded() error = %v, wantErr %v",
					isExcludedErr,
					testCase.wantErr,
				)
			}

			if isExcluded != testCase.want {
				t.Errorf("IsFocusedAppExcluded() = %v, want %v", isExcluded, testCase.want)
			}
		})
	}
}

func TestActionService_FocusedAppBundleID(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(*mocks.MockAccessibilityPort)
		want       string
		wantErr    bool
	}{
		{
			name: "success",
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.FocusedAppBundleIDFunc = func(_ context.Context) (string, error) {
					return "com.example.app", nil
				}
			},
			want:    "com.example.app",
			wantErr: false,
		},
		{
			name: "error getting bundle ID",
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.FocusedAppBundleIDFunc = func(_ context.Context) (string, error) {
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
				acc.FocusedAppBundleIDFunc = func(_ context.Context) (string, error) {
					return "", nil
				}
			},
			want:    "",
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			logger := logger.Get()

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockAcc)
			}

			config := config.ActionConfig{}
			service := services.NewActionService(mockAcc, mockOverlay, config, logger)
			ctx := context.Background()

			focusedApp, focusedAppErr := service.FocusedAppBundleID(ctx)

			if (focusedAppErr != nil) != testCase.wantErr {
				t.Errorf(
					"FocusedAppBundleID() error = %v, wantErr %v",
					focusedAppErr,
					testCase.wantErr,
				)
			}

			if focusedApp != testCase.want {
				t.Errorf("FocusedAppBundleID() = %v, want %v", focusedApp, testCase.want)
			}
		})
	}
}

func TestActionService_ExecuteAction(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	logger := logger.Get()
	config := config.ActionConfig{}
	service := services.NewActionService(mockAcc, mockOverlay, config, logger)

	elem, _ := element.NewElement("test", image.Rect(10, 10, 50, 50), element.RoleButton)
	ctx := context.Background()

	// Test successful execution
	mockAcc.PerformActionFunc = func(_ context.Context, _ *element.Element, actionType action.Type) error {
		if actionType != action.TypeLeftClick {
			t.Errorf("Expected TypeLeftClick, got %v", actionType)
		}

		return nil
	}

	err := service.ExecuteAction(ctx, elem, action.TypeLeftClick)
	if err != nil {
		t.Errorf("ExecuteAction() returned error: %v", err)
	}

	// Test execution with error
	mockAcc.PerformActionFunc = func(_ context.Context, _ *element.Element, _ action.Type) error {
		return derrors.New(derrors.CodeActionFailed, "action failed")
	}

	err = service.ExecuteAction(ctx, elem, action.TypeLeftClick)
	if err == nil {
		t.Error("ExecuteAction() should return error when accessibility fails")
	}
}

func TestActionService_ShowActionHighlight(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	logger := logger.Get()
	config := config.ActionConfig{}
	service := services.NewActionService(mockAcc, mockOverlay, config, logger)

	ctx := context.Background()

	// Test successful highlight
	screenBounds := image.Rect(0, 0, 1920, 1080)
	mockAcc.ScreenBoundsFunc = func(_ context.Context) (image.Rectangle, error) {
		return screenBounds, nil
	}

	mockOverlay.DrawActionHighlightFunc = func(_ context.Context, rect image.Rectangle, _ string, _ int) error {
		if rect != screenBounds {
			t.Errorf(
				"DrawActionHighlight called with wrong rect: %v, expected %v",
				rect,
				screenBounds,
			)
		}

		return nil
	}

	err := service.ShowActionHighlight(ctx)
	if err != nil {
		t.Errorf("ShowActionHighlight() returned error: %v", err)
	}

	// Test with screen bounds error
	mockAcc.ScreenBoundsFunc = func(_ context.Context) (image.Rectangle, error) {
		return image.Rectangle{}, derrors.New(
			derrors.CodeAccessibilityFailed,
			"screen bounds failed",
		)
	}

	err = service.ShowActionHighlight(ctx)
	if err == nil {
		t.Error("ShowActionHighlight() should return error when screen bounds fails")
	}
}

func TestActionService_MoveCursorToElement(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	logger := logger.Get()
	config := config.ActionConfig{}
	service := services.NewActionService(mockAcc, mockOverlay, config, logger)

	elem, _ := element.NewElement("test", image.Rect(10, 20, 50, 60), element.RoleButton)
	ctx := context.Background()

	// Test successful move
	mockAcc.MoveCursorToPointFunc = func(_ context.Context, point image.Point) error {
		expected := image.Point{X: 30, Y: 40} // center of element
		if point != expected {
			t.Errorf("MoveCursorToPoint called with %v, expected %v", point, expected)
		}

		return nil
	}

	err := service.MoveCursorToElement(ctx, elem)
	if err != nil {
		t.Errorf("MoveCursorToElement() returned error: %v", err)
	}

	// Test with error
	mockAcc.MoveCursorToPointFunc = func(_ context.Context, _ image.Point) error {
		return derrors.New(derrors.CodeAccessibilityFailed, "move failed")
	}

	err = service.MoveCursorToElement(ctx, elem)
	if err == nil {
		t.Error("MoveCursorToElement() should return error when move fails")
	}
}

func TestActionService_MoveCursorToPoint(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	logger := logger.Get()
	config := config.ActionConfig{}
	service := services.NewActionService(mockAcc, mockOverlay, config, logger)

	point := image.Point{X: 100, Y: 200}
	ctx := context.Background()

	// Test successful move
	mockAcc.MoveCursorToPointFunc = func(_ context.Context, p image.Point) error {
		if p != point {
			t.Errorf("MoveCursorToPoint called with %v, expected %v", p, point)
		}

		return nil
	}

	err := service.MoveCursorToPoint(ctx, point)
	if err != nil {
		t.Errorf("MoveCursorToPoint() returned error: %v", err)
	}

	// Test with error
	mockAcc.MoveCursorToPointFunc = func(_ context.Context, _ image.Point) error {
		return derrors.New(derrors.CodeAccessibilityFailed, "move failed")
	}

	err = service.MoveCursorToPoint(ctx, point)
	if err == nil {
		t.Error("MoveCursorToPoint() should return error when move fails")
	}
}

func TestActionService_CursorPosition(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	logger := logger.Get()
	config := config.ActionConfig{}
	service := services.NewActionService(mockAcc, mockOverlay, config, logger)

	expectedPoint := image.Point{X: 150, Y: 250}
	ctx := context.Background()

	// Test successful get
	mockAcc.CursorPositionFunc = func(_ context.Context) (image.Point, error) {
		return expectedPoint, nil
	}

	point, err := service.CursorPosition(ctx)
	if err != nil {
		t.Errorf("CursorPosition() returned error: %v", err)
	}

	if point != expectedPoint {
		t.Errorf("CursorPosition() = %v, expected %v", point, expectedPoint)
	}

	// Test with error
	mockAcc.CursorPositionFunc = func(_ context.Context) (image.Point, error) {
		return image.Point{}, derrors.New(derrors.CodeAccessibilityFailed, "position failed")
	}

	_, err = service.CursorPosition(ctx)
	if err == nil {
		t.Error("CursorPosition() should return error when accessibility fails")
	}
}
