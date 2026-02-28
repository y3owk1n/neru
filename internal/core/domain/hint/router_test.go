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
	manager := hint.NewManager(logger, nil)
	router := hint.NewRouter(manager, logger)

	tests := []struct {
		name      string
		key       string
		wantExit  bool
		wantExact bool
	}{
		{
			name:      "escape key exits",
			key:       "escape",
			wantExit:  true,
			wantExact: false,
		},
		{
			name:      "backspace key",
			key:       "backspace",
			wantExit:  false,
			wantExact: false,
		},
		{
			name:      "regular key input",
			key:       "a",
			wantExit:  false,
			wantExact: false,
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
	manager := hint.NewManager(logger, nil)
	router := hint.NewRouter(manager, logger)

	// Set up hints in manager with multi-character labels
	elem1, _ := element.NewElement("test1", image.Rect(0, 0, 10, 10), element.RoleButton)
	elem2, _ := element.NewElement("test2", image.Rect(10, 10, 20, 20), element.RoleButton)

	h1, _ := hint.NewHint("AB", elem1, image.Point{X: 5, Y: 5})
	h2, _ := hint.NewHint("AC", elem2, image.Point{X: 15, Y: 15})

	collection := hint.NewCollection([]*hint.Interface{h1, h2})
	manager.SetHints(collection)

	// Test partial match - typing "A" should not complete yet
	result := router.RouteKey("a")
	if result.Exit() {
		t.Error("Should not exit on partial match")
	}

	if result.ExactHint() != nil {
		t.Error("Should not have exact hint match for partial input 'a'")
	}

	// Test exact match - typing "AB" should complete
	result = router.RouteKey("b")
	if result.Exit() {
		t.Error("Should not exit on exact match")
	}

	if result.ExactHint() == nil {
		t.Error("Should have exact hint match for 'ab'")
	}

	if result.ExactHint().Label() != "AB" {
		t.Errorf("Expected hint label 'AB', got %s", result.ExactHint().Label())
	}

	// Reset and test another partial/exact sequence
	manager.SetHints(collection) // Reset input state

	// Type "A" again (partial)
	result = router.RouteKey("a")
	if result.ExactHint() != nil {
		t.Error("Should not have exact hint match for partial input 'a' (second time)")
	}

	// Type "C" to complete "AC"
	result = router.RouteKey("c")
	if result.ExactHint() == nil {
		t.Error("Should have exact hint match for 'ac'")
	}

	if result.ExactHint().Label() != "AC" {
		t.Errorf("Expected hint label 'AC', got %s", result.ExactHint().Label())
	}
}
