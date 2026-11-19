package hint_test

import (
	"context"
	"errors"
	"image"
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/hint"
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

				match, found, _, err = manager.HandleInput(string(char))
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

	err = inputErr(manager.HandleInput("A"))
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

	// "A" matches both hints (AA, AB), and the redraw it triggers should fail
	// without the lock.
	err = inputErr(manager.HandleInput("A"))
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

func TestManager_NoMatchRepeatedUsesImmediateUpdate(t *testing.T) {
	// When the user types an invalid key that resets to the full set, and
	// then types another invalid key (still the full set), each reset
	// redraws synchronously inside HandleInput.
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
	// First invalid key "X" has no match and resets to the full set (count 2).
	err = inputErr(manager.HandleInput("X"))
	if err != nil {
		t.Fatalf("HandleInput: %v", err)
	}

	if callCount != 2 { // 1 from SetHints + 1 from HandleInput("X")
		t.Errorf("Expected 2 callback calls after first invalid key, got %d", callCount)
	}
	// Second invalid key "Z" has no match either, so the set stays full (count 2).
	err = inputErr(manager.HandleInput("Z"))
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

	elem1, _ := element.NewElement("elem1", image.Rect(10, 10, 50, 50), element.RoleButton)
	elem2, _ := element.NewElement("elem2", image.Rect(60, 10, 100, 50), element.RoleButton)
	elem3, _ := element.NewElement("elem3", image.Rect(10, 60, 50, 100), element.RoleButton)
	testElements := []*element.Element{elem1, elem2, elem3}

	// An alphabet that mixes a letter, a number and a symbol.
	gen, err := hint.NewAlphabetGenerator("a1!", hint.LabelDirectionReverse)
	if err != nil {
		t.Fatalf("Failed to create hint generator: %v", err)
	}

	hintInterfaces, err := gen.Generate(context.Background(), testElements)
	if err != nil {
		t.Fatalf("Failed to generate hints: %v", err)
	}

	collection := hint.NewCollection(hintInterfaces)

	err = hintManager.SetHints(collection)
	if err != nil {
		t.Fatalf("SetHints: %v", err)
	}

	// A letter is accepted and completes a single-char hint.
	matchedHint, complete, _, err := hintManager.HandleInput("a")
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

	// A number is accepted and completes a single-char hint.
	matchedHint2, complete2, _, err := hintManager.HandleInput("1")
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

	// A symbol is accepted and completes a single-char hint.
	matchedHint3, complete3, _, err := hintManager.HandleInput("!")
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

	// Unicode characters like é and emoji are rejected by config validation,
	// so they never reach hint_characters and need no case here.
}

// inputErr keeps only the error from HandleInput, without a `_, _, _ =`
// assignment (which trips dogsled).
func inputErr(_ *hint.Interface, _, _ bool, err error) error {
	return err
}
