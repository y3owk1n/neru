//nolint:testpackage // Tests private hotkey dispatch/registration behavior.
package app

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/hotkeys"
)

// fakeReleaseHotkeyManager implements both HotkeyService and
// HotkeyReleaseService so registerHotkeys drives the release path, and captures
// the press/release callbacks per key so tests can invoke them.
type fakeReleaseHotkeyManager struct {
	mu         sync.Mutex
	press      map[string]hotkeys.Callback
	release    map[string]hotkeys.Callback
	viaRelease map[string]bool
	nextID     hotkeys.HotkeyID
}

func newFakeReleaseHotkeyManager() *fakeReleaseHotkeyManager {
	return &fakeReleaseHotkeyManager{
		press:      map[string]hotkeys.Callback{},
		release:    map[string]hotkeys.Callback{},
		viaRelease: map[string]bool{},
	}
}

func (f *fakeReleaseHotkeyManager) Register(
	key string, callback hotkeys.Callback,
) (hotkeys.HotkeyID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.nextID++
	f.press[key] = callback
	f.viaRelease[key] = false

	return f.nextID, nil
}

func (f *fakeReleaseHotkeyManager) RegisterWithRelease(
	key string, press, release hotkeys.Callback,
) (hotkeys.HotkeyID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.nextID++
	f.press[key] = press
	f.release[key] = release
	f.viaRelease[key] = true

	return f.nextID, nil
}

func (f *fakeReleaseHotkeyManager) UnregisterAll() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.press = map[string]hotkeys.Callback{}
	f.release = map[string]hotkeys.Callback{}
	f.viaRelease = map[string]bool{}
}

func (f *fakeReleaseHotkeyManager) pressCallback(key string) hotkeys.Callback {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.press[key]
}

func (f *fakeReleaseHotkeyManager) releaseCallback(key string) hotkeys.Callback {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.release[key]
}

func (f *fakeReleaseHotkeyManager) registeredViaRelease(key string) (bool, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	via, ok := f.viaRelease[key]

	return via, ok
}

// newDispatchTestApp builds a whitebox App wired with just enough for the
// hotkey registration/dispatch paths: a config, app state, a nop logger, a
// cancelable context, the given hotkey manager, and a real IPC controller
// backed by nil services (so executeHotkeyAction resolves built-in commands
// without any system dependency). modes is left nil.
func newDispatchTestApp(t *testing.T, cfg *config.Config, hkm HotkeyService) *App {
	t.Helper()

	logger := zap.NewNop()
	appState := state.NewAppState()
	configService := config.NewService(cfg, "", logger, nil)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	return &App{
		ctx:           ctx,
		logger:        logger,
		config:        cfg,
		appState:      appState,
		hotkeyManager: hkm,
		ipcController: NewIPCController(
			nil, // hintService
			nil, // gridService
			nil, // actionService
			nil, // scrollService
			configService,
			appState,
			cfg,
			nil, // modesHandler
			nil, // systemPort
			nil, // eventTap
			nil, // ipcServer
			nil, // reloadConfig
			logger,
		),
	}
}

func (a *App) repeatActive(key string) bool {
	a.hotkeyRepeatMu.Lock()
	defer a.hotkeyRepeatMu.Unlock()

	return a.hotkeyRepeatCancels[key] != nil
}

func TestEffectiveHeldHotkey(t *testing.T) {
	t.Parallel()

	const (
		heldRepeatAction = "action scroll_down"
		modeSwitch       = "recursive_grid"
		modeLaunch       = "hints"
	)

	tests := []struct {
		name        string
		enabled     bool
		hasOverride bool
		override    []string
		global      []string
		wantActions []string
		wantRepeat  bool
	}{
		{
			name:        "no override, held-repeat global, enabled -> repeat global",
			enabled:     true,
			global:      []string{heldRepeatAction},
			wantActions: []string{heldRepeatAction},
			wantRepeat:  true,
		},
		{
			name:        "no override, non-repeat global, enabled -> global once",
			enabled:     true,
			global:      []string{modeLaunch},
			wantActions: []string{modeLaunch},
			wantRepeat:  false,
		},
		{
			name:        "override mode switch beats held-repeat global -> override once",
			enabled:     true,
			hasOverride: true,
			override:    []string{modeSwitch},
			global:      []string{heldRepeatAction},
			wantActions: []string{modeSwitch},
			wantRepeat:  false,
		},
		{
			name:        "held-repeat override beats non-repeat global -> override repeats",
			enabled:     true,
			hasOverride: true,
			override:    []string{heldRepeatAction},
			global:      []string{modeLaunch},
			wantActions: []string{heldRepeatAction},
			wantRepeat:  true,
		},
		{
			name:        "held_repeat disabled -> never repeats",
			enabled:     false,
			global:      []string{heldRepeatAction},
			wantActions: []string{heldRepeatAction},
			wantRepeat:  false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			cfg.HeldRepeat.Enabled = testCase.enabled

			actions, repeat := (&App{}).effectiveHeldHotkey(
				testCase.hasOverride, testCase.override, testCase.global, cfg,
			)

			if !reflect.DeepEqual(actions, testCase.wantActions) {
				t.Fatalf("actions = %v, want %v", actions, testCase.wantActions)
			}

			if repeat != testCase.wantRepeat {
				t.Fatalf("repeat = %v, want %v", repeat, testCase.wantRepeat)
			}
		})
	}
}

func TestRegisterHotkeysUsesReleasePathWhenSupported(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Hotkeys.Bindings = map[string][]string{
		"Ctrl+Alt+J": {actionScrollDown}, // held-repeat action
		"Ctrl+Alt+P": {"ping"},           // non-repeat command
	}

	fake := newFakeReleaseHotkeyManager()
	app := newDispatchTestApp(t, cfg, fake)

	app.registerHotkeys("")

	for rawKey := range cfg.Hotkeys.Bindings {
		key := config.CanonicalHotkeyForPlatform(rawKey)

		via, ok := fake.registeredViaRelease(key)
		if !ok {
			t.Fatalf("key %q was not registered at all", key)
		}

		if !via {
			t.Fatalf("key %q registered via plain Register; want RegisterWithRelease", key)
		}

		if fake.pressCallback(key) == nil {
			t.Fatalf("key %q has no press callback", key)
		}

		if fake.releaseCallback(key) == nil {
			t.Fatalf("key %q has no release callback", key)
		}
	}
}

func TestDispatchModeAwareHeldHotkey_RepeatVsOnce(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.HeldRepeat.Enabled = true
	// Large delays so the repeat goroutine parks before ticking; the test
	// cancels via the release callback (and the deferred ctx cancel) first.
	cfg.HeldRepeat.InitialDelay = 60000
	cfg.HeldRepeat.Interval = 60000
	cfg.Hotkeys.Bindings = map[string][]string{
		"Ctrl+Alt+J": {"action scroll_down"}, // held-repeat -> should repeat
		"Ctrl+Alt+P": {"ping"},               // non-repeat -> single dispatch
	}

	fake := newFakeReleaseHotkeyManager()
	app := newDispatchTestApp(t, cfg, fake)
	app.registerHotkeys("")

	repeatKey := config.CanonicalHotkeyForPlatform("Ctrl+Alt+J")
	onceKey := config.CanonicalHotkeyForPlatform("Ctrl+Alt+P")

	// Held-repeat binding: press starts a repeat, release stops it.
	fake.pressCallback(repeatKey)()

	if !app.repeatActive(repeatKey) {
		t.Fatalf("expected held-repeat to be active after press on %q", repeatKey)
	}

	fake.releaseCallback(repeatKey)()

	if app.repeatActive(repeatKey) {
		t.Fatalf("expected held-repeat to be cleared after release on %q", repeatKey)
	}

	// Non-repeat binding: press dispatches once, never starts a repeat.
	fake.pressCallback(onceKey)()

	if app.repeatActive(onceKey) {
		t.Fatalf("non-repeat binding %q must not start a held-repeat", onceKey)
	}
}
