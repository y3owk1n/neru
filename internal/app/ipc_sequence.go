package app

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

// sequenceRunner executes an ordered list of action steps under a failure
// policy and reports what happened. It is the daemon's action-sequence
// executor, injected so the IPC layer does not reach back into the App.
type sequenceRunner func(
	ctx context.Context,
	source string,
	steps []string,
	policy sequencePolicy,
) sequenceOutcome

// macroRunner runs one named macro with its arguments and reports what
// happened. It is the same call the executor makes for a "macro" step, injected
// so the IPC layer does not reach back into the App.
//
// It takes the arguments already split rather than a step string because the
// CLI hands them over split, and reassembling them into one step only to split
// it again would put every argument through a round of quoting it does not need
// to survive.
type macroRunner func(ctx context.Context, name string, args []string) error

// stopOnErrorFlag makes every step of a run fatal, so a script does not have to
// repeat the per-step directive on each one.
const stopOnErrorFlag = "--stop-on-error"

// IPCControllerSequence handles the "run" command, which executes an action
// sequence on behalf of an external caller.
//
// Sequencing already backs every hotkey binding; this exposes the same
// executor over IPC so that external drivers (skhd, Hammerspoon, shell
// scripts) can compose steps in one call instead of paying a process spawn
// per step and losing bail handling in between.
type IPCControllerSequence struct {
	run      sequenceRunner
	runMacro macroRunner
	logger   *zap.Logger
}

// NewIPCControllerSequence creates a new sequence command handler. A nil runner
// is valid: the corresponding command then reports that it is unavailable,
// matching how the other handlers treat a missing dependency.
func NewIPCControllerSequence(
	run sequenceRunner,
	runMacro macroRunner,
	logger *zap.Logger,
) *IPCControllerSequence {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &IPCControllerSequence{run: run, runMacro: runMacro, logger: logger}
}

// RegisterHandlers registers the sequence command handlers.
func (h *IPCControllerSequence) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers[domain.CommandRun] = h.handleRun
	handlers[domain.CommandMacro] = h.handleMacro
}

// handleMacro runs the macro named by the command's first argument, passing the
// rest as its arguments.
//
// A macro is already reachable as a step ("neru run 'macro zoom 3'"), but only
// by writing the call as one string. Going through this command instead keeps
// the arguments as the caller passed them, so an argument containing spaces or
// quotes does not have to survive being quoted into a step and split back out.
func (h *IPCControllerSequence) handleMacro(ctx context.Context, cmd ipc.Command) ipc.Response {
	if len(cmd.Args) == 0 || strings.TrimSpace(cmd.Args[0]) == "" {
		return ipc.Response{
			Success: false,
			Message: "macro requires a name (e.g., neru macro window_click)",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if h.runMacro == nil {
		return ipc.Response{
			Success: false,
			Message: "macros are not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	// Only the name is trimmed. The arguments are the macro's data rather than
	// steps, so they are passed on exactly as given: trimming them would
	// silently rewrite an argument whose padding matters, and dropping the
	// blank ones would shift every later argument onto the wrong placeholder.
	name, macroArgs := strings.TrimSpace(cmd.Args[0]), cmd.Args[1:]

	h.logger.Debug("Running macro", zap.Int("args", len(macroArgs)))

	macroErr := h.runMacro(ctx, name, macroArgs)
	if macroErr == nil {
		return ipc.Response{
			Success: true,
			Message: fmt.Sprintf("ran macro %q", name),
			Code:    ipc.CodeOK,
		}
	}

	return ipc.Response{
		Success: false,
		Message: macroErr.Error(),
		Code:    macroFailureCode(macroErr),
	}
}

// macroFailureCode maps a macro failure onto the IPC code that describes it.
//
// The distinctions matter to a caller: a bail is a canceled mode rather than a
// fault, and an invalid call (no such macro, wrong number of arguments) is
// worth telling apart from a step that ran and failed.
func macroFailureCode(err error) string {
	switch {
	case derrors.IsCode(err, derrors.CodeChainBail):
		return ipc.CodeChainBail
	case derrors.IsCode(err, derrors.CodeInvalidInput):
		return ipc.CodeInvalidInput
	default:
		return ipc.CodeActionFailed
	}
}

// handleRun executes the steps carried by the command, in order.
func (h *IPCControllerSequence) handleRun(ctx context.Context, cmd ipc.Command) ipc.Response {
	args, policy := splitSequencePolicy(cmd.Args)

	steps := nonBlankSteps(args)
	if len(steps) == 0 {
		return ipc.Response{
			Success: false,
			Message: "run requires at least one action step (e.g., neru run 'action left_click' hints)",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if h.run == nil {
		return ipc.Response{
			Success: false,
			Message: "action sequences are not available",
			Code:    ipc.CodeActionFailed,
		}
	}

	h.logger.Debug("Running action sequence", zap.Int("steps", len(steps)))

	outcome := h.run(ctx, domain.CommandRun, steps, policy)

	switch {
	case outcome.failedIndex == 0 && outcome.err != nil:
		// The sequence was refused before any step ran, so there is no step to
		// name — nesting too deeply is the only way to get here today.
		return ipc.Response{
			Success: false,
			Message: "sequence did not run: " + outcome.err.Error(),
			Code:    ipc.CodeInvalidInput,
		}
	case outcome.bailed:
		return ipc.Response{
			Success: false,
			Message: fmt.Sprintf(
				"sequence stopped at step %d (%s): %s",
				outcome.failedIndex,
				outcome.failedStep,
				outcome.err,
			),
			Code: ipc.CodeChainBail,
		}
	case outcome.stopped:
		// Distinct from the case below: the steps after this one did not run,
		// which a caller deciding what to clean up needs to know.
		return ipc.Response{
			Success: false,
			Message: fmt.Sprintf(
				"sequence stopped at step %d (%s): %s",
				outcome.failedIndex,
				outcome.failedStep,
				outcome.err,
			),
			Code: ipc.CodeActionFailed,
		}
	case outcome.err != nil:
		// Only claim the sequence carried on when there was something after
		// the failing step to carry on to.
		tail := ""
		if outcome.executed > outcome.failedIndex {
			tail = ", later steps still ran"
		}

		return ipc.Response{
			Success: false,
			Message: fmt.Sprintf(
				"step %d (%s) failed%s: %s",
				outcome.failedIndex,
				outcome.failedStep,
				tail,
				outcome.err,
			),
			Code: ipc.CodeActionFailed,
		}
	default:
		return ipc.Response{
			Success: true,
			Message: fmt.Sprintf("ran %d step(s)", outcome.executed),
			Code:    ipc.CodeOK,
		}
	}
}

// splitSequencePolicy separates the sequence-wide policy flags from the steps.
// The flags are consumed here rather than passed on, so a step never sees them.
func splitSequencePolicy(args []string) ([]string, sequencePolicy) {
	var policy sequencePolicy

	remaining := make([]string, 0, len(args))

	for _, arg := range args {
		if strings.TrimSpace(arg) == stopOnErrorFlag {
			policy.stopOnError = true

			continue
		}

		remaining = append(remaining, arg)
	}

	return remaining, policy
}

// nonBlankSteps trims each step and drops the empty ones, so that a stray
// separator in a binding does not count as a step.
func nonBlankSteps(args []string) []string {
	steps := make([]string, 0, len(args))

	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" {
			continue
		}

		steps = append(steps, trimmed)
	}

	return steps
}
