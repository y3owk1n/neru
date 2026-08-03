package app

import (
	"context"

	"github.com/y3owk1n/neru/internal/app/hotkey"
	"github.com/y3owk1n/neru/internal/app/sequence"
)

// newSequenceExecutor builds the executor from the App's own components.
//
// Every dependency is handed over as a function or a one-method interface, so
// the executor never sees the App. That is what lets sequencing be tested — and
// eventually moved — without a daemon.
func (a *App) newSequenceExecutor() *sequence.Executor {
	// Handing over a typed nil would defeat the executor's own nil check: the
	// interface would be non-nil and the first step would dereference it.
	var commands sequence.CommandHandler
	if a.ipcController != nil {
		commands = a.ipcController
	}

	return sequence.NewExecutor(sequence.ExecutorDeps{
		Commands:    commands,
		Config:      a.configSnapshot,
		BaseContext: func() context.Context { return a.ctx },
		// A step that opens a mode has to clear the modifiers still physically
		// held from the hotkey that triggered it, or the new mode reads them as
		// deliberate. Only the App knows the key a source names.
		SuppressModifiers: func(source string) {
			if a.modes == nil {
				return
			}

			a.modes.SuppressModifiersUntilReleased(hotkey.ModifiersFromKey(source))
		},
		Logger: a.logger,
	})
}

// sequences returns the executor, building it if initialization has not.
// Construction is cheap and depends only on fields the App already holds.
func (a *App) sequences() *sequence.Executor {
	if a.sequenceExecutor != nil {
		return a.sequenceExecutor
	}

	return a.newSequenceExecutor()
}

// executeActionSequence runs steps in order and reports what happened.
func (a *App) executeActionSequence(
	ctx context.Context,
	source string,
	steps []string,
) sequence.Outcome {
	return a.sequences().Run(ctx, source, steps)
}

// executeActionSequenceWithPolicy is executeActionSequence with an explicit
// failure policy, for callers that set one for the whole sequence.
func (a *App) executeActionSequenceWithPolicy(
	ctx context.Context,
	source string,
	steps []string,
	policy sequence.Policy,
) sequence.Outcome {
	return a.sequences().RunWithPolicy(ctx, source, steps, policy)
}

// executeMacro runs the named macro's steps as a nested sequence.
func (a *App) executeMacro(ctx context.Context, name string, args []string) error {
	return a.sequences().RunMacro(ctx, name, args)
}

// runActionSequence executes a sequence and discards the outcome. It is the
// entry point for callers that have nobody to report to — hotkeys, held-key
// repeat, and a mode's --on-exit.
func (a *App) runActionSequence(source string, steps []string) {
	a.sequences().RunAndForget(source, steps)
}
