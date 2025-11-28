package hint_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	"go.uber.org/zap"
)

func TestRouter_RouteKey(t *testing.T) {
	logger := zap.NewNop()
	manager := hint.NewManager(logger)
	router := hint.NewRouter(manager, logger)

	tests := []struct {
		name       string
		key        string
		setupHints bool
		wantExit   bool
		wantExact  bool
	}{
		{
			name:       "escape key exits",
			key:        "escape",
			setupHints: false,
			wantExit:   true,
			wantExact:  false,
		},
		{
			name:       "backspace key",
			key:        "backspace",
			setupHints: false,
			wantExit:   false,
			wantExact:  false,
		},
		{
			name:       "regular key input",
			key:        "a",
			setupHints: false,
			wantExit:   false,
			wantExact:  false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := router.RouteKey(testCase.key)

			if result.Exit() != testCase.wantExit {
				t.Errorf("Exit() = %v, want %v", result.Exit(), testCase.wantExit)
			}

			if (result.ExactHint() != nil) != testCase.wantExact {
				t.Errorf(
					"ExactHint() = %v, want exact = %v",
					result.ExactHint(),
					testCase.wantExact,
				)
			}
		})
	}
}

func TestRouter_WithHints(t *testing.T) {
	logger := zap.NewNop()
	manager := hint.NewManager(logger)
	router := hint.NewRouter(manager, logger)

	// Set up hints in manager
	elem, _ := element.NewElement("test", image.Rect(0, 0, 10, 10), element.RoleButton)
	h, _ := hint.NewHint("A", elem, image.Point{X: 5, Y: 5})
	collection := hint.NewCollection([]*hint.Interface{h})
	manager.SetHints(collection)

	// Test exact match
	result := router.RouteKey("a")
	if result.Exit() {
		t.Error("Should not exit on exact match")
	}

	if result.ExactHint() == nil {
		t.Error("Should have exact hint match")
	}

	if result.ExactHint().Label() != "A" {
		t.Errorf("Expected hint label 'A', got %s", result.ExactHint().Label())
	}

	// Test partial match
	result = router.RouteKey("b")
	if result.Exit() {
		t.Error("Should not exit on partial match")
	}

	if result.ExactHint() != nil {
		t.Error("Should not have exact hint on partial match")
	}
}
