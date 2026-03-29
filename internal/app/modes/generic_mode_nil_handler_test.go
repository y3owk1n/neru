package modes_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/core/domain"
)

func TestGenericMode_NilHandlerDoesNotPanic(t *testing.T) {
	mode := modes.NewGenericMode(nil, domain.ModeHints, "Hints", modes.ModeBehavior{})

	mode.Activate(modes.ModeActivationOptions{})
	mode.HandleKey("a")
	mode.Exit()
}
