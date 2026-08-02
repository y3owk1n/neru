package app

import (
	"context"
	"slices"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
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

// bailOnErrorFlag marks a step whose failure ends the sequence. It is a
// sequencing directive rather than an action flag: the executor consumes it,
// and the step is dispatched without it.
const bailOnErrorFlag = "--bail-on-error"

// sequenceOutcome reports what an action sequence did. Callers that answer a
// client (the "run" command) turn it into a response; callers that answer
// nobody (hotkeys) ignore it and rely on the logging done here.
type sequenceOutcome struct {
	// err is the error that stopped the sequence when it stopped early,
	// otherwise the first step error.
	err error
	// failedStep is the step text that produced err.
	failedStep string
	// failedIndex is the 1-based position of failedStep among the steps that
	// ran. Reporting needs it separately from executed, because a failure that
	// does not stop the sequence still leaves later steps to run.
	failedIndex int
	// executed counts how far into the sequence execution reached, including a
	// step that failed, bailed, or was rejected before it could run.
	executed int
	// bailed reports whether a step asked the sequence to stop by reporting
	// CodeChainBail — a canceled mode rather than a fault.
	bailed bool
	// stopped reports whether the sequence ended before its last step, whether
	// through a bail or a failure the step marked as fatal.
	stopped bool
}

// sequencePolicy describes how a sequence treats a failing step.
//
// A policy applies to the steps of one sequence and does not cross into a
// nested one: a step that is itself a "run" carries whatever policy that run
// was given. The nested sequence's failure is still reported to the outer one,
// so an outer stop-on-error still stops there.
type sequencePolicy struct {
	// stopOnError makes every step fatal, as if each carried the per-step
	// directive. It is what "run --stop-on-error" applies.
	stopOnError bool
}

// splitBailOnError separates the trailing bail-on-error directive from a step.
//
// The directive is recognized only as the final unquoted token, so a step that
// merely carries the text as an argument keeps it: both
// `exec sh -c "echo --bail-on-error"` and `exec printf '--bail-on-error'` are
// passed through untouched.
//
// The directive anywhere else is an error rather than a silent pass-through.
// Left in place it would reach the action's own flag parser, which reports
// "invalid or missing flag value" without naming the flag — a much worse
// message than saying where the directive belongs.
func splitBailOnError(step string) (string, bool, error) {
	step = strings.TrimSpace(step)

	// Almost no step mentions the directive, and this runs for every step of
	// every sequence, including on the key-press path. Reject the common case
	// before tokenizing.
	if !strings.Contains(step, bailOnErrorFlag) {
		return step, false, nil
	}

	tokens := splitArgs(step)
	if len(tokens) < 2 { //nolint:mnd // a lone directive is not a step.
		return step, false, nil
	}

	if tokens[len(tokens)-1] == bailOnErrorFlag {
		// A quoted final token leaves the step not ending in the bare text.
		// The author wrote it as an argument, so hand it over as one.
		if !strings.HasSuffix(step, bailOnErrorFlag) {
			return step, false, nil
		}

		return strings.TrimSpace(strings.TrimSuffix(step, bailOnErrorFlag)), true, nil
	}

	if slices.Contains(tokens, bailOnErrorFlag) {
		return step, false, derrors.Newf(
			derrors.CodeInvalidInput,
			"%s must come last in a step",
			bailOnErrorFlag,
		)
	}

	return step, false, nil
}

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
) sequenceOutcome {
	return a.executeActionSequenceWithPolicy(ctx, source, steps, sequencePolicy{})
}

// executeActionSequenceWithPolicy is executeActionSequence with an explicit
// failure policy, for callers that set one for the whole sequence.
func (a *App) executeActionSequenceWithPolicy(
	ctx context.Context,
	source string,
	steps []string,
	policy sequencePolicy,
) sequenceOutcome {
	var outcome sequenceOutcome

	depth := sequenceDepth(ctx)
	if depth >= maxSequenceDepth {
		// Nothing ran, so there is no step to point at: executed and
		// failedIndex stay zero and reporting says the sequence never started.
		outcome.stopped = true
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

		dispatchStep, stepIsFatal, directiveErr := splitBailOnError(trimmedStep)
		stepIsFatal = stepIsFatal || policy.stopOnError

		outcome.executed++

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
			outcome.bailed = bailed
			outcome.stopped = true
			outcome.err = stepErr
			outcome.failedStep = trimmedStep
			outcome.failedIndex = outcome.executed

			if bailed {
				a.logger.Debug("Action sequence bailed",
					zap.String("source", source),
					zap.Int("step", outcome.executed))
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

		if outcome.err == nil {
			outcome.err = stepErr
			outcome.failedStep = trimmedStep
			outcome.failedIndex = outcome.executed
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
		macroSource(name),
		config.ExpandMacroSteps(body, args),
		sequencePolicy{},
	)

	if outcome.err == nil {
		return nil
	}

	// A bail inside a macro has to keep its meaning on the way out, or the
	// caller would treat a canceled mode as an ordinary failure.
	code := derrors.CodeActionFailed
	if outcome.bailed {
		code = derrors.CodeChainBail
	}

	return derrors.Wrapf(outcome.err, code, "macro %q", name)
}

// macroSource names a macro in logs the way it is written in the config.
func macroSource(name string) string {
	return config.MacroCommand + " " + name
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
