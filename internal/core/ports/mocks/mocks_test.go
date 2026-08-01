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

	err = mock.Health(context.Background())
	if err != nil {
		t.Errorf("Health() default should return nil, got %v", err)
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
