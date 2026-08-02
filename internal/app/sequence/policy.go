package sequence

import (
	"slices"
	"strings"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// BailOnErrorFlag marks a step fatal: a failure there stops the sequence
// instead of letting the remaining steps run. It is a sequencing directive
// rather than an action flag — the executor consumes it, and the step is
// dispatched without it.
const BailOnErrorFlag = "--bail-on-error"

// Outcome reports what an action sequence did. Callers that answer a client
// (the "run" command) turn it into a response; callers that answer nobody
// (hotkeys) ignore it and rely on the executor's logging.
type Outcome struct {
	// Err is the error that stopped the sequence when it stopped early,
	// otherwise the first step error.
	Err error
	// FailedStep is the step text that produced Err.
	FailedStep string
	// FailedIndex is the 1-based position of FailedStep among the steps that
	// ran. Reporting needs it separately from Executed, because a failure that
	// does not stop the sequence still leaves later steps to run.
	FailedIndex int
	// Executed counts how far into the sequence execution reached, including a
	// step that failed, bailed, or was rejected before it could run.
	Executed int
	// Bailed reports whether a step asked the sequence to stop by reporting
	// CodeChainBail — a canceled mode rather than a fault.
	Bailed bool
	// Stopped reports whether the sequence ended before its last step, whether
	// through a bail or a failure the step marked as fatal.
	Stopped bool
}

// Policy describes how a sequence treats a failing step.
//
// A policy applies to the steps of one sequence and does not cross into a
// nested one: a step that is itself a "run" carries whatever policy that run
// was given. The nested sequence's failure is still reported to the outer one,
// so an outer stop-on-error still stops there.
type Policy struct {
	// StopOnError makes every step fatal, as if each carried the per-step
	// directive. It is what "run --stop-on-error" applies.
	StopOnError bool
}

// SplitBailOnError separates the trailing bail-on-error directive from a step.
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
func SplitBailOnError(step string) (string, bool, error) {
	step = strings.TrimSpace(step)

	// Almost no step mentions the directive, and this runs for every step of
	// every sequence, including on the key-press path. Reject the common case
	// before tokenizing.
	if !strings.Contains(step, BailOnErrorFlag) {
		return step, false, nil
	}

	tokens := config.SplitStepArgs(step)
	if len(tokens) < 2 { //nolint:mnd // a lone directive is not a step.
		return step, false, nil
	}

	if tokens[len(tokens)-1] == BailOnErrorFlag {
		// A quoted final token leaves the step not ending in the bare text.
		// The author wrote it as an argument, so hand it over as one.
		if !strings.HasSuffix(step, BailOnErrorFlag) {
			return step, false, nil
		}

		return strings.TrimSpace(strings.TrimSuffix(step, BailOnErrorFlag)), true, nil
	}

	if slices.Contains(tokens, BailOnErrorFlag) {
		return step, false, derrors.Newf(
			derrors.CodeInvalidInput,
			"%s must come last in a step",
			BailOnErrorFlag,
		)
	}

	return step, false, nil
}

// MacroSource names a macro in logs the way it is written in the config.
func MacroSource(name string) string {
	return config.MacroCommand + " " + name
}
