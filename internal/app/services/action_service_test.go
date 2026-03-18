package services_test

import (
	"context"
	"image"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
	"github.com/y3owk1n/neru/internal/core/ports/mocks"
)

// mockAccessibilityPort is a no-op mock implementation of ports.AccessibilityPort for testing.
// Cursor/screen operations are handled separately via the SystemPort mock (testSystemState).
type mockAccessibilityPort struct{}

// testSystemState holds the shared cursor/screen state used by the SystemPort mock.
type testSystemState struct {
	cursorPos         image.Point
	screenBounds      image.Rectangle
	moveCalls         []image.Point
	namedScreenBounds map[string]image.Rectangle // keyed by lowercase name
	screenNames       []string                   // ordered list returned by ScreenNames
}

func newTestSystemState() *testSystemState {
	return &testSystemState{
		cursorPos: image.Point{X: 100, Y: 100},
		screenBounds: image.Rectangle{
			Min: image.Point{X: 0, Y: 0},
			Max: image.Point{X: 1920, Y: 1080},
		},
		moveCalls: []image.Point{},
	}
}

func newSystemMock(state *testSystemState) *mocks.SystemMock {
	return &mocks.SystemMock{
		ScreenBoundsFunc: func(ctx context.Context) (image.Rectangle, error) {
			return state.screenBounds, nil
		},
		CursorPositionFunc: func(ctx context.Context) (image.Point, error) {
			return state.cursorPos, nil
		},
		MoveCursorToPointFunc: func(ctx context.Context, point image.Point, bypassSmooth bool) error {
			state.moveCalls = append(state.moveCalls, point)
			state.cursorPos = point

			return nil
		},
		ScreenBoundsByNameFunc: func(ctx context.Context, name string) (image.Rectangle, bool, error) {
			if state.namedScreenBounds == nil {
				return image.Rectangle{}, false, nil
			}

			bounds, ok := state.namedScreenBounds[strings.ToLower(name)]

			return bounds, ok, nil
		},
		ScreenNamesFunc: func(ctx context.Context) ([]string, error) {
			return state.screenNames, nil
		},
	}
}

func (m *mockAccessibilityPort) Health(ctx context.Context) error {
	return nil
}

func (m *mockAccessibilityPort) ClickableElements(
	ctx context.Context,
	filter ports.ElementFilter,
) ([]*element.Element, error) {
	return nil, nil
}

func (m *mockAccessibilityPort) PerformAction(
	ctx context.Context,
	elem *element.Element,
	actionType action.Type,
) error {
	return nil
}

func (m *mockAccessibilityPort) PerformActionAtPoint(
	ctx context.Context,
	actionType action.Type,
	point image.Point,
	modifiers action.Modifiers,
) error {
	return nil
}

func (m *mockAccessibilityPort) Scroll(ctx context.Context, deltaX, deltaY int) error {
	return nil
}

func (m *mockAccessibilityPort) FocusedAppBundleID(ctx context.Context) (string, error) {
	return "", nil
}

func (m *mockAccessibilityPort) IsAppExcluded(ctx context.Context, bundleID string) bool {
	return false
}

func (m *mockAccessibilityPort) ClearCache() {
	// No-op for mock
}

// mockOverlayPort is a mock implementation of OverlayPort for testing.
type mockOverlayPort struct{}

func (m *mockOverlayPort) Health(ctx context.Context) error {
	return nil
}

func (m *mockOverlayPort) ShowHints(ctx context.Context, hints []*hint.Interface) error {
	return nil
}

func (m *mockOverlayPort) ShowGrid(ctx context.Context) error {
	return nil
}

func (m *mockOverlayPort) Show() {
}

func (m *mockOverlayPort) DrawModeIndicator(x, y int) {
}

func (m *mockOverlayPort) DrawStickyModifiersIndicator(x, y int, symbols string) {
}

func (m *mockOverlayPort) Hide(ctx context.Context) error {
	return nil
}

func (m *mockOverlayPort) IsVisible() bool {
	return false
}

func (m *mockOverlayPort) Refresh(ctx context.Context) error {
	return nil
}

func newTestActionService(
	t *testing.T,
	state *testSystemState,
) *services.ActionService {
	t.Helper()

	actionConfig := config.ActionConfig{
		MoveMouseStep: 10,
		KeyBindings: config.ActionKeyBindingsCfg{
			MoveMouseUp:    "Up",
			MoveMouseDown:  "Down",
			MoveMouseLeft:  "Left",
			MoveMouseRight: "Right",
		},
	}

	logger, _ := zap.NewDevelopment()

	return services.NewActionService(
		&mockAccessibilityPort{},
		&mockOverlayPort{},
		newSystemMock(state),
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)
}

func TestMoveMouseTo_clampsToScreenBounds(t *testing.T) {
	state := newTestSystemState()
	actionService := newTestActionService(t, state)

	ctx := context.Background()

	err := actionService.MoveMouseTo(ctx, 2000, 2000) // Beyond screen bounds
	if err != nil {
		t.Fatalf("MoveMouseTo failed: %v", err)
	}

	if len(state.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call, got %d", len(state.moveCalls))
	}

	movedTo := state.moveCalls[0]
	if movedTo.X != 1919 || movedTo.Y != 1079 {
		t.Errorf("Expected cursor moved to (1919, 1079), got (%d, %d)", movedTo.X, movedTo.Y)
	}
}

func TestMoveMouseToCenter(t *testing.T) {
	state := newTestSystemState()
	actionService := newTestActionService(t, state)
	ctx := context.Background()

	err := actionService.MoveMouseToCenter(ctx, 0, 0)
	if err != nil {
		t.Fatalf("MoveMouseToCenter failed: %v", err)
	}

	if len(state.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call, got %d", len(state.moveCalls))
	}

	movedTo := state.moveCalls[0]
	if movedTo.X != 960 || movedTo.Y != 540 {
		t.Errorf("Expected cursor at screen center (960, 540), got (%d, %d)", movedTo.X, movedTo.Y)
	}
}

func TestMoveMouseToCenter_withOffset(t *testing.T) {
	state := newTestSystemState()
	actionService := newTestActionService(t, state)
	ctx := context.Background()

	err := actionService.MoveMouseToCenter(ctx, 50, -30)
	if err != nil {
		t.Fatalf("MoveMouseToCenter failed: %v", err)
	}

	if len(state.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call, got %d", len(state.moveCalls))
	}

	movedTo := state.moveCalls[0]

	expectedX, expectedY := 1010, 510
	if movedTo.X != expectedX || movedTo.Y != expectedY {
		t.Errorf(
			"Expected cursor at (%d, %d), got (%d, %d)",
			expectedX,
			expectedY,
			movedTo.X,
			movedTo.Y,
		)
	}
}

func TestMoveMouseTo_degenerateBounds(t *testing.T) {
	state := newTestSystemState()
	// Zero-width, zero-height screen (degenerate bounds)
	state.screenBounds = image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: 0, Y: 0},
	}
	actionService := newTestActionService(t, state)
	ctx := context.Background()

	err := actionService.MoveMouseTo(ctx, 500, 500)
	if err != nil {
		t.Fatalf("MoveMouseTo failed: %v", err)
	}

	if len(state.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call, got %d", len(state.moveCalls))
	}

	movedTo := state.moveCalls[0]

	// With degenerate bounds, cursor should clamp to Min (0, 0)
	if movedTo.X != 0 || movedTo.Y != 0 {
		t.Errorf("Expected cursor clamped to (0, 0), got (%d, %d)", movedTo.X, movedTo.Y)
	}
}

func TestMoveMouseRelative_movesFromCurrentPosition(t *testing.T) {
	state := newTestSystemState()
	actionService := newTestActionService(t, state)

	ctx := context.Background()

	err := actionService.MoveMouseRelative(ctx, 50, -30)
	if err != nil {
		t.Fatalf("MoveMouseRelative failed: %v", err)
	}

	if len(state.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call, got %d", len(state.moveCalls))
	}

	movedTo := state.moveCalls[0]
	expectedX := 100 + 50  // original X + deltaX
	expectedY := 100 + -30 // original Y + deltaY

	if movedTo.X != expectedX || movedTo.Y != expectedY {
		t.Errorf(
			"Expected cursor moved to (%d, %d), got (%d, %d)",
			expectedX,
			expectedY,
			movedTo.X,
			movedTo.Y,
		)
	}
}

func TestMoveMouseRelative_multipleCallsAccumulate(t *testing.T) {
	state := newTestSystemState()
	actionService := newTestActionService(t, state)

	ctx := context.Background()

	// First move: +10 x, -10 y
	err := actionService.MoveMouseRelative(ctx, 10, -10)
	if err != nil {
		t.Fatalf("First MoveMouseRelative failed: %v", err)
	}

	// Second move: +5 x, +20 y
	err = actionService.MoveMouseRelative(ctx, 5, 20)
	if err != nil {
		t.Fatalf("Second MoveMouseRelative failed: %v", err)
	}

	if len(state.moveCalls) != 2 {
		t.Fatalf("Expected 2 move calls, got %d", len(state.moveCalls))
	}

	// First call should be from original position (100, 100)
	firstCall := state.moveCalls[0]
	if firstCall.X != 110 || firstCall.Y != 90 {
		t.Errorf("First move: Expected (110, 90), got (%d, %d)", firstCall.X, firstCall.Y)
	}

	// Second call should be from the updated position after first move (110, 90)
	// MoveMouseRelative calls CursorPosition, which returns the updated position
	secondCall := state.moveCalls[1]
	if secondCall.X != 115 || secondCall.Y != 110 {
		t.Errorf("Second move: Expected (115, 110), got (%d, %d)", secondCall.X, secondCall.Y)
	}
}

func TestHandleDirectActionKey_directionalKeys(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		step      int
		expectedX int
		expectedY int
	}{
		{"Up moves cursor up", "Up", 10, 100, 90},
		{"Down moves cursor down", "Down", 15, 100, 115},
		{"Left moves cursor left", "Left", 20, 80, 100},
		{"Right moves cursor right", "Right", 25, 125, 100},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			state := newTestSystemState()

			actionConfig := config.ActionConfig{
				MoveMouseStep: testCase.step,
				KeyBindings: config.ActionKeyBindingsCfg{
					MoveMouseUp:    "Up",
					MoveMouseDown:  "Down",
					MoveMouseLeft:  "Left",
					MoveMouseRight: "Right",
				},
			}

			logger, _ := zap.NewDevelopment()

			actionService := services.NewActionService(
				&mockAccessibilityPort{},
				&mockOverlayPort{},
				newSystemMock(state),
				actionConfig,
				actionConfig.KeyBindings,
				actionConfig.MoveMouseStep,
				logger,
			)

			ctx := context.Background()

			actionName, handled, err := actionService.HandleDirectActionKey(ctx, testCase.key)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !handled {
				t.Errorf("Expected %s key to be handled as direct action", testCase.key)
			}

			if actionName != "move_mouse_relative" {
				t.Errorf("Expected action name 'move_mouse_relative', got %q", actionName)
			}

			if len(state.moveCalls) != 1 {
				t.Fatalf(
					"Expected 1 move call after %s key, got %d",
					testCase.key,
					len(state.moveCalls),
				)
			}

			movedTo := state.moveCalls[0]
			if movedTo.X != testCase.expectedX {
				t.Errorf("Expected X = %d, got %d", testCase.expectedX, movedTo.X)
			}

			if movedTo.Y != testCase.expectedY {
				t.Errorf("Expected Y = %d, got %d", testCase.expectedY, movedTo.Y)
			}
		})
	}
}

func TestHandleDirectActionKey_repeatedKeyPressesMoveContinuously(t *testing.T) {
	state := newTestSystemState()

	actionConfig := config.ActionConfig{
		MoveMouseStep: 10,
		KeyBindings: config.ActionKeyBindingsCfg{
			MoveMouseUp:    "Up",
			MoveMouseDown:  "Down",
			MoveMouseLeft:  "Left",
			MoveMouseRight: "Right",
		},
	}

	logger, _ := zap.NewDevelopment()

	actionService := services.NewActionService(
		&mockAccessibilityPort{},
		&mockOverlayPort{},
		newSystemMock(state),
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

	ctx := context.Background()

	// Press Right 3 times
	for i := range 3 {
		_, _, err := actionService.HandleDirectActionKey(ctx, "Right")
		if err != nil {
			t.Fatalf("HandleDirectActionKey failed on press %d: %v", i+1, err)
		}
	}

	if len(state.moveCalls) != 3 {
		t.Fatalf("Expected 3 move calls after 3 Right key presses, got %d", len(state.moveCalls))
	}

	// Each call should be from the updated cursor position (100 -> 110 -> 120 -> 130)
	// because the system mock's MoveCursorToPoint updates cursorPos
	expectedPositions := []struct{ x, y int }{
		{110, 100}, // 100 + 10
		{120, 100}, // 110 + 10
		{130, 100}, // 120 + 10
	}

	for i, expected := range expectedPositions {
		actual := state.moveCalls[i]
		if actual.X != expected.x || actual.Y != expected.y {
			t.Errorf(
				"Move %d: Expected (%d, %d), got (%d, %d)",
				i+1,
				expected.x,
				expected.y,
				actual.X,
				actual.Y,
			)
		}
	}
}

func TestHandleDirectActionKey_caseInsensitive(t *testing.T) {
	state := newTestSystemState()

	actionConfig := config.ActionConfig{
		MoveMouseStep: 10,
		KeyBindings: config.ActionKeyBindingsCfg{
			MoveMouseUp:    "Up",
			MoveMouseDown:  "Down",
			MoveMouseLeft:  "Left",
			MoveMouseRight: "Right",
		},
	}

	logger, _ := zap.NewDevelopment()

	actionService := services.NewActionService(
		&mockAccessibilityPort{},
		&mockOverlayPort{},
		newSystemMock(state),
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

	ctx := context.Background()

	// Test lowercase
	_, handled, err := actionService.HandleDirectActionKey(ctx, "up")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !handled {
		t.Error("Expected 'up' (lowercase) to be handled as direct action")
	}

	if len(state.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call after 'up' key, got %d", len(state.moveCalls))
	}

	movedTo := state.moveCalls[0]
	if movedTo.Y != 90 {
		t.Errorf("Expected Y decreased to 90, got %d", movedTo.Y)
	}
}

func TestMoveMouseTo_doesNotBounceBack(t *testing.T) {
	state := newTestSystemState()
	actionService := newTestActionService(t, state)
	ctx := context.Background()
	// Move cursor multiple times
	_, _, err := actionService.HandleDirectActionKey(ctx, "Right")
	if err != nil {
		t.Fatalf("HandleDirectActionKey failed: %v", err)
	}

	_, _, err = actionService.HandleDirectActionKey(ctx, "Right")
	if err != nil {
		t.Fatalf("HandleDirectActionKey failed: %v", err)
	}

	_, _, err = actionService.HandleDirectActionKey(ctx, "Down")
	if err != nil {
		t.Fatalf("HandleDirectActionKey failed: %v", err)
	}
	// Verify cursor position is updated via state
	finalPos := state.cursorPos
	// Final position should be (120, 110): started at (100, 100), +20 X, +10 Y
	if finalPos.X != 120 || finalPos.Y != 110 {
		t.Errorf(
			"Expected final cursor position at (120, 110), got (%d, %d)",
			finalPos.X,
			finalPos.Y,
		)
	}
	// All moves should have been made
	if len(state.moveCalls) != 3 {
		t.Errorf("Expected 3 move calls, got %d", len(state.moveCalls))
	}
}

func TestMoveMouseRelative_clampsToScreenBounds(t *testing.T) {
	state := newTestSystemState()
	// Start at bottom-right corner
	state.cursorPos = image.Point{X: 1910, Y: 1070}
	actionService := newTestActionService(t, state)
	ctx := context.Background()
	// Try to move beyond bounds
	err := actionService.MoveMouseRelative(ctx, 100, 100)
	if err != nil {
		t.Fatalf("MoveMouseRelative failed: %v", err)
	}

	if len(state.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call, got %d", len(state.moveCalls))
	}

	movedTo := state.moveCalls[0]
	// Should be clamped to screen bounds (exclusive Max)
	if movedTo.X != 1919 || movedTo.Y != 1079 {
		t.Errorf("Expected cursor clamped to (1919, 1079), got (%d, %d)", movedTo.X, movedTo.Y)
	}
}

func TestIsMoveMouseKey(t *testing.T) {
	actionConfig := config.ActionConfig{
		MoveMouseStep: 10,
		KeyBindings: config.ActionKeyBindingsCfg{
			MoveMouseUp:    "Up",
			MoveMouseDown:  "Down",
			MoveMouseLeft:  "Left",
			MoveMouseRight: "Right",
		},
	}

	logger, _ := zap.NewDevelopment()

	actionService := services.NewActionService(
		&mockAccessibilityPort{},
		&mockOverlayPort{},
		&mocks.SystemMock{},
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

	tests := []struct {
		key      string
		expected bool
	}{
		// Move mouse keys
		{"Up", true},
		{"DOWN", true},
		{"left", true},
		{"RIGHT", true},
		{"up", true},
		{"Down", true},
		{"Left", true},
		{"Right", true},
		// Non-move mouse keys
		{"Shift+L", false},
		{"Shift+R", false},
		{"Shift+M", false},
		{"a", false},
		{"1", false},
		{"", false},
	}

	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			result := actionService.IsMoveMouseKey(test.key)
			if result != test.expected {
				t.Errorf("IsMoveMouseKey(%q) = %v, want %v", test.key, result, test.expected)
			}
		})
	}
}

func TestIsMoveMouseKey_shiftLetterBindings(t *testing.T) {
	actionConfig := config.ActionConfig{
		MoveMouseStep: 10,
		KeyBindings: config.ActionKeyBindingsCfg{
			MoveMouseUp:    "Shift+K",
			MoveMouseDown:  "Shift+J",
			MoveMouseLeft:  "Shift+H",
			MoveMouseRight: "Shift+L",
		},
	}
	logger, _ := zap.NewDevelopment()
	actionService := services.NewActionService(
		&mockAccessibilityPort{},
		&mockOverlayPort{},
		&mocks.SystemMock{},
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

	tests := []struct {
		key      string
		expected bool
	}{
		// Shift+Letter bindings (direct match)
		{"Shift+K", true},
		{"Shift+J", true},
		{"Shift+H", true},
		{"Shift+L", true},
		// Uppercase letters (Shift+Letter normalization)
		{"K", true},
		{"J", true},
		{"H", true},
		{"L", true},
		// Non-move mouse keys
		{"k", false},
		{"j", false},
		{"a", false},
		{"Up", false},
		{"", false},
	}
	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			result := actionService.IsMoveMouseKey(test.key)
			if result != test.expected {
				t.Errorf("IsMoveMouseKey(%q) = %v, want %v", test.key, result, test.expected)
			}
		})
	}
}

func TestMoveMouseToCenterOfMonitor(t *testing.T) {
	state := newTestSystemState()
	state.namedScreenBounds = map[string]image.Rectangle{
		"dell u2720q": {
			Min: image.Point{X: 1920, Y: 0},
			Max: image.Point{X: 3840, Y: 1080},
		},
		"built-in retina display": {
			Min: image.Point{X: 0, Y: 0},
			Max: image.Point{X: 1920, Y: 1080},
		},
	}
	state.screenNames = []string{"Built-in Retina Display", "DELL U2720Q"}
	actionService := newTestActionService(t, state)
	ctx := context.Background()

	err := actionService.MoveMouseToCenterOfMonitor(ctx, "DELL U2720Q", 0, 0)
	if err != nil {
		t.Fatalf("MoveMouseToCenterOfMonitor failed: %v", err)
	}

	if len(state.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call, got %d", len(state.moveCalls))
	}

	movedTo := state.moveCalls[0]
	// Center of (1920,0)-(3840,1080) = (2880, 540)
	if movedTo.X != 2880 || movedTo.Y != 540 {
		t.Errorf("Expected cursor at (2880, 540), got (%d, %d)", movedTo.X, movedTo.Y)
	}
}

func TestMoveMouseToCenterOfMonitor_withOffset(t *testing.T) {
	state := newTestSystemState()
	state.namedScreenBounds = map[string]image.Rectangle{
		"built-in retina display": {
			Min: image.Point{X: 0, Y: 0},
			Max: image.Point{X: 1920, Y: 1080},
		},
	}
	state.screenNames = []string{"Built-in Retina Display"}
	actionService := newTestActionService(t, state)
	ctx := context.Background()

	err := actionService.MoveMouseToCenterOfMonitor(ctx, "Built-in Retina Display", 50, -30)
	if err != nil {
		t.Fatalf("MoveMouseToCenterOfMonitor failed: %v", err)
	}

	if len(state.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call, got %d", len(state.moveCalls))
	}

	movedTo := state.moveCalls[0]
	// Center (960, 540) + offset (50, -30) = (1010, 510)
	if movedTo.X != 1010 || movedTo.Y != 510 {
		t.Errorf("Expected cursor at (1010, 510), got (%d, %d)", movedTo.X, movedTo.Y)
	}
}

func TestMoveMouseToCenterOfMonitor_offsetClampedToBounds(t *testing.T) {
	state := newTestSystemState()
	state.namedScreenBounds = map[string]image.Rectangle{
		"small monitor": {
			Min: image.Point{X: 0, Y: 0},
			Max: image.Point{X: 800, Y: 600},
		},
	}
	state.screenNames = []string{"Small Monitor"}
	actionService := newTestActionService(t, state)
	ctx := context.Background()
	// Offset far beyond bounds
	err := actionService.MoveMouseToCenterOfMonitor(ctx, "Small Monitor", 9999, 9999)
	if err != nil {
		t.Fatalf("MoveMouseToCenterOfMonitor failed: %v", err)
	}

	if len(state.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call, got %d", len(state.moveCalls))
	}

	movedTo := state.moveCalls[0]
	// Should be clamped to (799, 599) — the max pixel within bounds
	if movedTo.X != 799 || movedTo.Y != 599 {
		t.Errorf("Expected cursor clamped to (799, 599), got (%d, %d)", movedTo.X, movedTo.Y)
	}
}

func TestMoveMouseToCenterOfMonitor_notFound(t *testing.T) {
	state := newTestSystemState()
	state.namedScreenBounds = map[string]image.Rectangle{
		"built-in retina display": {
			Min: image.Point{X: 0, Y: 0},
			Max: image.Point{X: 1920, Y: 1080},
		},
	}
	state.screenNames = []string{"Built-in Retina Display", "DELL U2720Q"}
	actionService := newTestActionService(t, state)
	ctx := context.Background()

	err := actionService.MoveMouseToCenterOfMonitor(ctx, "NonExistent Monitor", 0, 0)
	if err == nil {
		t.Fatal("Expected error for non-existent monitor, got nil")
	}

	if !derrors.IsCode(err, derrors.CodeInvalidInput) {
		t.Errorf("Expected CodeInvalidInput, got %v", err)
	}

	if !strings.Contains(err.Error(), "NonExistent Monitor") {
		t.Errorf("Error should contain monitor name, got: %s", err.Error())
	}

	if !strings.Contains(err.Error(), "Built-in Retina Display") {
		t.Errorf("Error should list available monitors, got: %s", err.Error())
	}

	if len(state.moveCalls) != 0 {
		t.Errorf("Expected no move calls, got %d", len(state.moveCalls))
	}
}

func TestMoveMouseToCenterOfMonitor_notFoundNoScreens(t *testing.T) {
	state := newTestSystemState()
	// No named screens configured — simulates no screens detected
	actionService := newTestActionService(t, state)
	ctx := context.Background()

	err := actionService.MoveMouseToCenterOfMonitor(ctx, "Anything", 0, 0)
	if err == nil {
		t.Fatal("Expected error for non-existent monitor, got nil")
	}

	if !derrors.IsCode(err, derrors.CodeInvalidInput) {
		t.Errorf("Expected CodeInvalidInput, got %v", err)
	}
	// Should NOT contain "available monitors" since there are none
	if strings.Contains(err.Error(), "available monitors") {
		t.Errorf("Error should not list available monitors when none exist, got: %s", err.Error())
	}
}

func TestMoveMouseToCenterOfMonitor_systemError(t *testing.T) {
	state := newTestSystemState()
	systemMock := newSystemMock(state)
	// Override ScreenBoundsByNameFunc to return an error
	systemMock.ScreenBoundsByNameFunc = func(
		ctx context.Context,
		name string,
	) (image.Rectangle, bool, error) {
		return image.Rectangle{}, false, derrors.New(derrors.CodeInternal, "system failure")
	}
	actionConfig := config.ActionConfig{
		MoveMouseStep: 10,
		KeyBindings: config.ActionKeyBindingsCfg{
			MoveMouseUp:    "Up",
			MoveMouseDown:  "Down",
			MoveMouseLeft:  "Left",
			MoveMouseRight: "Right",
		},
	}
	logger, _ := zap.NewDevelopment()
	actionService := services.NewActionService(
		&mockAccessibilityPort{},
		&mockOverlayPort{},
		systemMock,
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)
	ctx := context.Background()

	err := actionService.MoveMouseToCenterOfMonitor(ctx, "Any Monitor", 0, 0)
	if err == nil {
		t.Fatal("Expected error when system fails, got nil")
	}

	if !derrors.IsCode(err, derrors.CodeAccessibilityFailed) {
		t.Errorf("Expected CodeAccessibilityFailed, got %v", err)
	}

	if len(state.moveCalls) != 0 {
		t.Errorf("Expected no move calls, got %d", len(state.moveCalls))
	}
}

func TestMoveMouseToCenterOfMonitor_caseInsensitive(t *testing.T) {
	state := newTestSystemState()
	state.namedScreenBounds = map[string]image.Rectangle{
		"dell u2720q": {
			Min: image.Point{X: 0, Y: 0},
			Max: image.Point{X: 2560, Y: 1440},
		},
	}
	state.screenNames = []string{"DELL U2720Q"}
	actionService := newTestActionService(t, state)
	ctx := context.Background()
	// Use mixed case that differs from both the key and the screen name
	err := actionService.MoveMouseToCenterOfMonitor(ctx, "Dell U2720Q", 0, 0)
	if err != nil {
		t.Fatalf("MoveMouseToCenterOfMonitor failed: %v", err)
	}

	if len(state.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call, got %d", len(state.moveCalls))
	}

	movedTo := state.moveCalls[0]
	// Center of (0,0)-(2560,1440) = (1280, 720)
	if movedTo.X != 1280 || movedTo.Y != 720 {
		t.Errorf("Expected cursor at (1280, 720), got (%d, %d)", movedTo.X, movedTo.Y)
	}
}

func TestHandleDirectActionKey_shiftLetterBindings(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		expectedX int
		expectedY int
	}{
		{"Shift+K (uppercase K) moves up", "K", 100, 90},
		{"Shift+J (uppercase J) moves down", "J", 100, 110},
		{"Shift+H (uppercase H) moves left", "H", 90, 100},
		{"Shift+L (uppercase L) moves right", "L", 110, 100},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			state := newTestSystemState()
			actionConfig := config.ActionConfig{
				MoveMouseStep: 10,
				KeyBindings: config.ActionKeyBindingsCfg{
					MoveMouseUp:    "Shift+K",
					MoveMouseDown:  "Shift+J",
					MoveMouseLeft:  "Shift+H",
					MoveMouseRight: "Shift+L",
				},
			}
			logger, _ := zap.NewDevelopment()
			actionService := services.NewActionService(
				&mockAccessibilityPort{},
				&mockOverlayPort{},
				newSystemMock(state),
				actionConfig,
				actionConfig.KeyBindings,
				actionConfig.MoveMouseStep,
				logger,
			)
			ctx := context.Background()

			actionName, handled, err := actionService.HandleDirectActionKey(ctx, testCase.key)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !handled {
				t.Errorf("Expected %s key to be handled as direct action", testCase.key)
			}

			if actionName != "move_mouse_relative" {
				t.Errorf("Expected action name 'move_mouse_relative', got %q", actionName)
			}

			if len(state.moveCalls) != 1 {
				t.Fatalf(
					"Expected 1 move call after %s key, got %d",
					testCase.key,
					len(state.moveCalls),
				)
			}

			movedTo := state.moveCalls[0]
			if movedTo.X != testCase.expectedX {
				t.Errorf("Expected X = %d, got %d", testCase.expectedX, movedTo.X)
			}

			if movedTo.Y != testCase.expectedY {
				t.Errorf("Expected Y = %d, got %d", testCase.expectedY, movedTo.Y)
			}
		})
	}
}
