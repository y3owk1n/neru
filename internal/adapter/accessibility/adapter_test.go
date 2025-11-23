package accessibility

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	"go.uber.org/zap"
)

func TestNewAdapter(t *testing.T) {
	logger := zap.NewNop()

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewAdapter(logger, tt.excludedBundles, tt.clickableRoles)

			if adapter == nil {
				t.Fatal("NewAdapter() returned nil")
			}

			if adapter.logger == nil {
				t.Error("Adapter logger is nil")
			}
		})
	}
}

func TestAdapter_IsAppExcluded(t *testing.T) {
	logger := zap.NewNop()
	excludedBundles := []string{"com.apple.finder", "com.apple.dock"}

	adapter := NewAdapter(logger, excludedBundles, []string{})
	ctx := context.Background()

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.IsAppExcluded(ctx, tt.bundleID)
			if got != tt.want {
				t.Errorf("IsAppExcluded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdapter_UpdateClickableRoles(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, []string{}, []string{"AXButton"})

	newRoles := []string{"AXButton", "AXLink", "AXMenuItem"}
	adapter.UpdateClickableRoles(newRoles)

	// Verify roles were updated (internal state)
	if len(adapter.clickableRoles) != len(newRoles) {
		t.Errorf("Expected %d roles, got %d", len(newRoles), len(adapter.clickableRoles))
	}
}

func TestAdapter_UpdateExcludedBundles(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, []string{"com.apple.finder"}, []string{})

	newBundles := []string{"com.apple.dock", "com.apple.systempreferences"}
	adapter.UpdateExcludedBundles(newBundles)

	ctx := context.Background()

	// Verify new bundles are excluded
	if !adapter.IsAppExcluded(ctx, "com.apple.dock") {
		t.Error("Expected com.apple.dock to be excluded")
	}

	// Verify old bundles are no longer excluded
	if adapter.IsAppExcluded(ctx, "com.apple.finder") {
		t.Error("Expected com.apple.finder to not be excluded after update")
	}
}

func TestAdapter_GetScreenBounds(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, []string{}, []string{})
	ctx := context.Background()

	bounds, err := adapter.GetScreenBounds(ctx)
	if err != nil {
		t.Fatalf("GetScreenBounds() error = %v", err)
	}

	// Verify bounds are valid
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Errorf("GetScreenBounds() returned invalid bounds: %v", bounds)
	}
}

func TestAdapter_GetCursorPosition(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, []string{}, []string{})
	ctx := context.Background()

	pos, err := adapter.GetCursorPosition(ctx)
	if err != nil {
		t.Fatalf("GetCursorPosition() error = %v", err)
	}

	// Cursor position should be within reasonable bounds
	// (not checking exact position as it depends on system state)
	_ = pos
}

func TestAdapter_MoveCursorToPoint(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, []string{}, []string{})
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.MoveCursorToPoint(ctx, tt.point)
			if err != nil {
				t.Errorf("MoveCursorToPoint() error = %v", err)
			}
		})
	}
}

func TestAdapter_Scroll(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, []string{}, []string{})
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.Scroll(ctx, tt.deltaX, tt.deltaY)
			if err != nil {
				t.Errorf("Scroll() error = %v", err)
			}
		})
	}
}

func TestAdapter_Health(_ *testing.T) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, []string{}, []string{})
	ctx := context.Background()

	err := adapter.Health(ctx)
	// Health check may fail if permissions are not granted
	// We just verify it doesn't panic
	_ = err
}

func TestAdapter_MatchesFilter(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, []string{}, []string{})

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.matchesFilter(elem, tt.filter)
			if got != tt.want {
				t.Errorf("matchesFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdapter_PerformActionAtPoint(t *testing.T) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, []string{}, []string{})
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.PerformActionAtPoint(ctx, tt.actionType, tt.point)
			if (err != nil) != tt.wantErr {
				t.Errorf("PerformActionAtPoint() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Benchmark tests.
func BenchmarkGetScreenBounds(b *testing.B) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, []string{}, []string{})
	ctx := context.Background()

	for b.Loop() {
		_, _ = adapter.GetScreenBounds(ctx)
	}
}

func BenchmarkGetCursorPosition(b *testing.B) {
	logger := zap.NewNop()
	adapter := NewAdapter(logger, []string{}, []string{})
	ctx := context.Background()

	for b.Loop() {
		_, _ = adapter.GetCursorPosition(ctx)
	}
}

func BenchmarkIsAppExcluded(b *testing.B) {
	logger := zap.NewNop()
	excludedBundles := []string{"com.apple.finder", "com.apple.dock"}
	adapter := NewAdapter(logger, excludedBundles, []string{})
	ctx := context.Background()

	for b.Loop() {
		_ = adapter.IsAppExcluded(ctx, "com.google.Chrome")
	}
}
