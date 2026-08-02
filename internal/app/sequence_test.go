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
					Message: "mode exited without selection",
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
