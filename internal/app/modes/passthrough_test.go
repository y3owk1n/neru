package modes

import (
	"slices"
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

func TestModeModifierKeys_HintsIncludesModifierHotkeys(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.Hotkeys = map[string]config.StringOrStringArray{
		"Cmd+L": {stepLeftClick},
		"Alt+K": {"action move_mouse_relative --dx=0 --dy=-10"},
		"k":     {"action scroll_up"},
	}

	got := modeModifierKeys(cfg.ResolveKeymap(config.ModeNameHints, ""), config.Keymap{})
	want := []string{
		config.CanonicalHotkeyForPlatform("Alt+K"),
		config.CanonicalHotkeyForPlatform("Cmd+L"),
	}

	if !slices.Equal(got, want) {
		t.Fatalf("modeModifierKeys(ModeHints) = %v, want %v", got, want)
	}
}

func TestModeModifierKeys_ScrollIncludesOnlyModifierHotkeys(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Scroll.Hotkeys = map[string]config.StringOrStringArray{
		"k":        {"action scroll_up"},
		"Cmd+Up":   {"action go_top"},
		"Cmd+Down": {"action go_bottom"},
		"gg":       {"action go_top"},
	}

	got := modeModifierKeys(cfg.ResolveKeymap(config.ModeNameScroll, ""), config.Keymap{})
	want := []string{
		config.CanonicalHotkeyForPlatform("Cmd+Down"),
		config.CanonicalHotkeyForPlatform("Cmd+Up"),
	}

	if !slices.Equal(got, want) {
		t.Fatalf("modeModifierKeys(ModeScroll) = %v, want %v", got, want)
	}
}

func TestHandlePassthroughLocked_IgnoresStaleSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.General.PassthroughUnboundedKeys = true

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	handler := newHandlerWithState(handlerState{
		config:      cfg,
		logger:      zap.NewNop(),
		appState:    appState,
		modeSession: 2,
	})

	handler.passthroughTick(domain.ModeHints, 1)

	if handler.refreshHintsTimer != nil {
		t.Fatal("expected stale passthrough callback to be ignored")
	}
}

func TestPassthroughCallbackFor_CapturesModeSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.General.PassthroughUnboundedKeys = true

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	handler := newHandlerWithState(handlerState{
		config:      cfg,
		logger:      zap.NewNop(),
		appState:    appState,
		modeSession: 1,
	})

	callback := handler.passthroughCallbackFor(domain.ModeHints, true)
	if callback == nil {
		t.Fatal("expected passthrough callback")
	}

	handler.modeSession = 2

	appState.SetMode(domain.ModeGrid)

	callback()

	if handler.refreshHintsTimer != nil {
		t.Fatal("expected captured passthrough callback to ignore a later mode session")
	}
}

func TestHandlePassthroughLocked_SchedulesHintRefreshForMatchingSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.General.PassthroughUnboundedKeys = true

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	handler := newHandlerWithState(handlerState{
		config:      cfg,
		logger:      zap.NewNop(),
		appState:    appState,
		modeSession: 3,
	})

	handler.passthroughTick(domain.ModeHints, 3)

	if handler.refreshHintsTimer == nil {
		t.Fatal("expected matching passthrough callback to schedule a hint refresh")
	}

	handler.refreshHintsTimer.Stop()
}

// The bug in #1252 and the tests that pin it.
//
// Switching applications mid-mode changed which bindings the next keystroke
// resolved against but not which chords the event tap let through, so the mode
// consumed the keys the application the user left had bound and passed through
// the ones the application they arrived in binds. The issue names every mode
// that takes per-app overrides, so these run over all four rather than over the
// one the reproduction happened to use.

const (
	// The action steps the fixture bindings run. Which one a key runs is never
	// asserted here — what matters is that a binding exists to be routed.
	stepLeftClick  = "action left_click"
	stepRightClick = "action right_click"

	passthroughFirstApp    = "com.apple.Safari"
	passthroughFirstChord  = "Cmd+Ctrl+G"
	passthroughSecondApp   = "com.apple.finder"
	passthroughSecondChord = "Cmd+Ctrl+H"
)

// passthroughModes is every mode that accepts per-app hotkey overrides, paired
// with the entry point that opens it and the section its overrides live in.
var passthroughModes = []struct {
	name       string
	enter      func(*Handler)
	appConfigs func(*config.Config, []config.AppConfig)
}{
	{
		name:  config.ModeNameHints,
		enter: (*Handler).SetModeHints,
		appConfigs: func(cfg *config.Config, apps []config.AppConfig) {
			cfg.Hints.AppConfigs = apps
		},
	},
	{
		name:  config.ModeNameGrid,
		enter: (*Handler).SetModeGrid,
		appConfigs: func(cfg *config.Config, apps []config.AppConfig) {
			cfg.Grid.AppConfigs = apps
		},
	},
	{
		name:  config.ModeNameRecursiveGrid,
		enter: (*Handler).SetModeRecursiveGrid,
		appConfigs: func(cfg *config.Config, apps []config.AppConfig) {
			cfg.RecursiveGrid.AppConfigs = apps
		},
	},
	{
		name:  config.ModeNameScroll,
		enter: (*Handler).SetModeScroll,
		appConfigs: func(cfg *config.Config, apps []config.AppConfig) {
			cfg.Scroll.AppConfigs = apps
		},
	},
}

// perAppPassthroughConfig puts a different modifier chord in force in each of
// two applications, on every mode that takes per-app overrides, so the lists
// pushed into the event tap say which application's overrides they were built
// from whichever mode is open.
func perAppPassthroughConfig() *config.Config {
	cfg := &config.Config{
		General: config.GeneralConfig{PassthroughUnboundedKeys: true},
	}

	apps := []config.AppConfig{
		{
			BundleID: passthroughFirstApp,
			Hotkeys: map[string]config.StringOrStringArray{
				passthroughFirstChord: {stepLeftClick},
			},
		},
		{
			BundleID: passthroughSecondApp,
			Hotkeys: map[string]config.StringOrStringArray{
				passthroughSecondChord: {stepRightClick},
			},
		},
	}

	for _, mode := range passthroughModes {
		mode.appConfigs(cfg, apps)
	}

	return cfg
}

// newPassthroughHandler builds a handler with the event tap wired up and the
// given application published as focused, then opens mode through enter, so the
// tap holds the lists mode entry pushed into it.
func newPassthroughHandler(
	t *testing.T,
	cfg *config.Config,
	focusedApp string,
	enter func(*Handler),
) (*Handler, *portmocks.MockEventTapPort) {
	t.Helper()

	handler := newHandlerWithState(handlerState{
		config:                cfg,
		logger:                zap.NewNop(),
		appState:              state.NewAppState(),
		modifierState:         state.NewModifierState(),
		executeActionSequence: func(string, []string) {},
	})

	tap := &portmocks.MockEventTapPort{}
	handler.SetEventTap(tap)

	handler.PublishFocusedApp(focusedApp)
	enter(handler)

	return handler, tap
}

func TestRefreshPassthroughForFocusedAppChange_RetargetsTheEventTap(t *testing.T) {
	t.Parallel()

	const unboundChord = "Cmd+Ctrl+J"

	canonical := config.CanonicalHotkeyForPlatform

	for _, mode := range passthroughModes {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()

			handler, tap := newPassthroughHandler(
				t,
				perAppPassthroughConfig(),
				passthroughFirstApp,
				mode.enter,
			)

			_, blacklist := tap.ModifierPassthrough()
			if !slices.Contains(blacklist, canonical(passthroughFirstChord)) {
				t.Fatalf(
					"opening the mode in the first application blacklisted %v, want %q among them",
					blacklist, canonical(passthroughFirstChord),
				)
			}

			handler.PublishFocusedApp(passthroughSecondApp)
			handler.RefreshPassthroughForFocusedAppChange()

			_, blacklist = tap.ModifierPassthrough()
			if !slices.Contains(blacklist, canonical(passthroughSecondChord)) {
				t.Errorf(
					"after the switch the blacklist is %v, want %q among them: the chord the "+
						"newly focused application binds is passed to it instead of being consumed",
					blacklist, canonical(passthroughSecondChord),
				)
			}

			if slices.Contains(blacklist, canonical(passthroughFirstChord)) {
				t.Errorf(
					"after the switch the blacklist is %v, want %q gone: nothing binds it any "+
						"more and the mode still consumes it",
					blacklist, canonical(passthroughFirstChord),
				)
			}

			want := []string{canonical(passthroughSecondChord)}
			if got := tap.InterceptedModifierKeys(); !slices.Equal(got, want) {
				t.Errorf("intercepted modifier keys = %v, want %v", got, want)
			}

			if slices.Contains(blacklist, canonical(unboundChord)) {
				t.Errorf("blacklist %v contains a chord neither application binds", blacklist)
			}
		})
	}
}

// TestRefreshPassthroughForFocusedAppChange_LeavesTheTapAlone covers the three
// ways a focus change cannot have moved either list. Each of them is a push
// into the event tap on every application switch that would buy nothing, and
// the tap is on the path of every keystroke.
func TestRefreshPassthroughForFocusedAppChange_LeavesTheTapAlone(t *testing.T) {
	t.Parallel()

	withoutOverrides := func() *config.Config {
		cfg := perAppPassthroughConfig()
		for _, mode := range passthroughModes {
			mode.appConfigs(cfg, nil)
		}

		cfg.Grid.Hotkeys = map[string]config.StringOrStringArray{
			passthroughFirstChord: {stepLeftClick},
		}

		return cfg
	}

	withPassthroughOff := func() *config.Config {
		cfg := perAppPassthroughConfig()
		cfg.General.PassthroughUnboundedKeys = false

		return cfg
	}

	tests := []struct {
		name   string
		config func() *config.Config
		// enter is nil where the mode stays idle, which is the case the tap is
		// not even running in.
		enter func(*Handler)
	}{
		{
			name:   "no mode is open",
			config: perAppPassthroughConfig,
		},
		{
			name:   "the active mode declares no per-app overrides",
			config: withoutOverrides,
			enter:  (*Handler).SetModeGrid,
		},
		{
			name:   "passthrough is switched off",
			config: withPassthroughOff,
			enter:  (*Handler).SetModeGrid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			enter := test.enter
			if enter == nil {
				enter = func(*Handler) {}
			}

			handler, tap := newPassthroughHandler(
				t,
				test.config(),
				passthroughFirstApp,
				enter,
			)

			// Counted from here, so mode entry's own pushes are not attributed
			// to the focus change.
			calls := 0
			tap.SetOnCall(func(string) { calls++ })

			handler.PublishFocusedApp(passthroughSecondApp)
			handler.RefreshPassthroughForFocusedAppChange()

			if calls != 0 {
				t.Errorf("an application switch made %d event-tap calls, want none", calls)
			}
		})
	}
}

// TestRefreshPassthroughForFocusedAppChange_IsSafeWhileKeysAreHandled pins the
// concurrency the fix introduces: the app layer starts one of these per
// application activation, so several can be in flight at once while the
// handler is also dispatching keys.
//
// They take no application as an argument — each reads the published cell — so
// the answer they converge on is the last publication, whatever order they
// arrive in. This has to fail under -race if that ever stops being true.
func TestRefreshPassthroughForFocusedAppChange_IsSafeWhileKeysAreHandled(t *testing.T) {
	t.Parallel()

	const (
		switches = 200
		// A key the mode binds in both applications, so dispatching it stays on
		// the keymap path the refresh contends with.
		sharedKey = "j"
	)

	canonical := config.CanonicalHotkeyForPlatform

	cfg := perAppPassthroughConfig()
	cfg.Grid.Hotkeys = map[string]config.StringOrStringArray{
		sharedKey: {stepLeftClick},
	}

	handler, tap := newPassthroughHandler(
		t,
		cfg,
		passthroughFirstApp,
		(*Handler).SetModeGrid,
	)

	var refreshing sync.WaitGroup

	for range 2 {
		refreshing.Go(func() {
			for range switches {
				handler.PublishFocusedApp(passthroughFirstApp)
				handler.RefreshPassthroughForFocusedAppChange()
				handler.PublishFocusedApp(passthroughSecondApp)
				handler.RefreshPassthroughForFocusedAppChange()
			}
		})
	}

	for range switches {
		handler.HandleKeyPress(sharedKey)
	}

	refreshing.Wait()

	// Whichever application is published last is the one both lists describe,
	// and the handler is still answering keys at all.
	handler.PublishFocusedApp(passthroughFirstApp)
	handler.RefreshPassthroughForFocusedAppChange()

	want := []string{canonical(passthroughFirstChord)}
	if got := tap.InterceptedModifierKeys(); !slices.Equal(got, want) {
		t.Errorf("intercepted modifier keys = %v after the switches, want %v", got, want)
	}
}

// The global chords a mode falls back to have to be blacklisted like the mode's
// own keys. Left off the list, the Wayland evdev tap reads them as unbound
// shortcuts and re-injects them into the focused application, so the user's own
// hotkey reaches their editor and the handler never sees the key at all — which
// is the shape the reported --toggle failure took.
func TestSyncModifierPassthrough_ConsumesTheGlobalChordsAModeFallsBackTo(t *testing.T) {
	t.Parallel()

	const globalChord = "Super+;"

	canonical := config.CanonicalHotkeyForPlatform

	cfg := &config.Config{
		General: config.GeneralConfig{PassthroughUnboundedKeys: true},
		Hotkeys: config.HotkeysConfig{
			Bindings: map[string][]string{
				globalChord: {"recursive_grid --toggle"},
				// A bare key is never in force inside a mode, so it must not be
				// consumed on the global table's behalf either.
				"F13": {config.ModeNameGrid},
			},
		},
		RecursiveGrid: config.RecursiveGridConfig{
			Hotkeys: map[string]config.StringOrStringArray{
				passthroughFirstChord: {stepLeftClick},
			},
		},
	}

	_, tap := newPassthroughHandler(t, cfg, "", (*Handler).SetModeRecursiveGrid)

	_, blacklist := tap.ModifierPassthrough()
	for _, want := range []string{canonical(globalChord), canonical(passthroughFirstChord)} {
		if !slices.Contains(blacklist, want) {
			t.Errorf("blacklist is %v, want %q among them", blacklist, want)
		}
	}

	if slices.Contains(blacklist, canonical("F13")) {
		t.Errorf("blacklist %v consumes a bare global key the mode never resolves", blacklist)
	}

	want := []string{canonical(passthroughFirstChord), canonical(globalChord)}
	slices.Sort(want)

	if got := tap.InterceptedModifierKeys(); !slices.Equal(got, want) {
		t.Errorf("intercepted modifier keys = %v, want %v", got, want)
	}
}
