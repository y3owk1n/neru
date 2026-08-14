package sequence

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// CommandHandler dispatches one action step. It is the IPC controller's
// HandleCommand, named here as an interface so the executor depends on the one
// method it calls rather than on the controller.
type CommandHandler interface {
	HandleCommand(ctx context.Context, cmd ipc.Command) ipc.Response
}

// ExecutorDeps collects what running a sequence needs.
//
// Every field is a function or a one-method interface rather than a component,
// which is what keeps this package out of the application's dependency graph:
// the executor drives modes and IPC without importing either.
type ExecutorDeps struct {
	// Commands dispatches a step that is not a shell command. A nil handler
	// makes every such step fail rather than silently succeed.
	Commands CommandHandler

	// Config reads the live configuration. It is a function because a sequence
	// must see a reload that lands between its steps, the same way a scroll
	// step picks up a step size changed mid-sequence.
	Config func() *config.Config

	// SuppressModifiers is called with the triggering source before a step
	// activates a mode, so modifiers still physically held from a hotkey do not
	// leak into that mode. Nil disables the behavior, which is correct for
	// sequences that no hotkey triggered.
	SuppressModifiers func(source string)

	// BaseContext returns the context steps run under — the daemon's, so a
	// blocking step is released at shutdown rather than at the caller's whim.
	// It is a function so the executor can be built before the context exists.
	BaseContext func() context.Context

	Logger *zap.Logger
}

// Executor runs action sequences.
//
// This is the only place sequencing is implemented. Global hotkeys, per-mode
// hotkeys, held-key repeat, a mode's --on-exit, and the "run" command all
// funnel through it, so a sequence behaves the same wherever it is written.
type Executor struct {
	commands          CommandHandler
	config            func() *config.Config
	suppressModifiers func(source string)
	baseContext       func() context.Context
	logger            *zap.Logger
}

// NewExecutor creates a sequence executor. A nil logger is replaced with a
// no-op, matching the rest of the codebase.
func NewExecutor(deps ExecutorDeps) *Executor {
	logger := deps.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Executor{
		commands:          deps.Commands,
		config:            deps.Config,
		suppressModifiers: deps.SuppressModifiers,
		baseContext:       deps.BaseContext,
		logger:            logger.Named("app.sequence"),
	}
}

// Run executes steps in order under the default policy.
//
// A step that reports CodeChainBail stops the sequence. Any other failure is
// reported and the remaining steps still run, unless the step was marked
// fatal — with the trailing --bail-on-error directive, or by a policy that
// marks every step — in which case the sequence stops there.
//
// Steps execute against the base context rather than against ctx, so a blocking
// step (wait_for_mode_exit) is still released at shutdown. The nesting depth is
// the only thing carried over from ctx.
func (e *Executor) Run(ctx context.Context, source string, steps []string) Outcome {
	return e.RunWithPolicy(ctx, source, steps, Policy{})
}

// RunWithPolicy is Run with an explicit failure policy, for callers that set
// one for the whole sequence.
func (e *Executor) RunWithPolicy(
	ctx context.Context,
	source string,
	steps []string,
	policy Policy,
) Outcome {
	var outcome Outcome

	depth := Depth(ctx)
	if depth >= MaxDepth {
		// Nothing ran, so there is no step to point at: Executed and
		// FailedIndex stay zero and reporting says the sequence never started.
		outcome.Stopped = true
		outcome.Err = derrors.Newf(
			derrors.CodeInvalidInput,
			"action sequence nested deeper than %d levels",
			MaxDepth,
		)

		e.logger.Error("Action sequence nested too deeply",
			zap.String("source", source),
			zap.Int("depth", depth))

		return outcome
	}

	stepCtx := e.stepContext(depth)

	for _, step := range steps {
		trimmedStep := strings.TrimSpace(step)
		if trimmedStep == "" {
			continue
		}

		dispatchStep, stepIsFatal, directiveErr := SplitBailOnError(trimmedStep)
		stepIsFatal = stepIsFatal || policy.StopOnError

		outcome.Executed++

		stepErr := directiveErr
		if stepErr == nil {
			stepErr = e.step(stepCtx, source, dispatchStep)
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
				e.logger.Debug("Action sequence bailed",
					zap.String("source", source),
					zap.Int("step", outcome.Executed))
			} else {
				e.logger.Error("Action sequence stopped at a failed step",
					zap.String("source", source),
					zap.String("action", trimmedStep),
					zap.Error(stepErr))
			}

			return outcome
		}

		e.logger.Error("Action sequence step failed",
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

// RunAndForget executes a sequence and discards the outcome. It is the entry
// point for callers that have nobody to report to — hotkeys, held-key repeat,
// and a mode's --on-exit — all of which rely on the logging Run already does.
func (e *Executor) RunAndForget(source string, steps []string) {
	e.Run(e.base(), source, steps)
}

// RunMacro runs the named macro's steps as a nested sequence.
//
// Running it nested rather than splicing its steps into the caller keeps two
// things honest: the depth guard sees the nesting, so a macro that invokes
// itself is stopped like any other runaway sequence; and the caller reports
// failures against the step the author actually wrote ("macro foo 1 2") rather
// than against an expanded position they never saw.
func (e *Executor) RunMacro(ctx context.Context, name string, args []string) error {
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
	body, defined := e.settings().Macros[name]
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
	outcome := e.RunWithPolicy(
		ctx,
		MacroSource(name),
		config.ExpandMacroSteps(body, args),
		Policy{},
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

// step runs one step of a sequence: a macro invocation expands to the sequence
// it names, anything else is dispatched as an action.
func (e *Executor) step(ctx context.Context, source, step string) error {
	name, args, isMacro := config.ParseMacroCall(step)
	if !isMacro {
		return e.action(ctx, source, step)
	}

	return e.RunMacro(ctx, name, args)
}

// action executes a single action step, which can be either a shell command or
// an IPC command. ctx carries the sequence's nesting depth into the IPC
// handlers, so a step that starts another sequence can be stopped before it
// recurses without bound.
func (e *Executor) action(ctx context.Context, source, actionStr string) error {
	actionStr = strings.TrimSpace(actionStr)

	if actionStr == action.PrefixExec || strings.HasPrefix(actionStr, action.PrefixExec+" ") {
		return e.shell(ctx, source, actionStr)
	}

	actionParts := config.SplitStepArgs(actionStr)
	actionStr = actionParts[0]
	params := actionParts[1:]

	if e.suppressModifiers != nil {
		switch actionStr {
		case domain.ModeString(domain.ModeHints),
			domain.ModeString(domain.ModeGrid),
			domain.ModeString(domain.ModeRecursiveGrid),
			domain.ModeString(domain.ModeScroll),
			domain.ModeString(domain.ModeMonitorSelect):
			e.suppressModifiers(source)
		}
	}

	if e.commands == nil {
		return derrors.New(derrors.CodeInternal, "no command handler")
	}

	ipcResponse := e.commands.HandleCommand(ctx, ipc.Command{Action: actionStr, Args: params})
	if !ipcResponse.Success {
		if ipcResponse.Code == ipc.CodeChainBail {
			return derrors.New(derrors.CodeChainBail, ipcResponse.Message)
		}

		return derrors.New(derrors.CodeIPCFailed, ipcResponse.Message)
	}

	e.logger.Debug(
		"action step executed",
		zap.String("source", source),
		zap.String("action", actionStr),
	)

	return nil
}

// shell executes an "exec ..." step through the configured shell.
func (e *Executor) shell(ctx context.Context, source, actionStr string) error {
	cmdString := strings.TrimSpace(strings.TrimPrefix(actionStr, action.PrefixExec))
	if cmdString == "" {
		e.logger.Error("exec step has empty command", zap.String("source", source))

		return derrors.New(derrors.CodeInvalidInput, "empty command")
	}

	e.logger.Debug(
		"Executing shell command from a sequence step",
		zap.String("source", source),
		zap.String("cmd", cmdString),
	)

	execCtx, cancel := context.WithTimeout(ctx, domain.ShellCommandTimeout)
	defer cancel()

	cfg := e.settings()
	shell := cfg.General.ExecShell
	shellArgs := cfg.General.ExecShellArgs

	args := make([]string, 0, len(shellArgs)+1)
	args = append(args, shellArgs...)
	args = append(args, cmdString)

	command := exec.CommandContext(execCtx, shell, args...) //nolint:gosec

	commandOutput, commandErr := command.CombinedOutput()
	if commandErr != nil {
		// The command string and its output are the two things this must not
		// write down: the command is config content, and the output is whatever
		// the user's own shell printed. Their sizes and the exit code say as
		// much about the failure as the log is entitled to know — matching the
		// success path below, and the caller still receives the wrapped error.
		e.logger.Error(
			"exec step failed",
			zap.String("source", source),
			zap.Int("cmd_length", len(cmdString)),
			zap.Int("output_bytes", len(commandOutput)),
			zap.Int("exit_code", exitCodeOf(commandErr)),
			zap.Error(commandErr),
		)

		return derrors.Wrap(commandErr, derrors.CodeInternal, "exec step failed")
	}

	e.logger.Debug(
		"exec step completed",
		zap.String("source", source),
		zap.Int("cmd_length", len(cmdString)),
		zap.Int("output_bytes", len(commandOutput)),
	)

	return nil
}

// exitCodeOf reports the exit status behind a failed command, or -1 when the
// command never ran far enough to have one (a missing shell, a timeout, a
// signal). It exists so the failure log can be specific about *how* the step
// failed without quoting anything the command said.
func exitCodeOf(commandErr error) int {
	var exitErr *exec.ExitError
	if errors.As(commandErr, &exitErr) {
		return exitErr.ExitCode()
	}

	return -1
}

// stepContext builds the context each step of a sequence runs under: the base
// context, so shutdown releases a step that blocks, carrying the next depth.
func (e *Executor) stepContext(depth int) context.Context {
	return WithDepth(e.base(), depth+1)
}

// base returns the daemon context, or a background one before it exists.
func (e *Executor) base() context.Context {
	if e.baseContext == nil {
		return context.Background()
	}

	if ctx := e.baseContext(); ctx != nil {
		return ctx
	}

	return context.Background()
}

// settings returns the live configuration, or an empty one if no reader was
// wired, so a missing dependency degrades instead of panicking.
func (e *Executor) settings() *config.Config {
	if e.config == nil {
		return &config.Config{}
	}

	if cfg := e.config(); cfg != nil {
		return cfg
	}

	return &config.Config{}
}
