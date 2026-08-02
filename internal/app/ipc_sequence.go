package app

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

// sequenceRunner executes an ordered list of action steps and reports what
// happened. It is the daemon's action-sequence executor, injected so the IPC
// layer does not reach back into the App.
type sequenceRunner func(ctx context.Context, source string, steps []string) sequenceOutcome

// IPCControllerSequence handles the "run" command, which executes an action
// sequence on behalf of an external caller.
//
// Sequencing already backs every hotkey binding; this exposes the same
// executor over IPC so that external drivers (skhd, Hammerspoon, shell
// scripts) can compose steps in one call instead of paying a process spawn
// per step and losing bail handling in between.
type IPCControllerSequence struct {
	run    sequenceRunner
	logger *zap.Logger
}

// NewIPCControllerSequence creates a new sequence command handler. A nil runner
// is valid: the command then reports that sequencing is unavailable, matching
// how the other handlers treat a missing dependency.
func NewIPCControllerSequence(run sequenceRunner, logger *zap.Logger) *IPCControllerSequence {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &IPCControllerSequence{run: run, logger: logger}
}

// RegisterHandlers registers the sequence command handlers.
func (h *IPCControllerSequence) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers[domain.CommandRun] = h.handleRun
}

// handleRun executes the steps carried by the command, in order.
func (h *IPCControllerSequence) handleRun(ctx context.Context, cmd ipc.Command) ipc.Response {
	steps := nonBlankSteps(cmd.Args)
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

	outcome := h.run(ctx, domain.CommandRun, steps)

	switch {
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
	case outcome.err != nil:
		return ipc.Response{
			Success: false,
			Message: fmt.Sprintf(
				"step %d (%s) failed: %s",
				outcome.failedIndex,
				outcome.failedStep,
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
