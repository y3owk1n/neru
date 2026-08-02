package app

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/sequence"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// executeActionSequence runs steps in order through the hotkey action grammar
// ("action ...", "exec ...", a mode name, and so on).
//
// This is the only place sequencing is implemented. Global hotkeys, per-mode
// hotkeys, held-key repeat, a mode's --on-exit, and the "run" command all
// funnel through it, so a sequence behaves the same wherever it is written.
//
// A step that reports CodeChainBail stops the sequence. Any other failure is
// reported and the remaining steps still run, unless the step was marked
// fatal — with the trailing --bail-on-error directive, or by a policy that
// marks every step — in which case the sequence stops there.
//
// Steps execute against the app context rather than against ctx, so a blocking
// step (wait_for_mode_exit) is still released at shutdown. The nesting depth is
// the only thing carried over from ctx.
func (a *App) executeActionSequence(
	ctx context.Context,
	source string,
	steps []string,
) sequence.Outcome {
	return a.executeActionSequenceWithPolicy(ctx, source, steps, sequence.Policy{})
}

// executeActionSequenceWithPolicy is executeActionSequence with an explicit
// failure policy, for callers that set one for the whole sequence.
func (a *App) executeActionSequenceWithPolicy(
	ctx context.Context,
	source string,
	steps []string,
	policy sequence.Policy,
) sequence.Outcome {
	var outcome sequence.Outcome

	depth := sequence.Depth(ctx)
	if depth >= sequence.MaxDepth {
		// Nothing ran, so there is no step to point at: executed and
		// failedIndex stay zero and reporting says the sequence never started.
		outcome.Stopped = true
		outcome.Err = derrors.Newf(
			derrors.CodeInvalidInput,
			"action sequence nested deeper than %d levels",
			sequence.MaxDepth,
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

		dispatchStep, stepIsFatal, directiveErr := sequence.SplitBailOnError(trimmedStep)
		stepIsFatal = stepIsFatal || policy.StopOnError

		outcome.Executed++

		stepErr := directiveErr
		if stepErr == nil {
			stepErr = a.executeStep(stepCtx, source, dispatchStep)
		}

		if stepErr == nil {
			continue
		}

		bailed := derrors.IsCode(stepErr, derrors.CodeChainBail)

		// A malformed directive is always fatal: the step never ran, and
		// carrying on would act on a sequence the author did not write.
		if bailed || stepIsFatal || directiveErr != nil {
			outcome.Bailed = bailed
			outcome.Stopped = true
			outcome.Err = stepErr
			outcome.FailedStep = trimmedStep
			outcome.FailedIndex = outcome.Executed

			if bailed {
				a.logger.Debug("Action sequence bailed",
					zap.String("source", source),
					zap.Int("step", outcome.Executed))
			} else {
				a.logger.Error("Action sequence stopped at a failed step",
					zap.String("source", source),
					zap.String("action", trimmedStep),
					zap.Error(stepErr))
			}

			return outcome
		}

		a.logger.Error("Action sequence step failed",
			zap.String("source", source),
			zap.String("action", trimmedStep),
			zap.Error(stepErr))

		if outcome.Err == nil {
			outcome.Err = stepErr
			outcome.FailedStep = trimmedStep
			outcome.FailedIndex = outcome.Executed
		}
	}

	return outcome
}

// executeStep runs one step of a sequence: a macro invocation expands to the
// sequence it names, anything else is dispatched as an action.
func (a *App) executeStep(ctx context.Context, source, step string) error {
	name, args, isMacro := config.ParseMacroCall(step)
	if !isMacro {
		return a.executeHotkeyAction(ctx, source, step)
	}

	return a.executeMacro(ctx, name, args)
}

// executeMacro runs the named macro's steps as a nested sequence.
//
// Running it nested rather than splicing its steps into the caller keeps two
// things honest: the depth guard sees the nesting, so a macro that invokes
// itself is stopped like any other runaway sequence; and the caller reports
// failures against the step the author actually wrote ("macro foo 1 2") rather
// than against an expanded position they never saw.
func (a *App) executeMacro(ctx context.Context, name string, args []string) error {
	if name == "" {
		return derrors.New(
			derrors.CodeInvalidInput,
			"macro requires a name (e.g. \"macro window_click 100 70\")",
		)
	}

	// The table is read when the call runs, not pinned for the whole sequence,
	// which is how every other step reads configuration — a scroll step takes
	// the step size current at the time it fires. A reload mid-sequence
	// therefore affects the calls after it, and a macro deleted by that reload
	// fails here rather than running a definition the config no longer has.
	// One body is read once and expanded once, so a single call is always
	// internally consistent.
	body, defined := a.configSnapshot().Macros[name]
	if !defined {
		return derrors.Newf(derrors.CodeInvalidInput, "no macro named %q", name)
	}

	if arity := config.MacroArity(body); len(args) != arity {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"macro %q takes %d argument(s), got %d",
			name,
			arity,
			len(args),
		)
	}

	// The nested sequence starts with its own policy: a macro decides for
	// itself which of its steps are fatal, and its overall failure is what the
	// caller sees.
	outcome := a.executeActionSequenceWithPolicy(
		ctx,
		sequence.MacroSource(name),
		config.ExpandMacroSteps(body, args),
		sequence.Policy{},
	)

	if outcome.Err == nil {
		return nil
	}

	// A bail inside a macro has to keep its meaning on the way out, or the
	// caller would treat a canceled mode as an ordinary failure.
	code := derrors.CodeActionFailed
	if outcome.Bailed {
		code = derrors.CodeChainBail
	}

	return derrors.Wrapf(outcome.Err, code, "macro %q", name)
}

// sequenceStepContext builds the context each step of a sequence runs under:
// the app context, so shutdown releases a step that blocks, carrying the next
// nesting depth.
func (a *App) sequenceStepContext(depth int) context.Context {
	base := a.ctx
	if base == nil {
		base = context.Background()
	}

	return sequence.WithDepth(base, depth+1)
}

// runActionSequence executes a sequence and discards the outcome. It is the
// entry point for callers that have nobody to report to — hotkeys, held-key
// repeat, and a mode's --on-exit — all of which rely on the logging that
// executeActionSequence already does.
func (a *App) runActionSequence(source string, steps []string) {
	a.executeActionSequence(a.ctx, source, steps)
}
