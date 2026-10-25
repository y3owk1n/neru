package hint_test

import (
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/hint"
)

func TestRouter_RouteKey(t *testing.T) {
	logger := zap.NewNop()
	manager := hint.NewManager(logger, nil)
	router := hint.NewRouter(manager, logger)

	tests := []struct {
		name      string
		key       string
		wantExact bool
	}{
		{
			name:      "escape key does not exit in router",
			key:       "escape",
			wantExact: false,
		},
		{
			name:      "backspace key",
			key:       "backspace",
			wantExact: false,
		},
		{
			name:      "regular key input",
			key:       "a",
			wantExact: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := router.RouteKey(testCase.key)
			if err != nil {
				t.Fatalf("RouteKey: %v", err)
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

	err := manager.SetHints(collection)
	if err != nil {
		t.Fatalf("SetHints: %v", err)
	}

	// Typing "A" is a partial match and does not complete yet.
	result, err := router.RouteKey("a")
	if err != nil {
		t.Fatalf("RouteKey: %v", err)
	}

	if result.ExactHint() != nil {
		t.Error("Should not have exact hint match for partial input 'a'")
	}

	// Typing "AB" is an exact match and completes.
	result, err = router.RouteKey("b")
	if err != nil {
		t.Fatalf("RouteKey: %v", err)
	}

	if result.ExactHint() == nil {
		t.Error("Should have exact hint match for 'ab'")
	}

	if result.ExactHint().Label() != "AB" {
		t.Errorf("Expected hint label 'AB', got %s", result.ExactHint().Label())
	}

	// Reset the input and run another partial then exact sequence.
	err = manager.SetHints(collection)
	if err != nil {
		t.Fatalf("SetHints: %v", err)
	}

	// Type "A" again (partial)
	result, err = router.RouteKey("a")
	if err != nil {
		t.Fatalf("RouteKey: %v", err)
	}

	if result.ExactHint() != nil {
		t.Error("Should not have exact hint match for partial input 'a' (second time)")
	}

	// Type "C" to complete "AC"
	result, err = router.RouteKey("c")
	if err != nil {
		t.Fatalf("RouteKey: %v", err)
	}

	if result.ExactHint() == nil {
		t.Error("Should have exact hint match for 'ac'")
	}

	if result.ExactHint().Label() != "AC" {
		t.Errorf("Expected hint label 'AC', got %s", result.ExactHint().Label())
	}
}

func TestRouter_UnmatchedKey(t *testing.T) {
	logger := zap.NewNop()
	manager := hint.NewManager(logger, nil)
	router := hint.NewRouter(manager, logger)

	elem1, _ := element.NewElement("test1", image.Rect(0, 0, 10, 10), element.RoleButton)
	h1, _ := hint.NewHint("AB", elem1, image.Point{X: 5, Y: 5})
	collection := hint.NewCollection([]*hint.Interface{h1})

	err := manager.SetHints(collection)
	if err != nil {
		t.Fatalf("SetHints: %v", err)
	}

	// With exitOnUnmatched off, an unmatched key resets the input and is not reported.
	result, err := router.RouteKey("z")
	if err != nil {
		t.Fatalf("RouteKey: %v", err)
	}

	if result.Unmatched() {
		t.Error("result.Unmatched() = true, want false when exitOnUnmatched is false")
	}

	manager.SetExitOnUnmatched(true)

	// Type matching prefix "A"
	result, err = router.RouteKey("a")
	if err != nil {
		t.Fatalf("RouteKey: %v", err)
	}

	if result.Unmatched() {
		t.Error("result.Unmatched() = true, want false for matching prefix 'a'")
	}

	// Type unmatched key "Z"
	result, err = router.RouteKey("z")
	if err != nil {
		t.Fatalf("RouteKey: %v", err)
	}

	if !result.Unmatched() {
		t.Error("result.Unmatched() = false, want true for unmatched key 'z'")
	}
}
