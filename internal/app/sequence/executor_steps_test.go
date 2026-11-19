package sequence_test

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/sequence"
	"github.com/y3owk1n/neru/internal/config"
)

const (
	failureMessage = "boom"
	bailMessage    = "mode exited without selection"

	stepOne   = "step one"
	stepTwo   = "step two"
	stepThree = "step three"
)

// stepRecorder is the command handler the executor dispatches to, recording
// every step a sequence dispatched and deciding how each one replies.
type stepRecorder struct {
	respond func(ctx context.Context, step string) ipc.Response
	steps   []string
	mu      sync.Mutex
}

func (r *stepRecorder) HandleCommand(ctx context.Context, cmd ipc.Command) ipc.Response {
	step := strings.TrimSpace(cmd.Action + " " + strings.Join(cmd.Args, " "))

	r.mu.Lock()
	r.steps = append(r.steps, step)
	r.mu.Unlock()

	if r.respond == nil {
		return ipc.Response{Success: true, Message: "ok", Code: ipc.CodeOK}
	}

	return r.respond(ctx, step)
}

func (r *stepRecorder) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.steps)
}

// newStepExecutor builds an executor that dispatches every step to the
// recorder, under a config that defines the given macros.
func newStepExecutor(recorder *stepRecorder, macros map[string][]string) *sequence.Executor {
	cfg := config.DefaultConfig()

	table := make(map[string]config.StringOrStringArray, len(macros))
	for name, steps := range macros {
		table[name] = steps
	}

	cfg.Macros = table

	return sequence.NewExecutor(sequence.ExecutorDeps{
		Commands:    recorder,
		Config:      func() *config.Config { return cfg },
		BaseContext: context.Background,
	})
}

func TestExecutor_Run_SkipsBlankSteps(t *testing.T) {
	recorder := &stepRecorder{}
	executor := newStepExecutor(recorder, nil)

	outcome := executor.Run(
		context.Background(),
		"test",
		[]string{"", stepOne, "   ", stepTwo},
	)

	if outcome.Executed != 2 {
		t.Fatalf("executed = %d, want 2 (blank steps should not count)", outcome.Executed)
	}

	want := []string{stepOne, stepTwo}
	if got := recorder.recorded(); !slices.Equal(got, want) {
		t.Fatalf("dispatched %v, want %v", got, want)
	}
}

func TestExecutor_Run_StopsOnBail(t *testing.T) {
	recorder := &stepRecorder{
		respond: func(_ context.Context, step string) ipc.Response {
			if step == stepTwo {
				return ipc.Response{
					Success: false,
					Message: bailMessage,
					Code:    ipc.CodeChainBail,
				}
			}

			return ipc.Response{Success: true, Code: ipc.CodeOK}
		},
	}
	executor := newStepExecutor(recorder, nil)

	outcome := executor.Run(
		context.Background(),
		"test",
		[]string{stepOne, stepTwo, stepThree},
	)

	if !outcome.Bailed {
		t.Fatal("outcome.Bailed = false, want true")
	}

	if outcome.Executed != 2 || outcome.FailedIndex != 2 {
		t.Fatalf("executed = %d, failedIndex = %d, want 2 and 2",
			outcome.Executed, outcome.FailedIndex)
	}

	want := []string{stepOne, stepTwo}
	if got := recorder.recorded(); !slices.Equal(got, want) {
		t.Fatalf("dispatched %v, want %v (a bail must stop the sequence)", got, want)
	}
}

func TestExecutor_Run_ContinuesAfterStepFailure(t *testing.T) {
	recorder := &stepRecorder{
		respond: func(_ context.Context, step string) ipc.Response {
			if step == stepOne {
				return ipc.Response{
					Success: false,
					Message: "boom",
					Code:    ipc.CodeActionFailed,
				}
			}

			return ipc.Response{Success: true, Code: ipc.CodeOK}
		},
	}
	executor := newStepExecutor(recorder, nil)

	steps := []string{stepOne, stepTwo, stepThree}

	outcome := executor.Run(context.Background(), "test", steps)

	if outcome.Bailed {
		t.Fatal("outcome.Bailed = true, want false for a regular failure")
	}

	if outcome.Err == nil {
		t.Fatal("outcome.Err = nil, want the first step error")
	}

	if outcome.FailedStep != stepOne || outcome.FailedIndex != 1 {
		t.Fatalf("failedStep = %q at %d, want %q at 1",
			outcome.FailedStep, outcome.FailedIndex, stepOne)
	}

	if outcome.Executed != len(steps) {
		t.Fatalf("executed = %d, want %d (a regular failure must not stop the sequence)",
			outcome.Executed, len(steps))
	}
}

// A step can start another sequence ("run" is itself an action), so the depth
// carried through the step context is what stops a binding that refers back to
// itself from recursing without bound.
func TestExecutor_Run_StopsRunawayNesting(t *testing.T) {
	var (
		executor *sequence.Executor
		depth    int
		depthMu  sync.Mutex
	)

	recorder := &stepRecorder{
		respond: func(ctx context.Context, _ string) ipc.Response {
			depthMu.Lock()
			depth++
			depthMu.Unlock()

			executor.Run(ctx, "nested", []string{"step deeper"})

			return ipc.Response{Success: true, Code: ipc.CodeOK}
		},
	}

	executor = newStepExecutor(recorder, nil)

	executor.Run(context.Background(), "test", []string{stepOne})

	depthMu.Lock()
	defer depthMu.Unlock()

	if depth != sequence.MaxDepth {
		t.Fatalf("nested %d levels, want the guard to stop at %d", depth, sequence.MaxDepth)
	}
}

// A step marked fatal ends the sequence, while an unmarked failure beside it
// does not. That is the whole point of making the policy explicit per step.
func TestExecutor_Run_StopsAtAFatalStep(t *testing.T) {
	recorder := &stepRecorder{
		respond: func(_ context.Context, step string) ipc.Response {
			if step == stepTwo {
				return ipc.Response{
					Success: false,
					Message: failureMessage,
					Code:    ipc.CodeActionFailed,
				}
			}

			return ipc.Response{Success: true, Code: ipc.CodeOK}
		},
	}
	executor := newStepExecutor(recorder, nil)

	outcome := executor.Run(
		context.Background(),
		"test",
		[]string{stepOne, stepTwo + " " + sequence.BailOnErrorFlag, stepThree},
	)

	if !outcome.Stopped {
		t.Fatal("outcome.Stopped = false, want the sequence to end at the fatal step")
	}

	if outcome.Bailed {
		t.Fatal("outcome.Bailed = true, want false: a failure is not a chain bail")
	}

	// The directive is a sequencing instruction, so the step must be dispatched
	// without it, or the action's own flag parser would reject it.
	want := []string{stepOne, stepTwo}
	if got := recorder.recorded(); !slices.Equal(got, want) {
		t.Fatalf("dispatched %v, want %v", got, want)
	}
}

func TestExecutor_RunWithPolicy_StopOnErrorMarksEveryStep(t *testing.T) {
	recorder := &stepRecorder{
		respond: func(_ context.Context, step string) ipc.Response {
			if step == stepOne {
				return ipc.Response{
					Success: false,
					Message: failureMessage,
					Code:    ipc.CodeActionFailed,
				}
			}

			return ipc.Response{Success: true, Code: ipc.CodeOK}
		},
	}
	executor := newStepExecutor(recorder, nil)

	outcome := executor.RunWithPolicy(
		context.Background(),
		"test",
		[]string{stepOne, stepTwo},
		sequence.Policy{StopOnError: true},
	)

	if !outcome.Stopped || outcome.Executed != 1 {
		t.Fatalf("stopped = %v after %d steps, want a stop at the first",
			outcome.Stopped, outcome.Executed)
	}
}

// A misplaced directive must not be dispatched as though it were part of the
// action, because the action's flag parser would reject it with a message that
// says nothing about sequencing.
func TestExecutor_Run_RejectsMisplacedDirective(t *testing.T) {
	recorder := &stepRecorder{}
	executor := newStepExecutor(recorder, nil)

	outcome := executor.Run(
		context.Background(),
		"test",
		[]string{stepOne + " " + sequence.BailOnErrorFlag + " --bare", stepTwo},
	)

	if !outcome.Stopped || outcome.Err == nil {
		t.Fatalf("outcome = %+v, want a stop with an error", outcome)
	}

	if got := recorder.recorded(); len(got) != 0 {
		t.Fatalf("dispatched %v, want nothing: the step was never valid", got)
	}
}

func TestExecutor_Run_ExpandsAMacro(t *testing.T) {
	recorder := &stepRecorder{}
	executor := newStepExecutor(recorder, map[string][]string{
		"two": {stepOne, stepTwo},
	})

	outcome := executor.Run(
		context.Background(),
		"test",
		[]string{"macro two", stepThree},
	)

	if outcome.Err != nil {
		t.Fatalf("outcome.Err = %v, want nil", outcome.Err)
	}

	// The caller counts the step it wrote, not the steps the macro carried.
	if outcome.Executed != 2 {
		t.Fatalf("executed = %d, want 2", outcome.Executed)
	}

	want := []string{stepOne, stepTwo, stepThree}
	if got := recorder.recorded(); !slices.Equal(got, want) {
		t.Fatalf("dispatched %v, want %v", got, want)
	}
}

func TestExecutor_Run_SubstitutesMacroArguments(t *testing.T) {
	recorder := &stepRecorder{}
	executor := newStepExecutor(recorder, map[string][]string{
		"say": {"step $2 $1"},
	})

	executor.Run(
		context.Background(),
		"test",
		[]string{"macro say alpha beta"},
	)

	want := []string{"step beta alpha"}
	if got := recorder.recorded(); !slices.Equal(got, want) {
		t.Fatalf("dispatched %v, want %v", got, want)
	}
}

func TestExecutor_Run_RejectsBadMacroCalls(t *testing.T) {
	tests := []struct {
		name string
		step string
	}{
		{name: "unknown macro", step: "macro nope"},
		{name: "too few arguments", step: "macro needs_two alpha"},
		{name: "too many arguments", step: "macro needs_two alpha beta gamma"},
		{name: "no name", step: "macro"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := &stepRecorder{}
			executor := newStepExecutor(recorder, map[string][]string{
				"needs_two": {"step $1 $2"},
			})

			outcome := executor.Run(
				context.Background(),
				"test",
				[]string{testCase.step},
			)

			if outcome.Err == nil {
				t.Fatalf("outcome.Err = nil, want a rejection of %q", testCase.step)
			}

			if got := recorder.recorded(); len(got) != 0 {
				t.Fatalf("dispatched %v, want nothing", got)
			}
		})
	}
}

// A macro that invokes itself is a runaway sequence, and the depth guard is
// what stops it, the same guard that bounds a nested run.
func TestExecutor_Run_StopsASelfInvokingMacro(t *testing.T) {
	recorder := &stepRecorder{}
	executor := newStepExecutor(recorder, map[string][]string{
		"loop": {stepOne, "macro loop"},
	})

	outcome := executor.Run(
		context.Background(),
		"test",
		[]string{"macro loop"},
	)

	if outcome.Err == nil {
		t.Fatal("outcome.Err = nil, want the depth guard to stop the recursion")
	}

	if got := len(recorder.recorded()); got != sequence.MaxDepth-1 {
		t.Fatalf("dispatched %d steps, want the guard to stop at %d", got, sequence.MaxDepth-1)
	}
}

// A canceled mode inside a macro must still read as a bail to the caller,
// rather than as an ordinary failure it might carry on past.
func TestExecutor_Run_PropagatesABailOutOfAMacro(t *testing.T) {
	recorder := &stepRecorder{
		respond: func(_ context.Context, step string) ipc.Response {
			if step == stepOne {
				return ipc.Response{
					Success: false,
					Message: bailMessage,
					Code:    ipc.CodeChainBail,
				}
			}

			return ipc.Response{Success: true, Code: ipc.CodeOK}
		},
	}
	executor := newStepExecutor(recorder, map[string][]string{
		"bails": {stepOne},
	})

	outcome := executor.Run(
		context.Background(),
		"test",
		[]string{"macro bails", stepTwo},
	)

	if !outcome.Bailed || !outcome.Stopped {
		t.Fatalf("outcome bailed=%v stopped=%v, want both", outcome.Bailed, outcome.Stopped)
	}

	if got := recorder.recorded(); !slices.Equal(got, []string{stepOne}) {
		t.Fatalf("dispatched %v, want the sequence to stop after the macro", got)
	}
}

// The bail directive is consumed before the call is parsed, so it must not be
// mistaken for one of the macro's arguments.
func TestExecutor_Run_BailDirectiveIsNotAMacroArgument(t *testing.T) {
	recorder := &stepRecorder{
		respond: func(_ context.Context, _ string) ipc.Response {
			return ipc.Response{Success: false, Message: failureMessage, Code: ipc.CodeActionFailed}
		},
	}
	executor := newStepExecutor(recorder, map[string][]string{
		"needs_two": {"step $1 $2"},
	})

	outcome := executor.Run(
		context.Background(),
		"test",
		[]string{"macro needs_two alpha beta " + sequence.BailOnErrorFlag, stepThree},
	)

	// The call is well-formed. It is the macro's own step that fails, and the
	// directive makes that failure end the calling sequence.
	if !outcome.Stopped {
		t.Fatalf("outcome = %+v, want the directive to stop the sequence", outcome)
	}

	want := []string{"step alpha beta"}
	if got := recorder.recorded(); !slices.Equal(got, want) {
		t.Fatalf("dispatched %v, want %v", got, want)
	}
}
