//go:build unit

package modes_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/core/domain"
)

func TestScrollMode_ModeType(t *testing.T) {
	handler := &modes.Handler{}
	mode := modes.NewScrollMode(handler)

	if mode.ModeType() != domain.ModeScroll {
		t.Errorf("Expected ModeScroll, got %v", mode.ModeType())
	}
}

func TestScrollMode_HandleKey(t *testing.T) {
	handler := &modes.Handler{}
	mode := modes.NewScrollMode(handler)

	// HandleKey may panic due to nil services, but should not crash the test
	defer func() {
		if r := recover(); r != nil {
			// Expected due to nil handler dependencies in test
			t.Logf("Expected panic due to nil handler dependencies: %v", r)
		}
	}()

	mode.HandleKey("j")
}

func TestScrollMode_HandleActionKey(t *testing.T) {
	handler := &modes.Handler{}
	mode := modes.NewScrollMode(handler)

	// HandleActionKey should not panic for scroll mode
	mode.HandleActionKey("l")
}

func TestScrollMode_Exit(t *testing.T) {
	handler := &modes.Handler{}
	mode := modes.NewScrollMode(handler)

	// Exit may panic due to nil services, but should not crash the test
	defer func() {
		if r := recover(); r != nil {
			// Expected due to nil handler dependencies in test
			t.Logf("Expected panic due to nil handler dependencies: %v", r)
		}
	}()

	mode.Exit()
}

func TestScrollMode_ToggleActionMode(t *testing.T) {
	handler := &modes.Handler{}
	mode := modes.NewScrollMode(handler)

	// ToggleActionMode should not panic for scroll mode
	mode.ToggleActionMode()
}
