package grid_test

import (
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain/grid"
)

func TestGridRouter_RouteKey(t *testing.T) {
	logger := zap.NewNop()

	// Create a simple grid for testing
	testGrid := grid.NewGrid("ABCD", image.Rect(0, 0, 100, 100), logger)
	manager := grid.NewManager(testGrid, 3, 3, "123456789", nil, nil, logger)
	router := grid.NewRouter(manager, logger)

	tests := []struct {
		name         string
		key          string
		wantExit     bool
		wantComplete bool
	}{
		{
			name:         "escape key does not exit in router",
			key:          "escape",
			wantExit:     false,
			wantComplete: false,
		},
		{
			name:         "escape sequence does not exit in router",
			key:          "\x1b",
			wantExit:     false,
			wantComplete: false,
		},
		{
			name:         "regular key - incomplete",
			key:          "a",
			wantExit:     false,
			wantComplete: false,
		},
		{
			name:         "backspace key",
			key:          "backspace",
			wantExit:     false,
			wantComplete: false,
		},
		{
			name:         "delete key",
			key:          "\x7f",
			wantExit:     false,
			wantComplete: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := router.RouteKey(testCase.key)

			if result.Exit() != testCase.wantExit {
				t.Errorf("Exit() = %v, want %v", result.Exit(), testCase.wantExit)
			}

			if result.Complete() != testCase.wantComplete {
				t.Errorf("Complete() = %v, want %v", result.Complete(), testCase.wantComplete)
			}
		})
	}
}

func TestGridRouter_EscapeKey(t *testing.T) {
	logger := zap.NewNop()

	// Create a simple grid for testing
	testGrid := grid.NewGrid("ABCD", image.Rect(0, 0, 100, 100), logger)
	manager := grid.NewManager(testGrid, 3, 3, "123456789", nil, nil, logger)
	router := grid.NewRouter(manager, logger)

	// Escape is handled by top-level custom hotkeys now, not router.
	result := router.RouteKey("escape")
	if result.Exit() {
		t.Error("Escape key should not cause router exit")
	}

	if result.Complete() {
		t.Error("Escape key should not complete coordinate")
	}
}
