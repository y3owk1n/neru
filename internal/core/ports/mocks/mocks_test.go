//go:build !integration

package mocks_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/core/ports"
	"github.com/y3owk1n/neru/internal/core/ports/mocks"
)

func TestMockAccessibilityPort_Defaults(t *testing.T) {
	mock := &mocks.MockAccessibilityPort{}

	// Test that methods return nil/zero values by default
	elements, err := mock.ClickableElements(context.Background(), ports.DefaultElementFilter())
	if elements != nil || err != nil {
		t.Errorf(
			"ClickableElements() default should return (nil, nil), got (%v, %v)",
			elements,
			err,
		)
	}

	err = mock.PerformAction(context.Background(), nil, 0)
	if err != nil {
		t.Errorf("PerformAction() default should return nil, got %v", err)
	}

	bundleID, err := mock.FocusedAppBundleID(context.Background())
	if bundleID != "" || err != nil {
		t.Errorf(
			"FocusedAppBundleID() default should return (\"\", nil), got (%q, %v)",
			bundleID,
			err,
		)
	}

	excluded := mock.IsAppExcluded(context.Background(), "test.app")
	if excluded {
		t.Error("IsAppExcluded() default should return false")
	}

	bounds, err := mock.ScreenBounds(context.Background())
	if !bounds.Empty() || err != nil {
		t.Errorf(
			"ScreenBounds() default should return (empty rect, nil), got (%v, %v)",
			bounds,
			err,
		)
	}

	err = mock.CheckPermissions(context.Background())
	if err != nil {
		t.Errorf("CheckPermissions() default should return nil, got %v", err)
	}

	err = mock.Health(context.Background())
	if err != nil {
		t.Errorf("Health() default should return nil, got %v", err)
	}
}

func TestMockConfigPort_Defaults(t *testing.T) {
	mock := &mocks.MockConfigPort{}

	config := mock.Get()
	if config == nil {
		t.Error("Get() should return a default config")
	}

	err := mock.Reload(context.Background(), "/test/path")
	if err != nil {
		t.Errorf("Reload() default should return nil, got %v", err)
	}

	ch := mock.Watch(context.Background())
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

	err := mock.ShowHints(context.Background(), nil)
	if err != nil {
		t.Errorf("ShowHints() default should return nil, got %v", err)
	}

	err = mock.ShowGrid(context.Background())
	if err != nil {
		t.Errorf("ShowGrid() default should return nil, got %v", err)
	}

	err = mock.Hide(context.Background())
	if err != nil {
		t.Errorf("Hide() default should return nil, got %v", err)
	}

	visible := mock.IsVisible()
	if visible {
		t.Error("IsVisible() default should return false")
	}

	err = mock.Refresh(context.Background())
	if err != nil {
		t.Errorf("Refresh() default should return nil, got %v", err)
	}

	err = mock.Health(context.Background())
	if err != nil {
		t.Errorf("Health() default should return nil, got %v", err)
	}
}
