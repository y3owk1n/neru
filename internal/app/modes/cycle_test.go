package modes

import (
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	componenthints "github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// TestAdvanceCycles_StepsFromTheStrategyInUse pins the hints half of a cycle
// list, which no journey reaches because the harness has no capture strategy.
// The step starts from the strategy the open session scanned with, including
// one the configuration chose, and entering from another mode keeps the first
// entry.
func TestAdvanceCycles_StepsFromTheStrategyInUse(t *testing.T) {
	tests := []struct {
		name     string
		current  domain.Mode
		inUse    string
		wantNext string
	}{
		{"entering from idle keeps the first entry", domain.ModeIdle, "", domain.StrategyAXTree},
		{
			"open on the first entry steps to the second",
			domain.ModeHints,
			domain.StrategyAXTree,
			domain.StrategyVision,
		},
		{
			"open on the last entry wraps",
			domain.ModeHints,
			domain.StrategyVision,
			domain.StrategyAXTree,
		},
		{
			"open on a strategy outside the list starts it",
			domain.ModeHints,
			domain.StrategyContour,
			domain.StrategyAXTree,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			appState := state.NewAppState()
			appState.SetMode(testCase.current)

			hintContext := &componenthints.Context{}
			hintContext.SetActiveScan(testCase.inUse, domain.CaptureScopeWindow)

			handler := newHandlerWithState(handlerState{
				logger:   zap.NewNop(),
				appState: appState,
				hints:    &components.HintsComponent{Context: hintContext},
			})

			activation, err := modecmd.Parse(domain.ModeHints, []string{"--strategy=axtree,vision"})
			if err != nil {
				t.Fatalf("Parse error = %v", err)
			}

			handler.advanceCycles(&activation, NewHintsMode(&handler.handlerState))

			if got := *activation.Strategy; got != testCase.wantNext {
				t.Errorf("strategy = %q, want %q", got, testCase.wantNext)
			}
		})
	}
}
