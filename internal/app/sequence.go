package app

import (
	"context"
	"strings"

	"go.uber.org/zap"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// maxSequenceDepth bounds how deeply one action sequence may invoke another.
// Sequences nest because a step can itself be a "run" command, so a binding
// that refers back to itself would otherwise recurse until the daemon runs out
// of stack.
const maxSequenceDepth = 5

// sequenceDepthKey types the context value that carries the current nesting
// depth. A struct key keeps the value private to this package.
type sequenceDepthKey struct{}

// sequenceDepth reports how many action sequences are already running above ctx.
func sequenceDepth(ctx context.Context) int {
	if ctx == nil {
		return 0
	}

	depth, ok := ctx.Value(sequenceDepthKey{}).(int)
	if !ok {
		return 0
	}

	return depth
}

// withSequenceDepth returns a context carrying the given nesting depth.
func withSequenceDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, sequenceDepthKey{}, depth)
}

// sequenceOutcome reports what an action sequence did. Callers that answer a
// client (the "run" command) turn it into a response; callers that answer
// nobody (hotkeys) ignore it and rely on the logging done here.
type sequenceOutcome struct {
	// err is the bail error when bailed is set, otherwise the first step error.
	err error
	// failedStep is the step text that produced err.
	failedStep string
	// failedIndex is the 1-based position of failedStep among the steps that
	// ran. Reporting needs it separately from executed, because a non-bailing
	// failure does not stop the sequence.
	failedIndex int
	// executed counts the steps that ran, including one that failed or bailed.
	executed int
	// bailed reports whether a step asked the sequence to stop early.
	bailed bool
}

// executeActionSequence runs steps in order through the hotkey action grammar
// ("action ...", "exec ...", a mode name, and so on).
//
// This is the only place sequencing is implemented. Global hotkeys, per-mode
// hotkeys, held-key repeat, a mode's --on-exit, and the "run" command all
// funnel through it, so a sequence behaves the same wherever it is written. A
// step that reports CodeChainBail stops the sequence; any other failure is
// reported and the remaining steps still run.
//
// Steps execute against the app context rather than against ctx, so a blocking
// step (wait_for_mode_exit) is still released at shutdown. The nesting depth is
// the only thing carried over from ctx.
func (a *App) executeActionSequence(
	ctx context.Context,
	source string,
	steps []string,
) sequenceOutcome {
	var outcome sequenceOutcome

	depth := sequenceDepth(ctx)
	if depth >= maxSequenceDepth {
		outcome.err = derrors.Newf(
			derrors.CodeInvalidInput,
			"action sequence nested deeper than %d levels",
			maxSequenceDepth,
		)

		a.logger.Error("Action sequence nested too deeply",
			zap.String("source", source),
			zap.Int("depth", depth))

		return outcome
	}

	stepCtx := a.sequenceStepContext(depth)

	for _, step := range steps {
		trimmedStep := strings.TrimSpace(step)
		if trimmedStep == "" {
			continue
		}

		outcome.executed++

		stepErr := a.executeHotkeyAction(stepCtx, source, trimmedStep)
		if stepErr == nil {
			continue
		}

		if derrors.IsCode(stepErr, derrors.CodeChainBail) {
			outcome.bailed = true
			outcome.err = stepErr
			outcome.failedStep = trimmedStep
			outcome.failedIndex = outcome.executed

			a.logger.Debug("Action sequence bailed",
				zap.String("source", source),
				zap.Int("step", outcome.executed))

			return outcome
		}

		a.logger.Error("Action sequence step failed",
			zap.String("source", source),
			zap.String("action", trimmedStep),
			zap.Error(stepErr))

		if outcome.err == nil {
			outcome.err = stepErr
			outcome.failedStep = trimmedStep
			outcome.failedIndex = outcome.executed
		}
	}

	return outcome
}

// sequenceStepContext builds the context each step of a sequence runs under:
// the app context, so shutdown releases a step that blocks, carrying the next
// nesting depth.
func (a *App) sequenceStepContext(depth int) context.Context {
	base := a.ctx
	if base == nil {
		base = context.Background()
	}

	return withSequenceDepth(base, depth+1)
}

// runActionSequence executes a sequence and discards the outcome. It is the
// entry point for callers that have nobody to report to — hotkeys, held-key
// repeat, and a mode's --on-exit — all of which rely on the logging that
// executeActionSequence already does.
func (a *App) runActionSequence(source string, steps []string) {
	a.executeActionSequence(a.ctx, source, steps)
}
