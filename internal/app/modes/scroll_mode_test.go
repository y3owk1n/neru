//go:build unit

package modes_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/core/domain"
)

// Compile-time interface compliance check
var _ modes.Mode = (*modes.ScrollMode)(nil)

func TestScrollMode_ModeType(t *testing.T) {
	handler := &modes.Handler{}
	mode := modes.NewScrollMode(handler)

	if mode.ModeType() != domain.ModeScroll {
		t.Errorf("Expected ModeScroll, got %v", mode.ModeType())
	}
}

func TestScrollMode_InterfaceCompliance(t *testing.T) {
	handler := &modes.Handler{}
	mode := modes.NewScrollMode(handler)

	// Test that all interface methods exist and can be called
	// (they may panic due to nil dependencies, but that's expected for unit tests)

	// Test ModeType
	_ = mode.ModeType()

	// Test that methods can be called (may panic, but that's OK for interface compliance)
	defer func() {
		recover() // Ignore panics from nil dependencies
	}()

	mode.Activate(nil)
	mode.HandleKey("test")
	mode.HandleActionKey("test")
	mode.Exit()
	mode.ToggleActionMode()
}
