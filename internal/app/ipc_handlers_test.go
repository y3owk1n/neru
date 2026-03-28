package app

import (
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

func TestExtractModeOptions_InvalidCursorSelectionModeEqualsValue(t *testing.T) {
	controller := &IPCControllerModes{
		logger: zap.NewNop(),
	}

	_, resp := controller.extractModeOptions(ipc.Command{
		Action: "grid",
		Args:   []string{"grid", "--cursor-selection-mode=invalid"},
	})

	if resp == nil {
		t.Fatal("extractModeOptions() expected error response")
	}

	if resp.Message != "--cursor-selection-mode requires follow or hold" {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}
