package services_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// mockAccessibilityPort is a mock implementation of AccessibilityPort for testing.
type mockAccessibilityPort struct {
	cursorPos    image.Point
	screenBounds image.Rectangle
	moveCalls    []image.Point
}

func newMockAccessibilityPort() *mockAccessibilityPort {
	return &mockAccessibilityPort{
		cursorPos: image.Point{X: 100, Y: 100},
		screenBounds: image.Rectangle{
			Min: image.Point{X: 0, Y: 0},
			Max: image.Point{X: 1920, Y: 1080},
		},
		moveCalls: []image.Point{},
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

func (m *mockAccessibilityPort) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	return m.screenBounds, nil
}

func (m *mockAccessibilityPort) MoveCursorToPoint(
	ctx context.Context,
	point image.Point,
	bypassSmooth bool,
) error {
	m.moveCalls = append(m.moveCalls, point)
	m.cursorPos = point // Update the simulated cursor position

	return nil
}

func (m *mockAccessibilityPort) CursorPosition(ctx context.Context) (image.Point, error) {
	return m.cursorPos, nil
}

func (m *mockAccessibilityPort) CheckPermissions(ctx context.Context) error {
	return nil
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

func (m *mockOverlayPort) DrawScrollHighlight(
	ctx context.Context,
	rect image.Rectangle,
	color string,
	width int,
) error {
	return nil
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

func TestMoveMouseTo_clampsToScreenBounds(t *testing.T) {
	mockAcc := newMockAccessibilityPort()

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
		mockAcc,
		&mockOverlayPort{},
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

	ctx := context.Background()

	err := actionService.MoveMouseTo(ctx, 2000, 2000) // Beyond screen bounds
	if err != nil {
		t.Fatalf("MoveMouseTo failed: %v", err)
	}

	if len(mockAcc.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call, got %d", len(mockAcc.moveCalls))
	}

	movedTo := mockAcc.moveCalls[0]
	if movedTo.X != 1920 || movedTo.Y != 1080 {
		t.Errorf("Expected cursor moved to (1920, 1080), got (%d, %d)", movedTo.X, movedTo.Y)
	}
}

func TestMoveMouseRelative_movesFromCurrentPosition(t *testing.T) {
	mockAcc := newMockAccessibilityPort()

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
		mockAcc,
		&mockOverlayPort{},
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

	ctx := context.Background()

	err := actionService.MoveMouseRelative(ctx, 50, -30)
	if err != nil {
		t.Fatalf("MoveMouseRelative failed: %v", err)
	}

	if len(mockAcc.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call, got %d", len(mockAcc.moveCalls))
	}

	movedTo := mockAcc.moveCalls[0]
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
	mockAcc := newMockAccessibilityPort()

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
		mockAcc,
		&mockOverlayPort{},
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

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

	if len(mockAcc.moveCalls) != 2 {
		t.Fatalf("Expected 2 move calls, got %d", len(mockAcc.moveCalls))
	}

	// First call should be from original position (100, 100)
	firstCall := mockAcc.moveCalls[0]
	if firstCall.X != 110 || firstCall.Y != 90 {
		t.Errorf("First move: Expected (110, 90), got (%d, %d)", firstCall.X, firstCall.Y)
	}

	// Second call should be from the updated position after first move (110, 90)
	// MoveMouseRelative calls CursorPosition, which returns the updated position
	secondCall := mockAcc.moveCalls[1]
	if secondCall.X != 115 || secondCall.Y != 110 {
		t.Errorf("Second move: Expected (115, 110), got (%d, %d)", secondCall.X, secondCall.Y)
	}
}

func TestHandleDirectActionKey_Up_movesCursorUp(t *testing.T) {
	mockAcc := newMockAccessibilityPort()

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
		mockAcc,
		&mockOverlayPort{},
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

	ctx := context.Background()

	handled := actionService.HandleDirectActionKey(ctx, "Up")
	if !handled {
		t.Error("Expected Up key to be handled as direct action")
	}

	if len(mockAcc.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call after Up key, got %d", len(mockAcc.moveCalls))
	}

	movedTo := mockAcc.moveCalls[0]
	// Up should decrease Y
	if movedTo.X != 100 {
		t.Errorf("Expected X unchanged at 100, got %d", movedTo.X)
	}

	if movedTo.Y != 90 {
		t.Errorf("Expected Y decreased to 90, got %d", movedTo.Y)
	}
}

func TestHandleDirectActionKey_Down_movesCursorDown(t *testing.T) {
	mockAcc := newMockAccessibilityPort()

	actionConfig := config.ActionConfig{
		MoveMouseStep: 15,
		KeyBindings: config.ActionKeyBindingsCfg{
			MoveMouseUp:    "Up",
			MoveMouseDown:  "Down",
			MoveMouseLeft:  "Left",
			MoveMouseRight: "Right",
		},
	}

	logger, _ := zap.NewDevelopment()

	actionService := services.NewActionService(
		mockAcc,
		&mockOverlayPort{},
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

	ctx := context.Background()

	handled := actionService.HandleDirectActionKey(ctx, "Down")
	if !handled {
		t.Error("Expected Down key to be handled as direct action")
	}

	if len(mockAcc.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call after Down key, got %d", len(mockAcc.moveCalls))
	}

	movedTo := mockAcc.moveCalls[0]
	if movedTo.X != 100 {
		t.Errorf("Expected X unchanged at 100, got %d", movedTo.X)
	}

	if movedTo.Y != 115 {
		t.Errorf("Expected Y increased to 115, got %d", movedTo.Y)
	}
}

func TestHandleDirectActionKey_Left_movesCursorLeft(t *testing.T) {
	mockAcc := newMockAccessibilityPort()

	actionConfig := config.ActionConfig{
		MoveMouseStep: 20,
		KeyBindings: config.ActionKeyBindingsCfg{
			MoveMouseUp:    "Up",
			MoveMouseDown:  "Down",
			MoveMouseLeft:  "Left",
			MoveMouseRight: "Right",
		},
	}

	logger, _ := zap.NewDevelopment()

	actionService := services.NewActionService(
		mockAcc,
		&mockOverlayPort{},
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

	ctx := context.Background()

	handled := actionService.HandleDirectActionKey(ctx, "Left")
	if !handled {
		t.Error("Expected Left key to be handled as direct action")
	}

	if len(mockAcc.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call after Left key, got %d", len(mockAcc.moveCalls))
	}

	movedTo := mockAcc.moveCalls[0]
	if movedTo.X != 80 {
		t.Errorf("Expected X decreased to 80, got %d", movedTo.X)
	}

	if movedTo.Y != 100 {
		t.Errorf("Expected Y unchanged at 100, got %d", movedTo.Y)
	}
}

func TestHandleDirectActionKey_Right_movesCursorRight(t *testing.T) {
	mockAcc := newMockAccessibilityPort()

	actionConfig := config.ActionConfig{
		MoveMouseStep: 25,
		KeyBindings: config.ActionKeyBindingsCfg{
			MoveMouseUp:    "Up",
			MoveMouseDown:  "Down",
			MoveMouseLeft:  "Left",
			MoveMouseRight: "Right",
		},
	}

	logger, _ := zap.NewDevelopment()

	actionService := services.NewActionService(
		mockAcc,
		&mockOverlayPort{},
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

	ctx := context.Background()

	handled := actionService.HandleDirectActionKey(ctx, "Right")
	if !handled {
		t.Error("Expected Right key to be handled as direct action")
	}

	if len(mockAcc.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call after Right key, got %d", len(mockAcc.moveCalls))
	}

	movedTo := mockAcc.moveCalls[0]
	if movedTo.X != 125 {
		t.Errorf("Expected X increased to 125, got %d", movedTo.X)
	}

	if movedTo.Y != 100 {
		t.Errorf("Expected Y unchanged at 100, got %d", movedTo.Y)
	}
}

func TestHandleDirectActionKey_repeatedKeyPressesMoveContinuously(t *testing.T) {
	mockAcc := newMockAccessibilityPort()

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
		mockAcc,
		&mockOverlayPort{},
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

	ctx := context.Background()

	// Press Right 3 times
	actionService.HandleDirectActionKey(ctx, "Right")
	actionService.HandleDirectActionKey(ctx, "Right")
	actionService.HandleDirectActionKey(ctx, "Right")

	if len(mockAcc.moveCalls) != 3 {
		t.Fatalf("Expected 3 move calls after 3 Right key presses, got %d", len(mockAcc.moveCalls))
	}

	// Each call should be from the updated cursor position (100 -> 110 -> 120 -> 130)
	// because mockAcc.MoveCursorToPoint updates cursorPos
	expectedPositions := []struct{ x, y int }{
		{110, 100}, // 100 + 10
		{120, 100}, // 110 + 10
		{130, 100}, // 120 + 10
	}

	for i, expected := range expectedPositions {
		actual := mockAcc.moveCalls[i]
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

func TestHandleDirectActionKey_cursorPositionUpdatesBetweenCalls(t *testing.T) {
	mockAcc := newMockAccessibilityPort()

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
		mockAcc,
		&mockOverlayPort{},
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

	ctx := context.Background()

	// Press Right 3 times
	actionService.HandleDirectActionKey(ctx, "Right")
	actionService.HandleDirectActionKey(ctx, "Right")
	actionService.HandleDirectActionKey(ctx, "Right")

	if len(mockAcc.moveCalls) != 3 {
		t.Fatalf("Expected 3 move calls, got %d", len(mockAcc.moveCalls))
	}

	// Each move should be relative to the updated cursor position
	// First: 100 + 10 = 110
	// Second: 110 + 10 = 120
	// Third: 120 + 10 = 130
	expectedPositions := []struct{ x, y int }{
		{110, 100},
		{120, 100},
		{130, 100},
	}

	for i, expected := range expectedPositions {
		actual := mockAcc.moveCalls[i]
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
	mockAcc := newMockAccessibilityPort()

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
		mockAcc,
		&mockOverlayPort{},
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

	ctx := context.Background()

	// Test lowercase
	handled := actionService.HandleDirectActionKey(ctx, "up")
	if !handled {
		t.Error("Expected 'up' (lowercase) to be handled as direct action")
	}

	if len(mockAcc.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call after 'up' key, got %d", len(mockAcc.moveCalls))
	}

	movedTo := mockAcc.moveCalls[0]
	if movedTo.Y != 90 {
		t.Errorf("Expected Y decreased to 90, got %d", movedTo.Y)
	}
}

func TestMoveMouseTo_doesNotBounceBack(t *testing.T) {
	mockAcc := newMockAccessibilityPort()

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
		mockAcc,
		&mockOverlayPort{},
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

	ctx := context.Background()

	// Move cursor multiple times
	actionService.HandleDirectActionKey(ctx, "Right")
	actionService.HandleDirectActionKey(ctx, "Right")
	actionService.HandleDirectActionKey(ctx, "Down")

	// Verify cursor position is updated
	finalPos, err := mockAcc.CursorPosition(ctx)
	if err != nil {
		t.Fatalf("CursorPosition failed: %v", err)
	}

	// Final position should be (120, 110): started at (100, 100), +20 X, +10 Y
	if finalPos.X != 120 || finalPos.Y != 110 {
		t.Errorf(
			"Expected final cursor position at (120, 110), got (%d, %d)",
			finalPos.X,
			finalPos.Y,
		)
	}

	// All moves should have been made
	if len(mockAcc.moveCalls) != 3 {
		t.Errorf("Expected 3 move calls, got %d", len(mockAcc.moveCalls))
	}
}

func TestMoveMouseRelative_clampsToScreenBounds(t *testing.T) {
	mockAcc := newMockAccessibilityPort()
	// Start at bottom-right corner
	mockAcc.cursorPos = image.Point{X: 1910, Y: 1070}

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
		mockAcc,
		&mockOverlayPort{},
		actionConfig,
		actionConfig.KeyBindings,
		actionConfig.MoveMouseStep,
		logger,
	)

	ctx := context.Background()

	// Try to move beyond bounds
	err := actionService.MoveMouseRelative(ctx, 100, 100)
	if err != nil {
		t.Fatalf("MoveMouseRelative failed: %v", err)
	}

	if len(mockAcc.moveCalls) != 1 {
		t.Fatalf("Expected 1 move call, got %d", len(mockAcc.moveCalls))
	}

	movedTo := mockAcc.moveCalls[0]
	// Should be clamped to screen bounds
	if movedTo.X != 1920 || movedTo.Y != 1080 {
		t.Errorf("Expected cursor clamped to (1920, 1080), got (%d, %d)", movedTo.X, movedTo.Y)
	}
}

func TestIsMoveMouseKey(t *testing.T) {
	mockAcc := newMockAccessibilityPort()

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
		mockAcc,
		&mockOverlayPort{},
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
