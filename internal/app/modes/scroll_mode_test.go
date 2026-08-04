package modes

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
)

// Compile-time interface compliance check.
var _ Mode = (*ScrollMode)(nil)

func TestScrollMode_ModeType(t *testing.T) {
	handler := &handlerState{}
	mode := NewScrollMode(handler)

	if mode.ModeType() != domain.ModeScroll {
		t.Errorf("Expected ModeScroll, got %v", mode.ModeType())
	}
}

func TestScrollMode_InterfaceCompliance(t *testing.T) {
	handler := &handlerState{}
	mode := NewScrollMode(handler)

	if mode == nil {
		t.Fatal("Expected NewScrollMode to return a non-nil mode")
	}

	// Keep a runtime assertion in addition to the compile-time check above.
	var interfaceMode Mode = mode
	if interfaceMode.ModeType() != domain.ModeScroll {
		t.Errorf("Expected ModeScroll, got %v", interfaceMode.ModeType())
	}
}
