package accessibility_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/accessibility"
	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	"go.uber.org/zap"
)

func TestNewAdapter(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}

	tests := []struct {
		name            string
		excludedBundles []string
		clickableRoles  []string
	}{
		{
			name:            "with excluded bundles",
			excludedBundles: []string{"com.apple.finder", "com.apple.dock"},
			clickableRoles:  []string{"AXButton", "AXLink"},
		},
		{
			name:            "empty configuration",
			excludedBundles: []string{},
			clickableRoles:  []string{},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			adapter := accessibility.NewAdapter(
				logger,
				testCase.excludedBundles,
				testCase.clickableRoles,
				mockClient,
			)

			if adapter == nil {
				t.Fatal("NewAdapter() returned nil")
			}

			if adapter.Logger() == nil {
				t.Error("Adapter logger is nil")
			}

			// Test ClickableRoles getter
			if len(adapter.ClickableRoles()) != len(testCase.clickableRoles) {
				t.Errorf(
					"ClickableRoles() length = %d, want %d",
					len(adapter.ClickableRoles()),
					len(testCase.clickableRoles),
				)
			}
		})
	}
}

func TestAdapter_UpdateClickableRoles(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}

	adapter := accessibility.NewAdapter(logger, []string{}, []string{"AXButton"}, mockClient)

	// Initial roles
	initialRoles := adapter.ClickableRoles()
	if len(initialRoles) != 1 || initialRoles[0] != "AXButton" {
		t.Errorf("Initial ClickableRoles() = %v, want [AXButton]", initialRoles)
	}

	// Update roles
	newRoles := []string{"AXLink", "AXTextField"}
	adapter.UpdateClickableRoles(newRoles)

	// Check updated roles
	updatedRoles := adapter.ClickableRoles()
	if len(updatedRoles) != 2 || updatedRoles[0] != "AXLink" || updatedRoles[1] != "AXTextField" {
		t.Errorf("Updated ClickableRoles() = %v, want [AXLink, AXTextField]", updatedRoles)
	}
}

func TestAdapter_UpdateExcludedBundles(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}

	adapter := accessibility.NewAdapter(logger, []string{"com.app1"}, []string{}, mockClient)

	// Initial exclusion
	if !adapter.IsAppExcluded(context.Background(), "com.app1") {
		t.Error("com.app1 should be excluded initially")
	}

	if adapter.IsAppExcluded(context.Background(), "com.app2") {
		t.Error("com.app2 should not be excluded initially")
	}

	// Update excluded bundles
	newBundles := []string{"com.app2", "com.app3"}
	adapter.UpdateExcludedBundles(newBundles)

	// Check updated exclusions
	if adapter.IsAppExcluded(context.Background(), "com.app1") {
		t.Error("com.app1 should not be excluded after update")
	}

	if !adapter.IsAppExcluded(context.Background(), "com.app2") {
		t.Error("com.app2 should be excluded after update")
	}

	if !adapter.IsAppExcluded(context.Background(), "com.app3") {
		t.Error("com.app3 should be excluded after update")
	}
}

func TestAdapter_ScreenBounds(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{
		MockScreenBounds: image.Rect(0, 0, 1920, 1080),
	}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	ctx := context.Background()

	screenBounds, screenBoundsErr := adapter.ScreenBounds(ctx)
	if screenBoundsErr != nil {
		t.Fatalf("ScreenBounds() error = %v", screenBoundsErr)
	}

	// Verify bounds match mock
	if screenBounds != mockClient.MockScreenBounds {
		t.Errorf("ScreenBounds() = %v, want %v", screenBounds, mockClient.MockScreenBounds)
	}
}

func TestAdapter_CursorPosition(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	ctx := context.Background()

	pos, posErr := adapter.CursorPosition(ctx)
	if posErr != nil {
		t.Fatalf("CursorPosition() error = %v", posErr)
	}

	// Cursor position should be zero as per mock default
	if pos != (image.Point{}) {
		t.Errorf("CursorPosition() = %v, want %v", pos, image.Point{})
	}
}

func TestAdapter_MoveCursorToPoint(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	ctx := context.Background()

	tests := []struct {
		name  string
		point image.Point
	}{
		{
			name:  "move to center",
			point: image.Point{X: 500, Y: 500},
		},
		{
			name:  "move to origin",
			point: image.Point{X: 0, Y: 0},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			moveCursorErr := adapter.MoveCursorToPoint(ctx, testCase.point)
			if moveCursorErr != nil {
				t.Errorf("MoveCursorToPoint() error = %v", moveCursorErr)
			}
		})
	}
}

func TestAdapter_Scroll(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	ctx := context.Background()

	tests := []struct {
		name   string
		deltaX int
		deltaY int
	}{
		{
			name:   "scroll down",
			deltaX: 0,
			deltaY: -10,
		},
		{
			name:   "scroll up",
			deltaX: 0,
			deltaY: 10,
		},
		{
			name:   "scroll right",
			deltaX: 10,
			deltaY: 0,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			scrollErr := adapter.Scroll(ctx, testCase.deltaX, testCase.deltaY)
			if scrollErr != nil {
				t.Errorf("Scroll() error = %v", scrollErr)
			}
		})
	}
}

func TestAdapter_Health(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{MockPermissions: true}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	ctx := context.Background()

	healthErr := adapter.Health(ctx)
	if healthErr != nil {
		t.Errorf("Health() error = %v", healthErr)
	}
}

func TestAdapter_MatchesFilter(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)

	// Create test element
	elem, _ := element.NewElement(
		element.ID("test-id"),
		image.Rect(0, 0, 100, 100),
		element.RoleButton,
	)

	tests := []struct {
		name   string
		filter ports.ElementFilter
		want   bool
	}{
		{
			name: "match by role",
			filter: ports.ElementFilter{
				Roles: []element.Role{element.RoleButton},
			},
			want: true,
		},
		{
			name: "no match by role",
			filter: ports.ElementFilter{
				Roles: []element.Role{element.RoleLink},
			},
			want: false,
		},
		{
			name: "match by min size",
			filter: ports.ElementFilter{
				MinSize: image.Point{X: 50, Y: 50},
			},
			want: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := adapter.MatchesFilter(elem, testCase.filter)
			if got != testCase.want {
				t.Errorf("matchesFilter() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestAdapter_PerformActionAtPoint(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	ctx := context.Background()

	tests := []struct {
		name       string
		actionType action.Type
		point      image.Point
		wantErr    bool
	}{
		{
			name:       "click at point",
			actionType: action.TypeLeftClick,
			point:      image.Point{X: 100, Y: 100},
			wantErr:    false,
		},
		{
			name:       "right click at point",
			actionType: action.TypeRightClick,
			point:      image.Point{X: 200, Y: 200},
			wantErr:    false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			performActionErr := adapter.PerformActionAtPoint(
				ctx,
				testCase.actionType,
				testCase.point,
			)
			if (performActionErr != nil) != testCase.wantErr {
				t.Errorf(
					"PerformActionAtPoint() error = %v, wantErr %v",
					performActionErr,
					testCase.wantErr,
				)
			}
		})
	}
}
