package axobserver

import (
	"testing"

	"go.uber.org/zap"
)

func TestChangeHandlerForwardsToOnChange(t *testing.T) {
	fired := 0
	handler := newChangeHandler(func() { fired++ }, zap.NewNop())

	handler("AXCreated")

	if fired != 1 {
		t.Fatalf("onChange fired %d times, want 1", fired)
	}
}

func TestChangeHandlerToleratesNilOnChange(t *testing.T) {
	handler := newChangeHandler(nil, zap.NewNop())

	handler("AXCreated")
}
