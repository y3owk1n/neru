package ipcctrl

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/sequence"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
)

// The steps these tests round-trip through the handler.
const (
	leftClickStep = "action left_click"
	hintsStep     = "hints --action left_click"
)

func TestHandleRun_RequiresSteps(t *testing.T) {
	t.Parallel()

	handler := NewSequenceHandler(
		func(context.Context, string, []string, sequence.Policy) sequence.Outcome {
			t.Fatal("the executor must not run for an empty sequence")

			return sequence.Outcome{}
		},
		nil,
		zap.NewNop(),
	)

	for _, args := range [][]string{nil, {}, {"  ", ""}} {
		resp := handler.handleRun(context.Background(), ipc.Command{Args: args})

		if resp.Success || resp.Code != ipc.CodeInvalidInput {
			t.Fatalf("handleRun(%v) = %+v, want an invalid-input failure", args, resp)
		}
	}
}

func TestHandleRun_ReportsMissingExecutor(t *testing.T) {
	t.Parallel()

	handler := NewSequenceHandler(nil, nil, zap.NewNop())

	resp := handler.handleRun(context.Background(), ipc.Command{Args: []string{"idle"}})

	if resp.Success || resp.Code != ipc.CodeActionFailed {
		t.Fatalf("handleRun() = %+v, want an action-failed response", resp)
	}
}

func TestHandleRun_PassesTrimmedStepsToExecutor(t *testing.T) {
	t.Parallel()

	var got []string

	handler := NewSequenceHandler(
		func(_ context.Context, _ string, steps []string, _ sequence.Policy) sequence.Outcome {
			got = slices.Clone(steps)

			return sequence.Outcome{Executed: len(steps)}
		},
		nil,
		zap.NewNop(),
	)

	resp := handler.handleRun(context.Background(), ipc.Command{
		Args: []string{" " + leftClickStep + " ", "", hintsStep},
	})

	if !resp.Success || resp.Code != ipc.CodeOK {
		t.Fatalf("handleRun() = %+v, want success", resp)
	}

	want := []string{leftClickStep, hintsStep}
	if !slices.Equal(got, want) {
		t.Fatalf("executor received %v, want %v", got, want)
	}
}

func TestHandleRun_ReportsBailAndFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		outcome  sequence.Outcome
		wantCode string
	}{
		{
			name: "bail",
			outcome: sequence.Outcome{
				Err:         derrors.New(derrors.CodeChainBail, "mode exited without selection"),
				FailedStep:  "action wait_for_mode_exit --bail",
				FailedIndex: 2,
				Executed:    2,
				Bailed:      true,
			},
			wantCode: ipc.CodeChainBail,
		},
		{
			name: "failed step",
			outcome: sequence.Outcome{
				Err:         derrors.New(derrors.CodeIPCFailed, "boom"),
				FailedStep:  leftClickStep,
				FailedIndex: 1,
				Executed:    3,
			},
			wantCode: ipc.CodeActionFailed,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			handler := NewSequenceHandler(
				func(context.Context, string, []string, sequence.Policy) sequence.Outcome {
					return testCase.outcome
				},
				nil,
				zap.NewNop(),
			)

			resp := handler.handleRun(context.Background(), ipc.Command{Args: []string{"a", "b"}})

			if resp.Success {
				t.Fatalf("handleRun() = %+v, want failure", resp)
			}

			if resp.Code != testCase.wantCode {
				t.Fatalf("code = %q, want %q", resp.Code, testCase.wantCode)
			}

			// The reporting has to name the step the caller wrote, not the
			// position the sequence happened to reach.
			if !strings.Contains(resp.Message, testCase.outcome.FailedStep) {
				t.Fatalf("message %q does not name the failing step", resp.Message)
			}
		})
	}
}

// A sleep step inside a sequence must release its goroutine when the daemon
// shuts down, rather than holding it for the rest of the duration.
func TestHandleSleepAction_ReleasedOnCancel(t *testing.T) {
	t.Parallel()

	handler := &ActionsHandler{logger: zap.NewNop()}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan ipc.Response, 1)

	go func() {
		done <- handler.handleSleepAction(ctx, []string{"30s"})
	}()

	select {
	case resp := <-done:
		if resp.Success {
			t.Fatalf("handleSleepAction() = %+v, want a canceled failure", resp)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handleSleepAction() ignored a canceled context")
	}
}

// The sequence-wide flag is consumed by the handler, so it never reaches the
// executor as a step, and it turns on the stop policy.
func TestHandleRun_StopOnErrorFlagIsAPolicyNotAStep(t *testing.T) {
	t.Parallel()

	var (
		gotSteps  []string
		gotPolicy sequence.Policy
	)

	handler := NewSequenceHandler(
		func(_ context.Context, _ string, steps []string, policy sequence.Policy) sequence.Outcome {
			gotSteps = slices.Clone(steps)
			gotPolicy = policy

			return sequence.Outcome{Executed: len(steps)}
		},
		nil,
		zap.NewNop(),
	)

	resp := handler.handleRun(context.Background(), ipc.Command{
		Args: []string{stopOnErrorFlag, leftClickStep, hintsStep},
	})

	if !resp.Success {
		t.Fatalf("handleRun() = %+v, want success", resp)
	}

	if !gotPolicy.StopOnError {
		t.Fatal("policy.StopOnError = false, want the flag to enable it")
	}

	want := []string{leftClickStep, hintsStep}
	if !slices.Equal(gotSteps, want) {
		t.Fatalf("executor received %v, want %v", gotSteps, want)
	}
}

func TestHandleRun_StopOnErrorAloneIsNotASequence(t *testing.T) {
	t.Parallel()

	handler := NewSequenceHandler(
		func(context.Context, string, []string, sequence.Policy) sequence.Outcome {
			t.Fatal("the executor must not run when the flag is the only argument")

			return sequence.Outcome{}
		},
		nil,
		zap.NewNop(),
	)

	resp := handler.handleRun(context.Background(), ipc.Command{Args: []string{stopOnErrorFlag}})

	if resp.Success || resp.Code != ipc.CodeInvalidInput {
		t.Fatalf("handleRun() = %+v, want an invalid-input failure", resp)
	}
}

// A sequence refused before it started has no step to point at, so the report
// must not invent one.
func TestHandleRun_ReportsASequenceThatNeverStarted(t *testing.T) {
	t.Parallel()

	handler := NewSequenceHandler(
		func(context.Context, string, []string, sequence.Policy) sequence.Outcome {
			return sequence.Outcome{
				Err:     derrors.New(derrors.CodeInvalidInput, "action sequence nested too deeply"),
				Stopped: true,
			}
		},
		nil,
		zap.NewNop(),
	)

	resp := handler.handleRun(context.Background(), ipc.Command{Args: []string{leftClickStep}})

	if resp.Success || resp.Code != ipc.CodeInvalidInput {
		t.Fatalf("handleRun() = %+v, want an invalid-input failure", resp)
	}

	if strings.Contains(resp.Message, "step 0") {
		t.Fatalf("message %q names a step that does not exist", resp.Message)
	}
}

// "later steps still ran" must only appear when there were later steps: a
// tolerated failure on the final step leaves nothing after it.
func TestHandleRun_DoesNotClaimLaterStepsRanWhenThereWereNone(t *testing.T) {
	t.Parallel()

	handler := NewSequenceHandler(
		func(context.Context, string, []string, sequence.Policy) sequence.Outcome {
			return sequence.Outcome{
				Err:         derrors.New(derrors.CodeIPCFailed, "boom"),
				FailedStep:  leftClickStep,
				FailedIndex: 1,
				Executed:    1,
			}
		},
		nil,
		zap.NewNop(),
	)

	resp := handler.handleRun(context.Background(), ipc.Command{Args: []string{leftClickStep}})

	if strings.Contains(resp.Message, "later steps still ran") {
		t.Fatalf("message %q claims steps ran after the last one", resp.Message)
	}
}

func TestHandleMacro_RequiresAName(t *testing.T) {
	t.Parallel()

	handler := NewSequenceHandler(
		nil,
		func(context.Context, string, []string) error {
			t.Fatal("the macro runner must not run without a name")

			return nil
		},
		zap.NewNop(),
	)

	for _, args := range [][]string{nil, {}, {"  ", ""}} {
		resp := handler.handleMacro(context.Background(), ipc.Command{Args: args})

		if resp.Success || resp.Code != ipc.CodeInvalidInput {
			t.Fatalf("handleMacro(%v) = %+v, want an invalid-input failure", args, resp)
		}
	}
}

func TestHandleMacro_ReportsMissingRunner(t *testing.T) {
	t.Parallel()

	handler := NewSequenceHandler(nil, nil, zap.NewNop())

	resp := handler.handleMacro(context.Background(), ipc.Command{Args: []string{"zoom"}})

	if resp.Success || resp.Code != ipc.CodeActionFailed {
		t.Fatalf("handleMacro() = %+v, want an action-failed response", resp)
	}
}

func TestHandleMacro_PassesArgumentsThroughUnsplit(t *testing.T) {
	t.Parallel()

	var (
		gotName string
		gotArgs []string
	)

	handler := NewSequenceHandler(
		nil,
		func(_ context.Context, name string, args []string) error {
			gotName, gotArgs = name, slices.Clone(args)

			return nil
		},
		zap.NewNop(),
	)

	// An argument containing spaces and both kinds of quote is exactly what a
	// call written as one step string could not carry intact.
	args := []string{"say_it", `hello "there" it's me`, "tail"}

	resp := handler.handleMacro(context.Background(), ipc.Command{Args: args})
	if !resp.Success {
		t.Fatalf("handleMacro() = %+v, want success", resp)
	}

	if gotName != "say_it" {
		t.Fatalf("macro name = %q, want %q", gotName, "say_it")
	}

	if !slices.Equal(gotArgs, args[1:]) {
		t.Fatalf("macro args = %q, want %q", gotArgs, args[1:])
	}
}

func TestHandleMacro_MapsFailureOntoTheCodeThatDescribesIt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		wantCode string
	}{
		{
			name:     "unknown macro",
			err:      derrors.New(derrors.CodeInvalidInput, `no macro named "nope"`),
			wantCode: ipc.CodeInvalidInput,
		},
		{
			// A canceled mode is not a fault, and a caller chaining calls
			// needs to tell the two apart.
			name:     "canceled mode",
			err:      derrors.New(derrors.CodeChainBail, "mode exited without selection"),
			wantCode: ipc.CodeChainBail,
		},
		{
			name:     "step failed",
			err:      derrors.New(derrors.CodeActionFailed, "click failed"),
			wantCode: ipc.CodeActionFailed,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			handler := NewSequenceHandler(
				nil,
				func(context.Context, string, []string) error {
					return testCase.err
				},
				zap.NewNop(),
			)

			resp := handler.handleMacro(context.Background(), ipc.Command{Args: []string{"m"}})

			if resp.Success {
				t.Fatalf("handleMacro() = %+v, want failure", resp)
			}

			if resp.Code != testCase.wantCode {
				t.Fatalf("code = %q, want %q", resp.Code, testCase.wantCode)
			}

			if !strings.Contains(resp.Message, testCase.err.Error()) {
				t.Fatalf("message %q does not carry the underlying error", resp.Message)
			}
		})
	}
}

func TestRegisterHandlers_RegistersRunAndMacro(t *testing.T) {
	t.Parallel()

	handlers := make(map[string]func(context.Context, ipc.Command) ipc.Response)

	NewSequenceHandler(nil, nil, zap.NewNop()).RegisterHandlers(handlers)

	for _, command := range []string{domain.CommandRun, domain.CommandMacro} {
		if handlers[command] == nil {
			t.Fatalf("no handler registered for %q", command)
		}
	}
}

// Macro arguments are data, not steps. Trimming them or dropping the blank ones
// would rewrite an argument whose padding matters and shift every later
// argument onto the wrong placeholder.
func TestHandleMacro_PassesArgumentsThroughVerbatim(t *testing.T) {
	t.Parallel()

	var gotArgs []string

	handler := NewSequenceHandler(
		nil,
		func(_ context.Context, _ string, args []string) error {
			gotArgs = slices.Clone(args)

			return nil
		},
		zap.NewNop(),
	)

	args := []string{"  say_it  ", " padded ", "", "third"}

	resp := handler.handleMacro(context.Background(), ipc.Command{Args: args})
	if !resp.Success {
		t.Fatalf("handleMacro() = %+v, want success", resp)
	}

	if !slices.Equal(gotArgs, args[1:]) {
		t.Fatalf("macro args = %q, want %q", gotArgs, args[1:])
	}
}
