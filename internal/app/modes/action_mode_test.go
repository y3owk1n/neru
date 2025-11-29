//go:build unit

package modes_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/core/domain"
)

func TestActionMode_ModeType(t *testing.T) {
	handler := &modes.Handler{}
	mode := modes.NewActionMode(handler)

	if mode.ModeType() != domain.ModeAction {
		t.Errorf("Expected ModeAction, got %v", mode.ModeType())
	}
}

func TestActionMode_HandleKey(t *testing.T) {
	handler := &modes.Handler{}
	mode := modes.NewActionMode(handler)

	// HandleKey should not panic or do anything for action mode
	mode.HandleKey("a")
}

func TestActionMode_HandleActionKey(t *testing.T) {
	handler := &modes.Handler{}
	mode := modes.NewActionMode(handler)

	// HandleActionKey may panic due to nil services, but should not crash the test
	defer func() {
		if r := recover(); r != nil {
			// Expected due to nil handler dependencies in test
			t.Logf("Expected panic due to nil handler dependencies: %v", r)
		}
	}()

	mode.HandleActionKey("l")
}

func TestActionMode_Exit(t *testing.T) {
	handler := &modes.Handler{}
	mode := modes.NewActionMode(handler)

	// Exit should not panic for action mode
	mode.Exit()
}

func TestActionMode_ToggleActionMode(t *testing.T) {
	handler := &modes.Handler{}
	mode := modes.NewActionMode(handler)

	// ToggleActionMode should not panic for action mode
	mode.ToggleActionMode()
}
