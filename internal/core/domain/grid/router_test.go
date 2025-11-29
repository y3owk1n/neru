//go:build unit

package grid_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/grid"
	"go.uber.org/zap"
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
			name:         "escape key exits",
			key:          "escape",
			wantExit:     true,
			wantComplete: false,
		},
		{
			name:         "escape sequence exits",
			key:          "\x1b",
			wantExit:     true,
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

	// Test escape key
	result := router.RouteKey("escape")
	if !result.Exit() {
		t.Error("Escape key should cause exit")
	}

	if result.Complete() {
		t.Error("Escape key should not complete coordinate")
	}
}
