package accessibility_test

import (
	"context"
	"image"
	"reflect"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// errTestAccessibility is a static error for testing accessibility failures.
var errTestAccessibility = derrors.New(derrors.CodeAccessibilityDenied, "accessibility denied")

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
		})
	}
}

func TestAdapter_IsAppExcluded(t *testing.T) {
	logger := zap.NewNop()
	excludedBundles := []string{"com.apple.finder", "com.apple.dock"}
	mockClient := &accessibility.MockAXClient{}

	adapter := accessibility.NewAdapter(logger, excludedBundles, []string{}, mockClient)
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

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := adapter.IsAppExcluded(ctx, testCase.bundleID)
			if got != testCase.want {
				t.Errorf("IsAppExcluded() = %v, want %v", got, testCase.want)
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
	if len(adapter.ClickableRoles()) != len(newRoles) {
		t.Errorf("Expected %d roles, got %d", len(newRoles), len(adapter.ClickableRoles()))
	}

	// Verify mock was updated
	if len(mockClient.MockClickableRoles) != len(newRoles) {
		t.Errorf(
			"Expected mock to have %d roles, got %d",
			len(newRoles),
			len(mockClient.MockClickableRoles),
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

// mockAXApp is a mock implementation of AXApp for testing.
type mockAXApp struct {
	bundleID string
}

func (m *mockAXApp) Release()                 {}
func (m *mockAXApp) BundleIdentifier() string { return m.bundleID }

func (m *mockAXApp) Info() (*accessibility.AXAppInfo, error) { return &accessibility.AXAppInfo{}, nil }

func TestAdapter_FocusedAppBundleID(t *testing.T) {
	tests := []struct {
		name       string
		mockApp    *mockAXApp
		mockErr    error
		wantBundle string
		wantErr    bool
	}{
		{
			name:       "success",
			mockApp:    &mockAXApp{bundleID: "com.google.Chrome"},
			mockErr:    nil,
			wantBundle: "com.google.Chrome",
			wantErr:    false,
		},
		{
			name:       "error from client",
			mockApp:    nil,
			mockErr:    errTestAccessibility,
			wantBundle: "",
			wantErr:    true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			logger := zap.NewNop()
			mockClient := &accessibility.MockAXClient{
				MockFocusedApp:    testCase.mockApp,
				MockFocusedAppErr: testCase.mockErr,
			}
			adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
			ctx := context.Background()

			bundleID, bundleIDErr := adapter.FocusedAppBundleID(ctx)

			if (bundleIDErr != nil) != testCase.wantErr {
				t.Errorf(
					"FocusedAppBundleID() error = %v, wantErr %v",
					bundleIDErr,
					testCase.wantErr,
				)

				return
			}

			if bundleID != testCase.wantBundle {
				t.Errorf("FocusedAppBundleID() = %v, want %v", bundleID, testCase.wantBundle)
			}
		})
	}
}

func TestAdapter_CheckPermissions(t *testing.T) {
	tests := []struct {
		name            string
		mockPermissions bool
		wantErr         bool
	}{
		{
			name:            "permissions granted",
			mockPermissions: true,
			wantErr:         false,
		},
		{
			name:            "permissions denied",
			mockPermissions: false,
			wantErr:         true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			logger := zap.NewNop()
			mockClient := &accessibility.MockAXClient{
				MockPermissions: testCase.mockPermissions,
			}
			adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)
			ctx := context.Background()

			permissionsErr := adapter.CheckPermissions(ctx)

			if (permissionsErr != nil) != testCase.wantErr {
				t.Errorf(
					"CheckPermissions() error = %v, wantErr %v",
					permissionsErr,
					testCase.wantErr,
				)
			}
		})
	}
}

func TestAdapter_Logger(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	adapter := accessibility.NewAdapter(logger, []string{}, []string{}, mockClient)

	if adapter.Logger() != logger {
		t.Error("Logger() returned wrong logger")
	}
}

func TestAdapter_ClickableRoles(t *testing.T) {
	logger := zap.NewNop()
	mockClient := &accessibility.MockAXClient{}
	roles := []string{"AXButton", "AXLink"}
	adapter := accessibility.NewAdapter(logger, []string{}, roles, mockClient)

	result := adapter.ClickableRoles()

	if len(result) != len(roles) {
		t.Errorf("ClickableRoles() length = %d, want %d", len(result), len(roles))
	}

	if !reflect.DeepEqual(result, roles) {
		t.Errorf("ClickableRoles() = %v, want %v", result, roles)
	}

	// Ensure the returned slice is a defensive copy and doesn't expose internal state
	result[0] = "ModifiedRole"

	result2 := adapter.ClickableRoles()
	if !reflect.DeepEqual(result2, roles) {
		t.Errorf(
			"ClickableRoles() returned slice was not a defensive copy, internal state was modified: got %v, want %v",
			result2,
			roles,
		)
	}
}
