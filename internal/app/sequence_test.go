package app

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/ipcctrl"
	"github.com/y3owk1n/neru/internal/app/sequence"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// sequenceTestCommand is the fake IPC command the sequence test dispatches, so
// a step exercises the real executeHotkeyAction path without depending on any
// service.
const sequenceTestCommand = "step"

// stepOne is the first step the sequence tests dispatch.
const stepOne = "step one"

// stepRecorder answers the fake step command, recording every step a sequence
// dispatched. A step equal to fail is answered with a failure.
type stepRecorder struct {
	steps []string
	fail  string
	mu    sync.Mutex
}

func (r *stepRecorder) handle(_ context.Context, cmd ipc.Command) ipc.Response {
	step := strings.TrimSpace(cmd.Action + " " + strings.Join(cmd.Args, " "))

	r.mu.Lock()
	r.steps = append(r.steps, step)
	r.mu.Unlock()

	if step == r.fail {
		return ipc.Response{Success: false, Message: "boom", Code: ipc.CodeActionFailed}
	}

	return ipc.Response{Success: true, Message: "ok", Code: ipc.CodeOK}
}

func (r *stepRecorder) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.steps)
}

// newSequenceTestApp builds a whitebox App whose IPC controller answers the
// fake step command with the recorder. Everything the executor touches, the
// logger, the app context and the controller, is real. Only the command the
// steps name is fake. Initialization never ran, so the App has no executor of
// its own yet.
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
		ipcController: ipcctrl.New(ipcctrl.Deps{
			ConfigService: loader.NewService(cfg, "", logger, nil),
			AppState:      appState,
			Config:        cfg,
			Logger:        logger,
		}),
	}

	application.ipcController.Handlers[sequenceTestCommand] = recorder.handle

	return application
}

// TestExecuteActionSequence_RunsEveryStepInOrder pins the fallback a sequence
// takes before startup has built the executor. The App builds one from its own
// controller and runs the steps through it. Executor semantics are tested in
// internal/app/sequence.
func TestExecuteActionSequence_RunsEveryStepInOrder(t *testing.T) {
	recorder := &stepRecorder{}
	application := newSequenceTestApp(t, recorder)

	steps := []string{stepOne, "step two", "step three"}

	outcome := application.sequences().Run(context.Background(), "test", steps)

	if outcome.Err != nil {
		t.Fatalf("outcome.Err = %v, want nil", outcome.Err)
	}

	if outcome.Executed != len(steps) {
		t.Fatalf("executed = %d, want %d", outcome.Executed, len(steps))
	}

	if got := recorder.recorded(); !slices.Equal(got, steps) {
		t.Fatalf("dispatched %v, want %v", got, steps)
	}
}

// TestExecuteActionSequence_StopOnErrorPolicyMarksEveryStep pins that the
// App wrapper hands the caller's policy to the executor, so `run
// --stop-on-error` stops at the first failing step.
func TestExecuteActionSequence_StopOnErrorPolicyMarksEveryStep(t *testing.T) {
	recorder := &stepRecorder{fail: stepOne}
	application := newSequenceTestApp(t, recorder)

	outcome := application.executeActionSequenceWithPolicy(
		context.Background(),
		"test",
		[]string{stepOne, "step two"},
		sequence.Policy{StopOnError: true},
	)

	if !outcome.Stopped || outcome.Executed != 1 {
		t.Fatalf("stopped = %v after %d steps, want a stop at the first",
			outcome.Stopped, outcome.Executed)
	}
}
