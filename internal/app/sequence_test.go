//nolint:testpackage // Tests the private action sequence executor.
package app

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

// sequenceTestCommand is the fake IPC command the sequence tests dispatch, so
// a step exercises the real executeHotkeyAction path without depending on any
// service.
const (
	sequenceTestCommand = "step"

	failureMessage = "boom"
	bailMessage    = "mode exited without selection"

	stepOne   = "step one"
	stepTwo   = "step two"
	stepThree = "step three"
)

// stepRecorder answers the fake step command, recording every step a sequence
// dispatched and deciding how each one replies.
type stepRecorder struct {
	respond func(ctx context.Context, step string) ipc.Response
	steps   []string
	mu      sync.Mutex
}

func (r *stepRecorder) handle(ctx context.Context, cmd ipc.Command) ipc.Response {
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

// newSequenceTestApp builds a whitebox App whose IPC controller answers the
// fake step command with the recorder. Everything the executor touches — the
// logger, the app context, the controller — is real; only the command the
// steps name is fake.
func newSequenceTestApp(t *testing.T, recorder *stepRecorder) *App {
	t.Helper()

	logger := zap.NewNop()
	cfg := config.DefaultConfig()
	appState := state.NewAppState()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	application := &App{
		ctx:      ctx,
		logger:   logger,
		config:   cfg,
		appState: appState,
		ipcController: NewIPCController(IPCControllerDeps{
			ConfigService: config.NewService(cfg, "", logger, nil),
			AppState:      appState,
			Config:        cfg,
			Logger:        logger,
		}),
	}

	application.ipcController.Handlers[sequenceTestCommand] = recorder.handle

	return application
}

func TestExecuteActionSequence_RunsEveryStepInOrder(t *testing.T) {
	recorder := &stepRecorder{}
	application := newSequenceTestApp(t, recorder)

	steps := []string{stepOne, stepTwo, stepThree}

	outcome := application.executeActionSequence(context.Background(), "test", steps)

	if outcome.err != nil {
		t.Fatalf("outcome.err = %v, want nil", outcome.err)
	}

	if outcome.executed != len(steps) {
		t.Fatalf("executed = %d, want %d", outcome.executed, len(steps))
	}

	if got := recorder.recorded(); !slices.Equal(got, steps) {
		t.Fatalf("dispatched %v, want %v", got, steps)
	}
}

func TestExecuteActionSequence_SkipsBlankSteps(t *testing.T) {
	recorder := &stepRecorder{}
	application := newSequenceTestApp(t, recorder)

	outcome := application.executeActionSequence(
		context.Background(),
		"test",
		[]string{"", stepOne, "   ", stepTwo},
	)

	if outcome.executed != 2 {
		t.Fatalf("executed = %d, want 2 (blank steps should not count)", outcome.executed)
	}

	want := []string{stepOne, stepTwo}
	if got := recorder.recorded(); !slices.Equal(got, want) {
		t.Fatalf("dispatched %v, want %v", got, want)
	}
}

func TestExecuteActionSequence_StopsOnBail(t *testing.T) {
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
	application := newSequenceTestApp(t, recorder)

	outcome := application.executeActionSequence(
		context.Background(),
		"test",
		[]string{stepOne, stepTwo, stepThree},
	)

	if !outcome.bailed {
		t.Fatal("outcome.bailed = false, want true")
	}

	if outcome.executed != 2 || outcome.failedIndex != 2 {
		t.Fatalf("executed = %d, failedIndex = %d, want 2 and 2",
			outcome.executed, outcome.failedIndex)
	}

	want := []string{stepOne, stepTwo}
	if got := recorder.recorded(); !slices.Equal(got, want) {
		t.Fatalf("dispatched %v, want %v (a bail must stop the sequence)", got, want)
	}
}

func TestExecuteActionSequence_ContinuesAfterStepFailure(t *testing.T) {
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
	application := newSequenceTestApp(t, recorder)

	steps := []string{stepOne, stepTwo, stepThree}

	outcome := application.executeActionSequence(context.Background(), "test", steps)

	if outcome.bailed {
		t.Fatal("outcome.bailed = true, want false for a regular failure")
	}

	if outcome.err == nil {
		t.Fatal("outcome.err = nil, want the first step error")
	}

	if outcome.failedStep != stepOne || outcome.failedIndex != 1 {
		t.Fatalf("failedStep = %q at %d, want %q at 1",
			outcome.failedStep, outcome.failedIndex, stepOne)
	}

	if outcome.executed != len(steps) {
		t.Fatalf("executed = %d, want %d (a regular failure must not stop the sequence)",
			outcome.executed, len(steps))
	}
}

// A step can start another sequence ("run" is itself an action), so the depth
// carried through the step context is what stops a binding that refers back to
// itself from recursing without bound.
func TestExecuteActionSequence_StopsRunawayNesting(t *testing.T) {
	var (
		application *App
		depth       int
		depthMu     sync.Mutex
	)

	recorder := &stepRecorder{
		respond: func(ctx context.Context, _ string) ipc.Response {
			depthMu.Lock()
			depth++
			depthMu.Unlock()

			application.executeActionSequence(ctx, "nested", []string{"step deeper"})

			return ipc.Response{Success: true, Code: ipc.CodeOK}
		},
	}

	application = newSequenceTestApp(t, recorder)

	application.executeActionSequence(context.Background(), "test", []string{stepOne})

	depthMu.Lock()
	defer depthMu.Unlock()

	if depth != maxSequenceDepth {
		t.Fatalf("nested %d levels, want the guard to stop at %d", depth, maxSequenceDepth)
	}
}

// A step marked fatal ends the sequence, while an unmarked failure beside it
// does not — that is the whole point of making the policy explicit per step.
func TestExecuteActionSequence_StopsAtAFatalStep(t *testing.T) {
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
	application := newSequenceTestApp(t, recorder)

	outcome := application.executeActionSequence(
		context.Background(),
		"test",
		[]string{stepOne, stepTwo + " " + bailOnErrorFlag, stepThree},
	)

	if !outcome.stopped {
		t.Fatal("outcome.stopped = false, want the sequence to end at the fatal step")
	}

	if outcome.bailed {
		t.Fatal("outcome.bailed = true, want false: a failure is not a chain bail")
	}

	// The directive is a sequencing instruction, so the step must be dispatched
	// without it — the action's own flag parser would reject it.
	want := []string{stepOne, stepTwo}
	if got := recorder.recorded(); !slices.Equal(got, want) {
		t.Fatalf("dispatched %v, want %v", got, want)
	}
}

func TestExecuteActionSequence_StopOnErrorPolicyMarksEveryStep(t *testing.T) {
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
	application := newSequenceTestApp(t, recorder)

	outcome := application.executeActionSequenceWithPolicy(
		context.Background(),
		"test",
		[]string{stepOne, stepTwo},
		sequencePolicy{stopOnError: true},
	)

	if !outcome.stopped || outcome.executed != 1 {
		t.Fatalf("stopped = %v after %d steps, want a stop at the first",
			outcome.stopped, outcome.executed)
	}
}

func TestSplitBailOnError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		step      string
		wantStep  string
		wantFatal bool
		wantErr   bool
	}{
		{
			name:      "trailing directive is consumed",
			step:      "action left_click " + bailOnErrorFlag,
			wantStep:  leftClickStep,
			wantFatal: true,
		},
		{
			name:     "no directive",
			step:     leftClickStep,
			wantStep: leftClickStep,
		},
		{
			// The text appears inside a quoted argument, so it belongs to the
			// step rather than to the sequence.
			name:     "quoted text is not a directive",
			step:     `exec sh -c "echo ` + bailOnErrorFlag + `"`,
			wantStep: `exec sh -c "echo ` + bailOnErrorFlag + `"`,
		},
		{
			// Quoted as the final argument: the author wants the text, not the
			// directive, so the step must reach the action unchanged.
			name:     "quoted final token is an argument",
			step:     `exec printf '` + bailOnErrorFlag + `'`,
			wantStep: `exec printf '` + bailOnErrorFlag + `'`,
		},
		{
			name:    "directive in the middle is rejected",
			step:    "action left_click " + bailOnErrorFlag + " --bare",
			wantErr: true,
		},
		{
			name:     "directive alone is not a step",
			step:     bailOnErrorFlag,
			wantStep: bailOnErrorFlag,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gotStep, gotFatal, gotErr := splitBailOnError(testCase.step)

			if (gotErr != nil) != testCase.wantErr {
				t.Fatalf("error = %v, wantErr = %v", gotErr, testCase.wantErr)
			}

			if testCase.wantErr {
				return
			}

			if gotStep != testCase.wantStep || gotFatal != testCase.wantFatal {
				t.Fatalf("splitBailOnError(%q) = (%q, %v), want (%q, %v)",
					testCase.step, gotStep, gotFatal, testCase.wantStep, testCase.wantFatal)
			}
		})
	}
}

// A misplaced directive must not be dispatched as though it were part of the
// action, because the action's flag parser would reject it with a message that
// says nothing about sequencing.
func TestExecuteActionSequence_RejectsMisplacedDirective(t *testing.T) {
	recorder := &stepRecorder{}
	application := newSequenceTestApp(t, recorder)

	outcome := application.executeActionSequence(
		context.Background(),
		"test",
		[]string{stepOne + " " + bailOnErrorFlag + " --bare", stepTwo},
	)

	if !outcome.stopped || outcome.err == nil {
		t.Fatalf("outcome = %+v, want a stop with an error", outcome)
	}

	if got := recorder.recorded(); len(got) != 0 {
		t.Fatalf("dispatched %v, want nothing: the step was never valid", got)
	}
}

// macroTestApp is a sequence test app whose config defines the given macros.
func macroTestApp(t *testing.T, recorder *stepRecorder, macros map[string][]string) *App {
	t.Helper()

	application := newSequenceTestApp(t, recorder)

	table := make(map[string]config.StringOrStringArray, len(macros))
	for name, steps := range macros {
		table[name] = steps
	}

	application.config.Macros = table

	return application
}

func TestExecuteActionSequence_ExpandsAMacro(t *testing.T) {
	recorder := &stepRecorder{}
	application := macroTestApp(t, recorder, map[string][]string{
		"two": {stepOne, stepTwo},
	})

	outcome := application.executeActionSequence(
		context.Background(),
		"test",
		[]string{"macro two", stepThree},
	)

	if outcome.err != nil {
		t.Fatalf("outcome.err = %v, want nil", outcome.err)
	}

	// The caller counts the step it wrote, not the steps the macro carried.
	if outcome.executed != 2 {
		t.Fatalf("executed = %d, want 2", outcome.executed)
	}

	want := []string{stepOne, stepTwo, stepThree}
	if got := recorder.recorded(); !slices.Equal(got, want) {
		t.Fatalf("dispatched %v, want %v", got, want)
	}
}

func TestExecuteActionSequence_SubstitutesMacroArguments(t *testing.T) {
	recorder := &stepRecorder{}
	application := macroTestApp(t, recorder, map[string][]string{
		"say": {"step $2 $1"},
	})

	application.executeActionSequence(
		context.Background(),
		"test",
		[]string{"macro say alpha beta"},
	)

	want := []string{"step beta alpha"}
	if got := recorder.recorded(); !slices.Equal(got, want) {
		t.Fatalf("dispatched %v, want %v", got, want)
	}
}

func TestExecuteActionSequence_RejectsBadMacroCalls(t *testing.T) {
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
			application := macroTestApp(t, recorder, map[string][]string{
				"needs_two": {"step $1 $2"},
			})

			outcome := application.executeActionSequence(
				context.Background(),
				"test",
				[]string{testCase.step},
			)

			if outcome.err == nil {
				t.Fatalf("outcome.err = nil, want a rejection of %q", testCase.step)
			}

			if got := recorder.recorded(); len(got) != 0 {
				t.Fatalf("dispatched %v, want nothing", got)
			}
		})
	}
}

// A macro that invokes itself is a runaway sequence, and the depth guard is
// what stops it — the same guard that bounds a nested run.
func TestExecuteActionSequence_StopsASelfInvokingMacro(t *testing.T) {
	recorder := &stepRecorder{}
	application := macroTestApp(t, recorder, map[string][]string{
		"loop": {stepOne, "macro loop"},
	})

	outcome := application.executeActionSequence(
		context.Background(),
		"test",
		[]string{"macro loop"},
	)

	if outcome.err == nil {
		t.Fatal("outcome.err = nil, want the depth guard to stop the recursion")
	}

	if got := len(recorder.recorded()); got != maxSequenceDepth-1 {
		t.Fatalf("dispatched %d steps, want the guard to stop at %d", got, maxSequenceDepth-1)
	}
}

// A canceled mode inside a macro must still read as a bail to the caller,
// rather than as an ordinary failure it might carry on past.
func TestExecuteActionSequence_PropagatesABailOutOfAMacro(t *testing.T) {
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
	application := macroTestApp(t, recorder, map[string][]string{
		"bails": {stepOne},
	})

	outcome := application.executeActionSequence(
		context.Background(),
		"test",
		[]string{"macro bails", stepTwo},
	)

	if !outcome.bailed || !outcome.stopped {
		t.Fatalf("outcome bailed=%v stopped=%v, want both", outcome.bailed, outcome.stopped)
	}

	if got := recorder.recorded(); !slices.Equal(got, []string{stepOne}) {
		t.Fatalf("dispatched %v, want the sequence to stop after the macro", got)
	}
}

// The bail directive is consumed before the call is parsed, so it must not be
// mistaken for one of the macro's arguments.
func TestExecuteActionSequence_BailDirectiveIsNotAMacroArgument(t *testing.T) {
	recorder := &stepRecorder{
		respond: func(_ context.Context, _ string) ipc.Response {
			return ipc.Response{Success: false, Message: failureMessage, Code: ipc.CodeActionFailed}
		},
	}
	application := macroTestApp(t, recorder, map[string][]string{
		"needs_two": {"step $1 $2"},
	})

	outcome := application.executeActionSequence(
		context.Background(),
		"test",
		[]string{"macro needs_two alpha beta " + bailOnErrorFlag, stepThree},
	)

	// The call is well-formed: it is the macro's own step that fails, and the
	// directive makes that failure end the calling sequence.
	if !outcome.stopped {
		t.Fatalf("outcome = %+v, want the directive to stop the sequence", outcome)
	}

	want := []string{"step alpha beta"}
	if got := recorder.recorded(); !slices.Equal(got, want) {
		t.Fatalf("dispatched %v, want %v", got, want)
	}
}
