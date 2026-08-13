package keybinding

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/ports"
)

// ModeBindings reports what the active mode does with a key, and clears
// modifiers left held by the hotkey that opened it.
//
// It is declared here, at the consumer, rather than taken from the component
// that satisfies it, and names only the two methods binding hotkeys calls. That
// is what let this package leave internal/app: the App passes its mode handler
// without dragging the package along. The other interfaces here follow suit.
type ModeBindings interface {
	// ModeHotkeyOverride reports the actions the active mode binds to key, and
	// whether it binds it at all.
	ModeHotkeyOverride(key string) ([]string, bool)
	// SuppressModifiersForHotkey stops modifiers still physically held from
	// leaking into the mode a hotkey just opened.
	SuppressModifiersForHotkey(mods action.Modifiers)
}

// EnabledState is the part of the application state that decides whether
// hotkeys should be registered at all.
type EnabledState interface {
	IsEnabled() bool
	HotkeysRegistered() bool
	SetHotkeysRegistered(registered bool)
}

// FocusedApp reports the frontmost application, which selects the per-app
// hotkey table.
type FocusedApp interface {
	FocusedAppBundleID(ctx context.Context) (string, error)
}

// Deps collects what binding hotkeys needs.
type Deps struct {
	// Manager is the platform hotkey backend.
	Manager ports.HotkeyPort
	// Modes resolves per-mode overrides. May be nil before the mode handler
	// exists, in which case every key falls through to its global binding.
	Modes ModeBindings
	// State gates registration on the daemon being enabled.
	State EnabledState
	// FocusedApp selects the per-app table. May be nil, which means the global
	// table is always used.
	FocusedApp FocusedApp
	// Config reads the live configuration, so a reload changes the next
	// registration rather than needing the binder rebuilt.
	Config func() *config.Config
	// RunSequence executes a binding's steps. It is the sequence executor's
	// RunAndForget: hotkeys have nobody to report an outcome to.
	RunSequence func(source string, steps []string)
	// PublishRegisteredHotkeys reports the chords the platform backend actually
	// took, every time the table is rebuilt. May be nil.
	//
	// The event taps need it because two of them hand a registered chord back to
	// the mechanism that owns it rather than dispatching it themselves (macOS in
	// its tap, Windows in its hook), and a chord the backend *refused* is owned by
	// nobody: handing that one back drops it, where dispatching it at least lets
	// the mode handler resolve it. Registration is the only place that knows which
	// is which — the configured table cannot tell, because a refusal is logged and
	// skipped (registerHotkeys).
	PublishRegisteredHotkeys func(keys []string)
	// Context bounds held-key repeat, so shutdown stops a repeating key.
	Context func() context.Context

	Logger *zap.Logger
}

// Binder owns hotkey registration and the held-key repeat loop.
//
// The repeat state below — the cancel table and the two mutexes — is private to
// this file. A repeating key is a goroutine per key that has to be cancellable
// from the event-tap thread, which is what the table and its lock are for.
type Binder struct {
	hotkeyManager     ports.HotkeyPort
	modes             ModeBindings
	appState          EnabledState
	actionService     FocusedApp
	configSnapshot    func() *config.Config
	runActionSequence func(source string, steps []string)
	publishRegistered func(keys []string)
	ctx               func() context.Context
	logger            *zap.Logger

	// hotkeyRepeatMu guards the repeat cancel table against the event-tap
	// thread and the repeat goroutines.
	hotkeyRepeatMu      sync.Mutex
	hotkeyRepeatCancels map[string]context.CancelFunc

	// hotkeyRegistrationMu serializes re-registration, which can be triggered
	// concurrently by a focus change, a config reload and the systray.
	hotkeyRegistrationMu  sync.Mutex
	currentHotkeyBundleID string
}

// New creates a hotkey binder. A nil logger is replaced with a no-op.
func New(deps Deps) *Binder {
	logger := deps.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Binder{
		hotkeyManager:       deps.Manager,
		modes:               deps.Modes,
		appState:            deps.State,
		actionService:       deps.FocusedApp,
		configSnapshot:      deps.Config,
		runActionSequence:   deps.RunSequence,
		publishRegistered:   deps.PublishRegisteredHotkeys,
		ctx:                 deps.Context,
		logger:              logger.Named("app.hotkey"),
		hotkeyRepeatCancels: make(map[string]context.CancelFunc),
	}
}

// Register registers the hotkeys for the given application bundle identifier.
func (b *Binder) Register(bundleID string) { b.registerHotkeys(bundleID) }

// RefreshFor re-registers hotkeys for an application, or for the currently
// focused one when bundleID is empty.
func (b *Binder) RefreshFor(bundleID string) { b.refreshHotkeysForAppOrCurrent(bundleID) }

// StopAllRepeats cancels every in-flight held-key repeat. Shutdown and mode
// changes both need it.
func (b *Binder) StopAllRepeats() { b.stopAllHotkeyRepeats() }

// Unregister drops every registered hotkey and stops the repeats they own.
//
// It takes the registration lock, so a focus change or a systray toggle cannot
// re-register underneath a config reload that is midway through tearing the
// table down.
func (b *Binder) Unregister() {
	b.hotkeyRegistrationMu.Lock()
	defer b.hotkeyRegistrationMu.Unlock()

	b.unregisterLocked()
}

// ForceRefresh unregisters and registers again for the focused application.
//
// A plain RefreshFor is a no-op when the bundle identifier has not changed,
// which is wrong after a config reload: the bindings changed even though the
// application did not. This drops the table first so the new config always
// takes effect.
func (b *Binder) ForceRefresh() {
	b.Unregister()
	b.RefreshFor("")
}

// Restore re-registers hotkeys if the daemon is enabled and they are currently
// down — the recovery path after a config reload that failed to apply.
func (b *Binder) Restore() {
	b.hotkeyRegistrationMu.Lock()
	defer b.hotkeyRegistrationMu.Unlock()

	if b.appState == nil || !b.appState.IsEnabled() || b.appState.HotkeysRegistered() {
		return
	}

	// Ask which application is focused now rather than trusting the cached
	// identifier: focus can change while the blocking reload alert is up.
	bundleID := ""

	if b.actionService != nil {
		focused, err := b.actionService.FocusedAppBundleID(b.context())
		if err == nil {
			bundleID = focused
		}
	}

	b.registerHotkeys(bundleID)
	b.appState.SetHotkeysRegistered(true)
	b.currentHotkeyBundleID = bundleID

	b.logger.Debug("Hotkeys restored after failed config reload")
}

// Reregister drops the hotkey table and builds it again for the focused
// application, keeping the whole cycle under one lock.
//
// A keyboard layout change needs this: key names like "Space" or "H" map to
// different keycodes afterwards, so every registration is stale at once.
func (b *Binder) Reregister() {
	b.hotkeyRegistrationMu.Lock()
	defer b.hotkeyRegistrationMu.Unlock()

	if b.appState == nil || !b.appState.HotkeysRegistered() {
		b.logger.Debug("Hotkeys not currently registered; skipping re-registration")

		return
	}

	b.unregisterLocked()

	// Ask which application is focused so the [[app_configs]] overrides match
	// the app the user is actually in.
	bundleID := ""

	if b.actionService != nil {
		focused, err := b.actionService.FocusedAppBundleID(b.context())
		if err == nil {
			bundleID = focused
		}
	}

	b.currentHotkeyBundleID = bundleID
	b.registerHotkeys(bundleID)
	b.appState.SetHotkeysRegistered(true)
}

// RunActions dispatches a binding's steps under the given source name, without
// waiting for them. Callers that are not hotkeys use it for the same grammar —
// the Mission Control hooks, for instance.
func (b *Binder) RunActions(source string, actions []string) {
	b.dispatchHotkeyActionsAsync(source, actions)
}

// unregisterLocked drops the hotkey table. The caller holds
// hotkeyRegistrationMu.
func (b *Binder) unregisterLocked() {
	if b.appState == nil || !b.appState.HotkeysRegistered() {
		return
	}

	b.logger.Debug("Unregistering current hotkeys")
	b.stopAllHotkeyRepeats()

	if b.hotkeyManager != nil {
		b.hotkeyManager.UnregisterAll()
	}

	b.appState.SetHotkeysRegistered(false)
}

// context returns the daemon context, or a background one before it exists.
func (b *Binder) context() context.Context {
	if b.ctx == nil {
		return context.Background()
	}

	if ctx := b.ctx(); ctx != nil {
		return ctx
	}

	return context.Background()
}

// settings returns the live configuration, or an empty one if no reader was
// wired, so a missing dependency degrades instead of panicking.
func (b *Binder) settings() *config.Config {
	if b.configSnapshot == nil {
		return &config.Config{}
	}

	if cfg := b.configSnapshot(); cfg != nil {
		return cfg
	}

	return &config.Config{}
}
