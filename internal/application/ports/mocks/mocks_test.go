package mocks_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/application/ports/mocks"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
)

func TestMockAccessibilityPort_Defaults(t *testing.T) {
	mock := &mocks.MockAccessibilityPort{}

	// Test that methods return nil/zero values by default
	elements, err := mock.ClickableElements(context.TODO(), ports.DefaultElementFilter())
	if elements != nil || err != nil {
		t.Errorf(
			"ClickableElements() default should return (nil, nil), got (%v, %v)",
			elements,
			err,
		)
	}

	err = mock.PerformAction(context.TODO(), nil, 0)
	if err != nil {
		t.Errorf("PerformAction() default should return nil, got %v", err)
	}

	bundleID, err := mock.FocusedAppBundleID(context.TODO())
	if bundleID != "" || err != nil {
		t.Errorf(
			"FocusedAppBundleID() default should return (\"\", nil), got (%q, %v)",
			bundleID,
			err,
		)
	}

	excluded := mock.IsAppExcluded(context.TODO(), "test.app")
	if excluded {
		t.Error("IsAppExcluded() default should return false")
	}

	bounds, err := mock.ScreenBounds(context.TODO())
	if !bounds.Empty() || err != nil {
		t.Errorf(
			"ScreenBounds() default should return (empty rect, nil), got (%v, %v)",
			bounds,
			err,
		)
	}

	err = mock.CheckPermissions(context.TODO())
	if err != nil {
		t.Errorf("CheckPermissions() default should return nil, got %v", err)
	}

	err = mock.Health(context.TODO())
	if err != nil {
		t.Errorf("Health() default should return nil, got %v", err)
	}
}

func TestMockAccessibilityPort_WithFuncs(t *testing.T) {
	mock := &mocks.MockAccessibilityPort{
		ClickableElementsFunc: func(ctx context.Context, filter ports.ElementFilter) ([]*element.Element, error) {
			return []*element.Element{}, nil
		},
		PerformActionFunc: func(ctx context.Context, elem *element.Element, actionType action.Type) error {
			return nil
		},
		FocusedAppBundleIDFunc: func(ctx context.Context) (string, error) {
			return "test.bundle", nil
		},
		IsAppExcludedFunc: func(ctx context.Context, bundleID string) bool {
			return bundleID == "excluded.app"
		},
		ScreenBoundsFunc: func(ctx context.Context) (image.Rectangle, error) {
			return image.Rect(0, 0, 1920, 1080), nil
		},
		CheckPermissionsFunc: func(ctx context.Context) error {
			return nil
		},
		HealthFunc: func(ctx context.Context) error {
			return nil
		},
	}

	// Test that funcs are called
	elements, err := mock.ClickableElements(context.TODO(), ports.DefaultElementFilter())
	if err != nil || len(elements) != 0 {
		t.Errorf("ClickableElements() should call func, got (%v, %v)", elements, err)
	}

	err = mock.PerformAction(context.TODO(), nil, 0)
	if err != nil {
		t.Errorf("PerformAction() should call func, got %v", err)
	}

	bundleID, err := mock.FocusedAppBundleID(context.TODO())
	if err != nil || bundleID != "test.bundle" {
		t.Errorf("FocusedAppBundleID() should call func, got (%q, %v)", bundleID, err)
	}

	excluded := mock.IsAppExcluded(context.TODO(), "excluded.app")
	if !excluded {
		t.Error("IsAppExcluded() should call func and return true")
	}

	bounds, err := mock.ScreenBounds(context.TODO())
	if err != nil || bounds != image.Rect(0, 0, 1920, 1080) {
		t.Errorf("ScreenBounds() should call func, got (%v, %v)", bounds, err)
	}

	err = mock.CheckPermissions(context.TODO())
	if err != nil {
		t.Errorf("CheckPermissions() should call func, got %v", err)
	}

	err = mock.Health(context.TODO())
	if err != nil {
		t.Errorf("Health() should call func, got %v", err)
	}
}

func TestMockConfigPort_Defaults(t *testing.T) {
	mock := &mocks.MockConfigPort{}

	config := mock.Get()
	if config == nil {
		t.Error("Get() should return a default config")
	}

	err := mock.Reload(context.TODO(), "/test/path")
	if err != nil {
		t.Errorf("Reload() default should return nil, got %v", err)
	}

	ch := mock.Watch(context.TODO())
	select {
	case cfg := <-ch:
		if cfg == nil {
			t.Error("Watch() should send a config on channel")
		}
	default:
		t.Error("Watch() should immediately send a config")
	}

	err = mock.Validate(nil)
	if err != nil {
		t.Errorf("Validate() default should return nil, got %v", err)
	}

	path := mock.Path()
	if path == "" {
		t.Error("Path() should return a non-empty path")
	}
}

func TestMockOverlayPort_Defaults(t *testing.T) {
	mock := &mocks.MockOverlayPort{}

	err := mock.ShowHints(context.TODO(), nil)
	if err != nil {
		t.Errorf("ShowHints() default should return nil, got %v", err)
	}

	err = mock.ShowGrid(context.TODO(), 5, 5)
	if err != nil {
		t.Errorf("ShowGrid() default should return nil, got %v", err)
	}

	err = mock.Hide(context.TODO())
	if err != nil {
		t.Errorf("Hide() default should return nil, got %v", err)
	}

	visible := mock.IsVisible()
	if visible {
		t.Error("IsVisible() default should return false")
	}

	err = mock.Refresh(context.TODO())
	if err != nil {
		t.Errorf("Refresh() default should return nil, got %v", err)
	}

	err = mock.Health(context.TODO())
	if err != nil {
		t.Errorf("Health() default should return nil, got %v", err)
	}
}
