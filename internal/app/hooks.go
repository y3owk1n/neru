package app

import (
	"slices"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/sequence"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/event"
)

const (
	// hookEventBuffer is how many events the runner holds while it is busy.
	// The loop only ever waits on its own bookkeeping and a config read, so a
	// full buffer means a burst far beyond anything a person causes.
	hookEventBuffer = 64

	// hookStopTimeout bounds the wait for hooks still running at shutdown.
	// Their steps run on the root context, which is canceled by then, so they
	// end on their own. The bound is for a step that does not listen.
	hookStopTimeout = time.Second
)

// hookRunner runs the [hooks] steps for each event the bus publishes.
//
// It is one goroutine reading the bus, plus at most one goroutine per hook
// while that hook runs, so a hook that keeps firing never runs twice at once.
type hookRunner struct {
	stop     func()
	loopDone chan struct{}
	running  sync.WaitGroup

	mu    sync.Mutex
	hooks map[event.Name]*hookState
}

// hookState is what the runner remembers about one hook between its events.
type hookState struct {
	busy bool
	// Events numbered in (from, to] were raised while the hook last ran, so
	// they are its own doing and do not start it again.
	from, to uint64
}

// startHooks subscribes the hook runner to the bus. Call it before the
// application watcher starts, so the first focus change has a listener.
func (a *App) startHooks() {
	events, stop := a.events.Subscribe(hookEventBuffer)

	a.hookRunner = &hookRunner{
		stop:     stop,
		loopDone: make(chan struct{}),
		hooks:    make(map[event.Name]*hookState),
	}

	go a.runHooks(a.hookRunner, events)
}

// stopHooks ends the subscription and waits, within hookStopTimeout, for the
// hooks still running.
func (a *App) stopHooks() {
	runner := a.hookRunner
	if runner == nil {
		return
	}

	runner.stop()
	<-runner.loopDone

	done := make(chan struct{})

	go func() {
		runner.running.Wait()
		close(done)
	}()

	timer := time.NewTimer(hookStopTimeout)
	defer timer.Stop()

	select {
	case <-done:
	case <-timer.C:
		a.logger.Warn("Timed out waiting for hooks to finish",
			zap.Duration("waited", hookStopTimeout))
	}
}

func (a *App) runHooks(runner *hookRunner, events <-chan event.Event) {
	defer close(runner.loopDone)

	for evt := range events {
		if evt.Dropped > 0 {
			a.logger.Warn("Hook runner missed events",
				zap.Uint64("dropped", evt.Dropped))
		}

		// Shutdown has begun, so what is still buffered starts nothing. The
		// quit hook is run by the shutdown itself (runQuitHook).
		if a.ctx.Err() != nil || evt.Name == event.Quit {
			continue
		}

		steps := a.configSnapshot().Hooks.Steps(evt.Name)
		if len(steps) == 0 {
			continue
		}

		if !a.hookMayRun(evt.Name) {
			continue
		}

		if !runner.claim(evt) {
			a.logger.Debug("Hook skipped while it was running",
				zap.String("event", string(evt.Name)),
				zap.Uint64("seq", evt.Seq))

			continue
		}

		runner.running.Go(func() {
			if evt.Name == event.Enable {
				a.awaitResume()
			}

			ctx := sequence.WithHook(a.ctx, hookEnv(evt))
			a.executeActionSequenceWithPolicy(
				ctx,
				config.HookField(evt.Name),
				steps,
				sequence.Policy{},
			)

			runner.finish(evt.Name, a.events.Seq())
		})
	}
}

// hookMayRun reports whether the hook for name runs now. While Neru is stopped
// only the pause and resume hooks do, the exit of the mode the pause closed,
// and the daemon's own start and quit, since nothing else should act then. No mode is
// entered while stopped, so that exit is the only one there can be.
func (a *App) hookMayRun(name event.Name) bool {
	switch name {
	case event.Enable, event.Disable, event.ModeExit, event.Ready, event.Quit:
		return true
	case event.ModeEnter, event.AppFocus, event.ConfigReload,
		event.MissionControlActivated, event.MissionControlDeactivated,
		event.ScrollInvert, event.ScreenShareHide, event.CursorSave,
		event.CursorRestore, event.StickyModifiers, event.MonitorMove,
		event.ScreenChange:
		return a.appState.IsEnabled()
	}

	return false
}

// runQuitHook publishes the quit and runs its hook's exec steps before the
// shutdown cancels the root context they would run on. The runner skips the
// event, so the hook runs once. A daemon that never started its hooks never
// got ready, so it reports no quit.
//
// Only exec steps run. Any other step reaches the mode handler, and on macOS
// shutdown runs on the main thread after the Cocoa loop has stopped, where a
// step that waits on the main queue would never return.
//
// A timer bounds the wait at hookStopTimeout. A context deadline would not,
// because a step runs on the daemon's context rather than its caller's
// (sequence.Executor.stepContext). An exec step whose shell leaves a child
// holding its output runs until that child exits, and the process exit ends
// whatever is still running.
func (a *App) runQuitHook() {
	if a.hookRunner == nil {
		return
	}

	evt := event.Event{Name: event.Quit}
	a.events.Publish(evt)

	steps := slices.DeleteFunc(
		slices.Clone(a.configSnapshot().Hooks.Steps(event.Quit)),
		func(step string) bool { return !action.IsExecStep(step) },
	)
	if len(steps) == 0 {
		return
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		a.executeActionSequenceWithPolicy(
			sequence.WithHook(a.ctx, hookEnv(evt)),
			config.HookField(event.Quit),
			steps,
			sequence.Policy{},
		)
	}()

	timer := time.NewTimer(hookStopTimeout)
	defer timer.Stop()

	select {
	case <-done:
	case <-timer.C:
		a.logger.Warn("Timed out waiting for the quit hook",
			zap.Duration("waited", hookStopTimeout))
	}
}

// awaitResume waits for a resume in progress to finish applying. A resume is
// published just before the state flips (setEnabledLocked), so its hook waits
// here for the flip.
//
// Taking enabledMu is the barrier, not a critical section: the resume holds it
// until it has applied, and this holds nothing while it waits. It runs on the
// hook's goroutine, never the runner loop, so a resume that never finishes
// during shutdown falls under hookStopTimeout instead of hanging it.
func (a *App) awaitResume() {
	a.enabledMu.Lock()
	a.enabledMu.Unlock() //nolint:staticcheck // a barrier: wait for the resume to apply
}

// claim marks the hook for evt as running, unless it already is or evt was
// raised while it last ran.
func (r *hookRunner) claim(evt event.Event) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, known := r.hooks[evt.Name]
	if !known {
		state = &hookState{}
		r.hooks[evt.Name] = state
	}

	if state.busy || (evt.Seq > state.from && evt.Seq <= state.to) {
		return false
	}

	state.busy = true
	state.from = evt.Seq

	return true
}

// finish records that the hook for name has stopped, with lastSeq the number
// of the last event published while it ran.
func (r *hookRunner) finish(name event.Name, lastSeq uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state := r.hooks[name]
	state.busy = false
	state.to = lastSeq
}

// hookEnv is the environment a hook's exec steps see: the event's name, and
// each field the event carries.
func hookEnv(evt event.Event) []string {
	env := []string{"NERU_EVENT=" + string(evt.Name)}

	if evt.Mode != "" {
		env = append(env, "NERU_MODE="+evt.Mode)
	}

	if evt.Reason != "" {
		env = append(env, "NERU_REASON="+string(evt.Reason))
	}

	if evt.Action != "" {
		env = append(env, "NERU_ACTION="+evt.Action)
	}

	if evt.BundleID != "" {
		env = append(env, "NERU_BUNDLE_ID="+evt.BundleID)
	}

	if evt.Slot != "" {
		env = append(env, "NERU_SLOT="+evt.Slot)
	}

	if evt.Monitor != "" {
		env = append(env, "NERU_MONITOR="+evt.Monitor)
	}

	if evt.Name == event.ConfigReload {
		env = append(env, "NERU_OK="+strconv.FormatBool(evt.OK))
	}

	if evt.Name == event.ScrollInvert || evt.Name == event.ScreenShareHide {
		env = append(env, "NERU_ON="+strconv.FormatBool(evt.On))
	}

	if evt.Name == event.CursorSave {
		env = append(env,
			"NERU_X="+strconv.Itoa(evt.Point.X),
			"NERU_Y="+strconv.Itoa(evt.Point.Y))
	}

	// Set even when empty, so a hook reads a release as no modifiers.
	if evt.Name == event.StickyModifiers {
		env = append(env, "NERU_MODIFIERS="+evt.Modifiers)
	}

	return env
}
