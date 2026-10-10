package app_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/event"
)

// hookScrollStep is a hook step a journey can see run: the accessibility fake
// records the scroll.
const hookScrollStep = "action scroll_down"

// goosWindows is runtime.GOOS on Windows, whose shell the record steps change.
const goosWindows = "windows"

// TestSimulation_HookRunsWhenItsEventFires is the hook journey at its
// simplest: entering a mode runs the steps written for it.
func TestSimulation_HookRunsWhenItsEventFires(t *testing.T) {
	cfg := simConfig()
	cfg.Hooks.OnModeEnter = config.StringOrStringArray{hookScrollStep}

	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)

	sim.waitFor("hook scrolled", func() bool { return len(sim.ax.recordedScrolls()) > 0 })
}

// TestSimulation_HookDoesNotStartItselfAgain pins the loop guard: a hook that
// enters a mode raises its own event, and that event does not run it again,
// so the journey settles in grid instead of re-entering it forever.
func TestSimulation_HookDoesNotStartItselfAgain(t *testing.T) {
	cfg := simConfig()
	cfg.Hooks.OnModeEnter = config.StringOrStringArray{"grid"}

	sim := newSimHarness(t, cfg, threeButtons(t))
	events := subscribeEvents(t, sim)

	sim.pressHotkey(hintsHotkey)

	events.expect(
		modeEnter("hints"),
		modeExit("hints", event.ExitSwitched),
		modeEnter("grid"),
	)
	events.expectNone(250 * time.Millisecond)

	sim.waitMode(domain.ModeGrid)
}

// TestSimulation_ResumeHookRunsOnceNeruIsRunning pins that the resume hook
// sees Neru running: its event is published just before the resume applies,
// and a mode it opens would otherwise be refused.
func TestSimulation_ResumeHookRunsOnceNeruIsRunning(t *testing.T) {
	cfg := simConfig()
	cfg.Hooks.OnEnable = config.StringOrStringArray{domain.ModeString(domain.ModeScroll)}

	sim := newSimHarness(t, cfg, nil)

	sim.app.SetEnabled(false)
	sim.app.SetEnabled(true)

	sim.waitMode(domain.ModeScroll)
}

// TestSimulation_HooksWaitWhileNeruIsStopped pins that `neru stop` pauses
// hooks too: a focus change while stopped runs nothing, and the next one after
// `neru start` runs its hook.
func TestSimulation_HooksWaitWhileNeruIsStopped(t *testing.T) {
	cfg := simConfig()
	cfg.Hooks.OnAppFocus = config.StringOrStringArray{hookScrollStep}

	sim := newSimHarness(t, cfg, nil)

	sim.app.SetEnabled(false)
	sim.focusApp("Some Editor", "com.example.editor")

	time.Sleep(250 * time.Millisecond)

	if scrolls := sim.ax.recordedScrolls(); len(scrolls) > 0 {
		t.Fatalf("hook ran while Neru was stopped: %v", scrolls)
	}

	sim.app.SetEnabled(true)
	sim.focusApp("Another Editor", "com.example.other")

	sim.waitFor(
		"hook scrolled after start",
		func() bool { return len(sim.ax.recordedScrolls()) > 0 },
	)
}

// recordEnvStep is an exec step that writes the variable name to the file the
// variable outVar names, for the reason writeEnvStep in internal/app/sequence
// gives. On Windows it needs the shell useRecordShell sets.
func recordEnvStep(name, outVar string) string {
	if runtime.GOOS == goosWindows {
		return fmt.Sprintf(
			"exec Set-Content -NoNewline -LiteralPath $env:%s -Value $env:%s",
			outVar,
			name,
		)
	}

	return fmt.Sprintf("exec printf '%%s' \"$%s\" > \"$%s\"", name, outVar)
}

// useRecordShell sets the shell recordEnvStep's steps are written for.
func useRecordShell(cfg *config.Config) {
	if runtime.GOOS == goosWindows {
		cfg.General.ExecShell = filepath.Join(
			os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe",
		)
		cfg.General.ExecShellArgs = []string{"-NoProfile", "-NonInteractive", "-Command"}
	}
}

// TestSimulation_PauseReportsTheModeItClosed pins that `neru stop` with a mode
// open runs that mode's exit hook, so a status bar fed by it does not keep
// showing a mode that has closed.
func TestSimulation_PauseReportsTheModeItClosed(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "with space")

	mkdirErr := os.Mkdir(dir, 0o700)
	if mkdirErr != nil {
		t.Fatalf("creating %s: %v", dir, mkdirErr)
	}

	recorded := filepath.Join(dir, "mode")
	t.Setenv("NERU_TEST_OUT", recorded)

	cfg := simConfig()
	useRecordShell(cfg)
	cfg.Hooks.OnModeExit = config.StringOrStringArray{recordEnvStep("NERU_MODE", "NERU_TEST_OUT")}

	sim := newSimHarness(t, cfg, nil)

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)

	sim.app.SetEnabled(false)

	sim.waitFor("exit hook recorded the closed mode", func() bool {
		got, readErr := os.ReadFile(recorded)

		return readErr == nil &&
			strings.TrimSpace(string(got)) == domain.ModeString(domain.ModeScroll)
	})
}

// TestSimulation_ShutdownDoesNotWaitOnARunningHook pins that a hook still
// running when the daemon quits does not hold the quit up: its steps run on
// the root context shutdown cancels.
//
// The timing is read around the harness's own shutdown, from a t.Cleanup
// registered after the harness's (so LIFO runs it first) to one registered
// before it (run last), the way the shutdown journey reads its order.
func TestSimulation_ShutdownDoesNotWaitOnARunningHook(t *testing.T) {
	var started time.Time

	t.Cleanup(func() {
		if elapsed := time.Since(started); elapsed > simShutdownBudget {
			t.Errorf(
				"shutdown took %v with a hook running, want under %v",
				elapsed,
				simShutdownBudget,
			)
		}
	})

	cfg := simConfig()
	cfg.Hooks.OnModeEnter = config.StringOrStringArray{"action wait_for_mode_exit"}

	sim := newSimHarness(t, cfg, nil)

	t.Cleanup(func() { started = time.Now() })

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)
}

// TestSimulation_MissionControlRunsBothHookTables pins the deprecation's
// promise: until v2 a Mission Control transition runs the [hooks] steps and
// the deprecated [hints] ones, each once.
func TestSimulation_MissionControlRunsBothHookTables(t *testing.T) {
	cfg := simConfig()
	cfg.Hints.DetectMissionControl = true
	cfg.Hints.IncludeDockHints = true
	cfg.Hooks.OnMissionControlActivated = config.StringOrStringArray{hookScrollStep}
	cfg.Hints.OnMissionControlActivated = config.StringOrStringArray{"action scroll_up"}

	sim := newSimHarness(t, cfg, nil)
	events := subscribeEvents(t, sim)

	sim.watcher.EmitMissionControlActivated()

	events.expect(event.Event{Name: event.MissionControlActivated})
	sim.waitFor("both hooks scrolled", func() bool { return len(sim.ax.recordedScrolls()) == 2 })

	scrolls := sim.ax.recordedScrolls()
	if scrolls[0].Y*scrolls[1].Y >= 0 {
		t.Errorf("scrolls = %v, want one down from [hooks] and one up from [hints]", scrolls)
	}
}

// TestSimulation_ReadyHookRunsOnceTheDaemonIsUp pins that on_ready runs at
// start with nothing else to set it off.
func TestSimulation_ReadyHookRunsOnceTheDaemonIsUp(t *testing.T) {
	cfg := simConfig()
	cfg.Hooks.OnReady = config.StringOrStringArray{hookScrollStep}

	sim := newSimHarness(t, cfg, nil)

	sim.waitFor("ready hook scrolled", func() bool { return len(sim.ax.recordedScrolls()) > 0 })
}

// TestSimulation_QuitHookFinishesBeforeShutdown pins that on_quit runs to the
// end while the daemon shuts down, rather than being cut off by the shutdown
// it reports.
//
// The check is registered before the harness, so it runs after the harness
// has shut the app down.
func TestSimulation_QuitHookFinishesBeforeShutdown(t *testing.T) {
	recorded := filepath.Join(t.TempDir(), "event")
	t.Setenv("NERU_TEST_OUT", recorded)

	t.Cleanup(func() {
		got, readErr := os.ReadFile(recorded)
		if readErr != nil || string(got) != string(event.Quit) {
			t.Errorf("quit hook recorded %q (%v), want %q", got, readErr, event.Quit)
		}
	})

	cfg := simConfig()
	useRecordShell(cfg)
	cfg.Hooks.OnQuit = config.StringOrStringArray{
		hookScrollStep,
		recordEnvStep("NERU_EVENT", "NERU_TEST_OUT"),
	}

	sim := newSimHarness(t, cfg, nil)

	// Registered after the harness, so it runs first, before the app is shut
	// down: the scroll step was skipped, because only exec steps run on quit.
	t.Cleanup(func() {
		sim.app.Cleanup()

		if scrolls := sim.ax.recordedScrolls(); len(scrolls) > 0 {
			t.Errorf("quit hook ran a step that is not exec: %v", scrolls)
		}
	})
}

// TestSimulation_QuitHookCannotHoldShutdownUp pins the bound on the quit
// hook: a step whose shell leaves a child holding its output runs until the
// child exits, and shutdown goes on without it after one second.
func TestSimulation_QuitHookCannotHoldShutdownUp(t *testing.T) {
	if runtime.GOOS == goosWindows {
		t.Skip("backgrounds a child with POSIX shell syntax")
	}

	cfg := simConfig()
	cfg.Hooks.OnQuit = config.StringOrStringArray{"exec sleep 10 &"}

	sim := newSimHarness(t, cfg, nil)

	t.Cleanup(func() {
		started := time.Now()

		sim.app.Cleanup()

		if elapsed := time.Since(started); elapsed > 3*time.Second {
			t.Errorf("shutdown took %v behind the quit hook, want about one second", elapsed)
		}
	})
}

// TestSimulation_StickyModifiersHookSeesTheSetHeld pins the variable a sticky
// modifier hook reads, in the spelling a --modifier flag takes.
func TestSimulation_StickyModifiersHookSeesTheSetHeld(t *testing.T) {
	recorded := filepath.Join(t.TempDir(), "modifiers")
	t.Setenv("NERU_TEST_OUT", recorded)

	cfg := simConfig()
	useRecordShell(cfg)
	cfg.Hooks.OnStickyModifiers = config.StringOrStringArray{
		recordEnvStep("NERU_MODIFIERS", "NERU_TEST_OUT"),
	}

	sim := newSimHarness(t, cfg, threeButtons(t))

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.press("__modifier_shift_down", "__modifier_shift_up")

	sim.waitFor("hook recorded the sticky set", func() bool {
		got, readErr := os.ReadFile(recorded)

		return readErr == nil && string(got) == "shift"
	})
}
