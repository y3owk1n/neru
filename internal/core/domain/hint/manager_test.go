package hint_test

import (
	"context"
	"errors"
	"image"
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
)

func TestManager_Filtering(t *testing.T) {
	// Setup hints
	element, _ := element.NewElement(element.ID("1"), image.Rect(0, 0, 10, 10), element.RoleButton)
	h1, _ := hint.NewHint("AA", element, image.Point{0, 0})
	h2, _ := hint.NewHint("AB", element, image.Point{0, 0})
	h3, _ := hint.NewHint("AC", element, image.Point{0, 0})

	collection := hint.NewCollection([]*hint.Interface{h1, h2, h3})

	manager := hint.NewManager(logger.Get(), nil)

	err := manager.SetHints(collection)
	if err != nil {
		t.Fatalf("SetHints: %v", err)
	}

	tests := []struct {
		name        string
		input       string
		wantCount   int
		wantMatched string // Label of the exact match if any
	}{
		{"empty input", "", 3, ""},
		{"partial match A", "A", 3, ""},
		{"exact match AA", "AA", 1, "AA"},
		{"exact match AB", "AB", 1, "AB"},
		{"no match AD", "AD", 3, ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := manager.Reset()
			if err != nil {
				t.Fatalf("Reset: %v", err)
			}

			var (
				match *hint.Interface
				found bool
			)

			for _, char := range testCase.input {
				var err error

				match, found, err = manager.HandleInput(string(char))
				if err != nil {
					t.Fatalf("HandleInput: %v", err)
				}
			}

			filtered := manager.FilteredHints()
			if len(filtered) != testCase.wantCount {
				t.Errorf(
					"FilteredHints() count = %d, want %d",
					len(filtered),
					testCase.wantCount,
				)
			}

			if testCase.wantMatched != "" {
				if !found || match == nil {
					t.Errorf("Expected exact match for %s, got nil", testCase.wantMatched)
				} else if match.Label() != testCase.wantMatched {
					t.Errorf("Expected exact match %s, got %s", testCase.wantMatched, match.Label())
				}
			} else if found {
				t.Errorf("Expected no exact match, got %s", match.Label())
			}
		})
	}
}

func TestManager_Backspace(t *testing.T) {
	element, _ := element.NewElement(element.ID("1"), image.Rect(0, 0, 10, 10), element.RoleButton)
	h1, _ := hint.NewHint("AA", element, image.Point{0, 0})
	collection := hint.NewCollection([]*hint.Interface{h1})

	manager := hint.NewManager(logger.Get(), nil)

	err := manager.SetHints(collection)
	if err != nil {
		t.Fatalf("SetHints: %v", err)
	}

	// Type 'A'
	_, _, err = manager.HandleInput("A")
	if err != nil {
		t.Fatalf("HandleInput: %v", err)
	}

	if len(manager.FilteredHints()) != 1 {
		t.Error("Expected 1 hint after 'A'")
	}

	// Backspace via explicit API (backspace is driven by hotkeys, not HandleInput)
	err = manager.HandleBackspace()
	if err != nil {
		t.Fatalf("HandleBackspace: %v", err)
	}

	if len(manager.FilteredHints()) != 1 {
		t.Error("Expected 1 hint after Backspace")
	}

	if manager.CurrentInput() != "" {
		t.Errorf("Expected empty input, got %q", manager.CurrentInput())
	}
}

func TestManager_ClearReleasesSessionState(t *testing.T) {
	elem, _ := element.NewElement(element.ID("1"), image.Rect(0, 0, 10, 10), element.RoleButton)
	h1, _ := hint.NewHint("AA", elem, image.Point{0, 0})
	h2, _ := hint.NewHint("AB", elem, image.Point{0, 0})
	collection := hint.NewCollection([]*hint.Interface{h1, h2})

	manager := hint.NewManager(logger.Get(), nil)

	err := manager.SetHints(collection)
	if err != nil {
		t.Fatalf("SetHints: %v", err)
	}

	_, _, err = manager.HandleInput("A")
	if err != nil {
		t.Fatalf("HandleInput: %v", err)
	}

	err = manager.Clear()
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}

	if manager.CurrentInput() != "" {
		t.Errorf("Expected input to be cleared, got %q", manager.CurrentInput())
	}

	if filtered := manager.FilteredHints(); filtered != nil {
		t.Fatalf("Expected no filtered hints after Clear(), got %d", len(filtered))
	}
}

func TestHintManager_RouterIntegration(t *testing.T) {
	logger := logger.Get()

	// Create hint manager
	hintManager := hint.NewManager(logger, nil)

	// Create hint router
	hintRouter := hint.NewRouter(hintManager, logger)

	// Create some test elements
	elem1, _ := element.NewElement("elem1", image.Rect(10, 10, 50, 50), element.RoleButton)
	elem2, _ := element.NewElement("elem2", image.Rect(60, 10, 100, 50), element.RoleButton)
	elem3, _ := element.NewElement("elem3", image.Rect(10, 60, 50, 100), element.RoleButton)
	testElements := []*element.Element{elem1, elem2, elem3}

	// Create hint generator
	gen, err := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)
	if err != nil {
		t.Fatalf("Failed to create hint generator: %v", err)
	}

	// Generate hints
	hintInterfaces, err := gen.Generate(context.Background(), testElements)
	if err != nil {
		t.Fatalf("Failed to generate hints: %v", err)
	}

	// Create hint collection
	hints := hint.NewCollection(hintInterfaces)

	// Set hints in manager
	err = hintManager.SetHints(hints)
	if err != nil {
		t.Fatalf("SetHints: %v", err)
	}

	t.Run("Hint manager and router integration", func(t *testing.T) {
		// Router no longer handles mode exit keys.
		result, err := hintRouter.RouteKey("escape")
		if err != nil {
			t.Fatalf("RouteKey: %v", err)
		}

		if result.ExactHint() != nil {
			t.Error("Expected no exact match on escape in hint router")
		}
	})

	t.Run("Hint manager callback integration", func(t *testing.T) {
		var callbackCalled bool

		// Set callback
		hintManager.SetUpdateCallback(func(hints []*hint.Interface) {
			callbackCalled = true
		})

		// Reset should trigger callback
		err := hintManager.Reset()
		if err != nil {
			t.Fatalf("Reset: %v", err)
		}

		if !callbackCalled {
			t.Error("Expected callback to be called on reset")
		}
	})
}

func TestCollection_Empty(t *testing.T) {
	// Empty collection
	empty := hint.NewCollection([]*hint.Interface{})
	if !empty.Empty() {
		t.Error("Empty collection should return true for Empty()")
	}

	// Non-empty collection
	elem, _ := element.NewElement(element.ID("1"), image.Rect(0, 0, 10, 10), element.RoleButton)
	h, _ := hint.NewHint("A", elem, image.Point{0, 0})

	nonEmpty := hint.NewCollection([]*hint.Interface{h})
	if nonEmpty.Empty() {
		t.Error("Non-empty collection should return false for Empty()")
	}
}

func TestManager_ImmediateUpdateRequiresExternalMu(t *testing.T) {
	// When externalMu is set, HandleInput's immediate-update path must be
	// called while the caller holds externalMu.
	var mut sync.Mutex

	elem, _ := element.NewElement(element.ID("1"), image.Rect(0, 0, 10, 10), element.RoleButton)
	h1, _ := hint.NewHint("AA", elem, image.Point{0, 0})
	h2, _ := hint.NewHint("AB", elem, image.Point{0, 0})
	collection := hint.NewCollection([]*hint.Interface{h1, h2})
	manager := hint.NewManager(logger.Get(), &mut)
	manager.SetUpdateCallback(func(_ []*hint.Interface) {})
	mut.Lock()

	err := manager.SetHints(collection)
	if err != nil {
		t.Fatalf("SetHints: %v", err)
	}
	mut.Unlock()

	// "A" narrows to 2 hints (AA, AB) — same count as the full set (2),
	// so this triggers immediateUpdate which should fail without the lock.
	_, _, err = manager.HandleInput("A")
	if !errors.Is(err, hint.ErrExternalMuNotHeld) {
		t.Fatalf("HandleInput without externalMu: got %v, want %v", err, hint.ErrExternalMuNotHeld)
	}
}

func TestManager_SetHintsRequiresExternalMu(t *testing.T) {
	var mut sync.Mutex

	elem, _ := element.NewElement(element.ID("1"), image.Rect(0, 0, 10, 10), element.RoleButton)
	h1, _ := hint.NewHint("AA", elem, image.Point{0, 0})
	collection := hint.NewCollection([]*hint.Interface{h1})
	manager := hint.NewManager(logger.Get(), &mut)
	manager.SetUpdateCallback(func(_ []*hint.Interface) {})

	err := manager.SetHints(collection)
	if !errors.Is(err, hint.ErrExternalMuNotHeld) {
		t.Fatalf("SetHints without externalMu: got %v, want %v", err, hint.ErrExternalMuNotHeld)
	}
}

func TestManager_ResetRequiresExternalMu(t *testing.T) {
	var mut sync.Mutex

	elem, _ := element.NewElement(element.ID("1"), image.Rect(0, 0, 10, 10), element.RoleButton)
	h1, _ := hint.NewHint("AA", elem, image.Point{0, 0})
	collection := hint.NewCollection([]*hint.Interface{h1})
	manager := hint.NewManager(logger.Get(), &mut)
	manager.SetUpdateCallback(func(_ []*hint.Interface) {})
	mut.Lock()

	err := manager.SetHints(collection)
	if err != nil {
		t.Fatalf("SetHints: %v", err)
	}
	mut.Unlock()

	err = manager.Reset()
	if !errors.Is(err, hint.ErrExternalMuNotHeld) {
		t.Fatalf("Reset without externalMu: got %v, want %v", err, hint.ErrExternalMuNotHeld)
	}
}

func TestManager_ImmediateUpdateSucceedsWithExternalMu(t *testing.T) {
	// Mirror the production call pattern: hold externalMu, then call
	// HandleInput. The immediate-update path should succeed without panic.
	var mut sync.Mutex

	elem, _ := element.NewElement(element.ID("1"), image.Rect(0, 0, 10, 10), element.RoleButton)
	h1, _ := hint.NewHint("AA", elem, image.Point{0, 0})
	h2, _ := hint.NewHint("AB", elem, image.Point{0, 0})
	collection := hint.NewCollection([]*hint.Interface{h1, h2})
	manager := hint.NewManager(logger.Get(), &mut)

	var callbackCalled bool
	manager.SetUpdateCallback(func(_ []*hint.Interface) {
		callbackCalled = true
	})
	mut.Lock()

	err := manager.SetHints(collection)
	if err != nil {
		t.Fatalf("SetHints: %v", err)
	}
	// "A" narrows to 2 hints (AA, AB) — same count → immediateUpdate.
	_, _, err = manager.HandleInput("A")
	if err != nil {
		t.Fatalf("HandleInput: %v", err)
	}
	mut.Unlock()

	if !callbackCalled {
		t.Error("Expected callback to be called via immediateUpdate")
	}
}

func TestManager_NoMatchRepeatedUsesImmediateUpdate(t *testing.T) {
	// When the user types an invalid key that resets to the full set, and
	// then types another invalid key (still the full set), the count
	// doesn't change so the no-match path should use immediateUpdate
	// (synchronous callback) rather than debouncedUpdate.
	var mut sync.Mutex

	elem, _ := element.NewElement(element.ID("1"), image.Rect(0, 0, 10, 10), element.RoleButton)
	h1, _ := hint.NewHint("AA", elem, image.Point{0, 0})
	h2, _ := hint.NewHint("AB", elem, image.Point{0, 0})
	collection := hint.NewCollection([]*hint.Interface{h1, h2})
	manager := hint.NewManager(logger.Get(), &mut)
	callCount := 0
	manager.SetUpdateCallback(func(_ []*hint.Interface) {
		callCount++
	})
	mut.Lock()

	err := manager.SetHints(collection)
	if err != nil {
		t.Fatalf("SetHints: %v", err)
	}
	// First invalid key "X" → no match, resets to full set (count 2).
	// Previous count was 2 (from SetHints), so same count → immediateUpdate.
	_, _, err = manager.HandleInput("X")
	if err != nil {
		t.Fatalf("HandleInput: %v", err)
	}

	if callCount != 2 { // 1 from SetHints + 1 from HandleInput("X")
		t.Errorf("Expected 2 callback calls after first invalid key, got %d", callCount)
	}
	// Second invalid key "Z" → no match again, full set (count 2).
	// Previous count was 2, same count → immediateUpdate (synchronous).
	_, _, err = manager.HandleInput("Z")
	if err != nil {
		t.Fatalf("HandleInput: %v", err)
	}

	if callCount != 3 { // +1 synchronous callback
		t.Errorf("Expected 3 callback calls after second invalid key, got %d", callCount)
	}
	mut.Unlock()
}

func TestManager_AcceptsNonLetterCharacters(t *testing.T) {
	logger := logger.Get()
	hintManager := hint.NewManager(logger, nil)

	// Create test elements
	elem1, _ := element.NewElement("elem1", image.Rect(10, 10, 50, 50), element.RoleButton)
	elem2, _ := element.NewElement("elem2", image.Rect(60, 10, 100, 50), element.RoleButton)
	elem3, _ := element.NewElement("elem3", image.Rect(10, 60, 50, 100), element.RoleButton)
	testElements := []*element.Element{elem1, elem2, elem3}

	// Create hint generator with numbers and symbols
	gen, err := hint.NewAlphabetGenerator("a1!", hint.LabelDirectionReverse)
	if err != nil {
		t.Fatalf("Failed to create hint generator: %v", err)
	}

	// Generate hints
	hintInterfaces, err := gen.Generate(context.Background(), testElements)
	if err != nil {
		t.Fatalf("Failed to generate hints: %v", err)
	}

	// Set hints
	collection := hint.NewCollection(hintInterfaces)

	err = hintManager.SetHints(collection)
	if err != nil {
		t.Fatalf("SetHints: %v", err)
	}

	// Test that letters are accepted and complete for single-char hints
	matchedHint, complete, err := hintManager.HandleInput("a")
	if err != nil {
		t.Fatalf("HandleInput: %v", err)
	}

	if !complete {
		t.Error("Expected complete after single letter matching hint")
	}

	if matchedHint == nil {
		t.Error("Expected hint to be returned")
	}

	err = hintManager.Reset()
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}

	// Test that numbers are accepted and complete for single-char hints
	matchedHint2, complete2, err := hintManager.HandleInput("1")
	if err != nil {
		t.Fatalf("HandleInput: %v", err)
	}

	if !complete2 {
		t.Error("Expected complete after single number matching hint")
	}

	if matchedHint2 == nil {
		t.Error("Expected hint to be returned")
	}

	err = hintManager.Reset()
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}

	// Test that symbols are accepted and complete for single-char hints
	matchedHint3, complete3, err := hintManager.HandleInput("!")
	if err != nil {
		t.Fatalf("HandleInput: %v", err)
	}

	if !complete3 {
		t.Error("Expected complete after single symbol matching hint")
	}

	if matchedHint3 == nil {
		t.Error("Expected hint to be returned")
	}

	err = hintManager.Reset()
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}

	// Note: Unicode characters like é and emoji are rejected at config validation level
	// so they won't be present in hint_characters, making this test unnecessary
}
