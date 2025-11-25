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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter := accessibility.NewAdapter(
				logger,
				test.excludedBundles,
				test.clickableRoles,
				mockClient,
			)

			if adapter == nil {
				t.Fatal("NewAdapter() returned nil")
			}

			if adapter.Logger == nil {
				t.Error("Adapter logger is nil")
			}
		})
	}
}

func TestAdapter_IsAppExcluded(t *testing.T) {
	logger := zap.NewNop()
	excludedBundles := []string{"com.apple.finder", "com.apple.dock"}
	mockClient := &accessibility.MockAXClient{}

	adapter := accessibility.NewAdapter(logger, excludedBundles, []string{}, mockClient)
	context := context.Background()

	tests := []struct {
		name     string
		bundleID string
		want     bool
	}{
		{
			name:     "excluded bundle",
			bundleID: "com.apple.finder",
			want:     true,
		},
		{
			name:     "not excluded bundle",
			bundleID: "com.google.Chrome",
			want:     false,
		},
		{
			name:     "empty bundle ID",
			bundleID: "",
			want:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := adapter.IsAppExcluded(context, test.bundleID)
			if got != test.want {
				t.Errorf("IsAppExcluded() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestAdapter_UpdateClickableRoles(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{"AXButton"}, mockClient)

	newRoles := []string{"AXButton", "AXLink", "AXMenuItem"}
	adapter.UpdateClickableRoles(newRoles)

	// Verify roles were updated (internal state)
	if len(adapter.ClickableRoles) != len(newRoles) {
		t.Errorf("Expected %d roles, got %d", len(newRoles), len(adapter.ClickableRoles))
	}

	// Verify mock was updated
	if len(mockClient.ClickableRoles) != len(newRoles) {
		t.Errorf(
			"Expected mock to have %d roles, got %d",
			len(newRoles),
			len(mockClient.ClickableRoles),
		)
	}
}

func TestAdapter_UpdateExcludedBundles(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(
		logger,
		[]string{"com.apple.finder"},
		[]string{},
		mockClient,
	)

	newBundles := []string{"com.apple.dock", "com.apple.systempreferences"}
	adapter.UpdateExcludedBundles(newBundles)

	context := context.Background()

	// Verify new bundles are excluded
	if !adapter.IsAppExcluded(context, "com.apple.dock") {
		t.Error("Expected com.apple.dock to be excluded")
	}

	// Verify old bundles are no longer excluded
	if adapter.IsAppExcluded(context, "com.apple.finder") {
		t.Error("Expected com.apple.finder to not be excluded after update")
	}
}

func TestAdapter_GetScreenBounds(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{
		ScreenBounds: image.Rect(0, 0, 1920, 1080),
	}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	context := context.Background()

	screenBounds, screenBoundsErr := adapter.GetScreenBounds(context)
	if screenBoundsErr != nil {
		t.Fatalf("GetScreenBounds() error = %v", screenBoundsErr)
	}

	// Verify bounds match mock
	if screenBounds != mockClient.ScreenBounds {
		t.Errorf("GetScreenBounds() = %v, want %v", screenBounds, mockClient.ScreenBounds)
	}
}

func TestAdapter_GetCursorPosition(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	context := context.Background()

	pos, posErr := adapter.GetCursorPosition(context)
	if posErr != nil {
		t.Fatalf("GetCursorPosition() error = %v", posErr)
	}

	// Cursor position should be zero as per mock default
	if pos != (image.Point{}) {
		t.Errorf("GetCursorPosition() = %v, want %v", pos, image.Point{})
	}
}

func TestAdapter_MoveCursorToPoint(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	context := context.Background()

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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			moveCursorErr := adapter.MoveCursorToPoint(context, test.point)
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
	context := context.Background()

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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scrollErr := adapter.Scroll(context, test.deltaX, test.deltaY)
			if scrollErr != nil {
				t.Errorf("Scroll() error = %v", scrollErr)
			}
		})
	}
}

func TestAdapter_Health(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{Permissions: true}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	context := context.Background()

	healthErr := adapter.Health(context)
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := adapter.MatchesFilter(elem, test.filter)
			if got != test.want {
				t.Errorf("matchesFilter() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestAdapter_PerformActionAtPoint(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	context := context.Background()

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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			performActionErr := adapter.PerformActionAtPoint(context, test.actionType, test.point)
			if (performActionErr != nil) != test.wantErr {
				t.Errorf(
					"PerformActionAtPoint() error = %v, wantErr %v",
					performActionErr,
					test.wantErr,
				)
			}
		})
	}
}

// Benchmark tests.
func BenchmarkGetScreenBounds(b *testing.B) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	context := context.Background()

	for b.Loop() {
		_, _ = adapter.GetScreenBounds(context)
	}
}

func BenchmarkGetCursorPosition(b *testing.B) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
	context := context.Background()

	for b.Loop() {
		_, _ = adapter.GetCursorPosition(context)
	}
}

func BenchmarkIsAppExcluded(b *testing.B) {
	logger := zap.NewNop()
	excludedBundles := []string{"com.apple.finder", "com.apple.dock"}
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, excludedBundles, []string{}, mockClient)
	context := context.Background()

	for b.Loop() {
		_ = adapter.IsAppExcluded(context, "com.google.Chrome")
	}
}
