//nolint:testpackage // Tests the private run-command handler.
package app

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

// The steps these tests round-trip through the handler.
const (
	leftClickStep = "action left_click"
	hintsStep     = "hints --action left_click"
)

func TestHandleRun_RequiresSteps(t *testing.T) {
	t.Parallel()

	handler := NewIPCControllerSequence(
		func(context.Context, string, []string, sequencePolicy) sequenceOutcome {
			t.Fatal("the executor must not run for an empty sequence")

			return sequenceOutcome{}
		},
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

	handler := NewIPCControllerSequence(nil, zap.NewNop())

	resp := handler.handleRun(context.Background(), ipc.Command{Args: []string{"idle"}})

	if resp.Success || resp.Code != ipc.CodeActionFailed {
		t.Fatalf("handleRun() = %+v, want an action-failed response", resp)
	}
}

func TestHandleRun_PassesTrimmedStepsToExecutor(t *testing.T) {
	t.Parallel()

	var got []string

	handler := NewIPCControllerSequence(
		func(_ context.Context, _ string, steps []string, _ sequencePolicy) sequenceOutcome {
			got = slices.Clone(steps)

			return sequenceOutcome{executed: len(steps)}
		},
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
		outcome  sequenceOutcome
		wantCode string
	}{
		{
			name: "bail",
			outcome: sequenceOutcome{
				err:         derrors.New(derrors.CodeChainBail, "mode exited without selection"),
				failedStep:  "action wait_for_mode_exit --bail",
				failedIndex: 2,
				executed:    2,
				bailed:      true,
			},
			wantCode: ipc.CodeChainBail,
		},
		{
			name: "failed step",
			outcome: sequenceOutcome{
				err:         derrors.New(derrors.CodeIPCFailed, "boom"),
				failedStep:  leftClickStep,
				failedIndex: 1,
				executed:    3,
			},
			wantCode: ipc.CodeActionFailed,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			handler := NewIPCControllerSequence(
				func(context.Context, string, []string, sequencePolicy) sequenceOutcome {
					return testCase.outcome
				},
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
			if !strings.Contains(resp.Message, testCase.outcome.failedStep) {
				t.Fatalf("message %q does not name the failing step", resp.Message)
			}
		})
	}
}

// A sleep step inside a sequence must release its goroutine when the daemon
// shuts down, rather than holding it for the rest of the duration.
func TestHandleSleepAction_ReleasedOnCancel(t *testing.T) {
	t.Parallel()

	handler := &IPCControllerActions{logger: zap.NewNop()}

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
		gotPolicy sequencePolicy
	)

	handler := NewIPCControllerSequence(
		func(_ context.Context, _ string, steps []string, policy sequencePolicy) sequenceOutcome {
			gotSteps = slices.Clone(steps)
			gotPolicy = policy

			return sequenceOutcome{executed: len(steps)}
		},
		zap.NewNop(),
	)

	resp := handler.handleRun(context.Background(), ipc.Command{
		Args: []string{stopOnErrorFlag, leftClickStep, hintsStep},
	})

	if !resp.Success {
		t.Fatalf("handleRun() = %+v, want success", resp)
	}

	if !gotPolicy.stopOnError {
		t.Fatal("policy.stopOnError = false, want the flag to enable it")
	}

	want := []string{leftClickStep, hintsStep}
	if !slices.Equal(gotSteps, want) {
		t.Fatalf("executor received %v, want %v", gotSteps, want)
	}
}

func TestHandleRun_StopOnErrorAloneIsNotASequence(t *testing.T) {
	t.Parallel()

	handler := NewIPCControllerSequence(
		func(context.Context, string, []string, sequencePolicy) sequenceOutcome {
			t.Fatal("the executor must not run when the flag is the only argument")

			return sequenceOutcome{}
		},
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

	handler := NewIPCControllerSequence(
		func(context.Context, string, []string, sequencePolicy) sequenceOutcome {
			return sequenceOutcome{
				err:     derrors.New(derrors.CodeInvalidInput, "action sequence nested too deeply"),
				stopped: true,
			}
		},
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

	handler := NewIPCControllerSequence(
		func(context.Context, string, []string, sequencePolicy) sequenceOutcome {
			return sequenceOutcome{
				err:         derrors.New(derrors.CodeIPCFailed, "boom"),
				failedStep:  leftClickStep,
				failedIndex: 1,
				executed:    1,
			}
		},
		zap.NewNop(),
	)

	resp := handler.handleRun(context.Background(), ipc.Command{Args: []string{leftClickStep}})

	if strings.Contains(resp.Message, "later steps still ran") {
		t.Fatalf("message %q claims steps ran after the last one", resp.Message)
	}
}
