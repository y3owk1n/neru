package modes

// The keymap the handler dispatches keys against, and the focused app it is
// settled for.
//
// A keystroke consults a keymap; it never builds one and it never asks the
// operating system anything. What settles one is a change of mode, of focused
// app, or of configuration — and the focused app is learned by being told,
// which is what keeps the keystroke path free of a call that can block on
// another process. ADR 0005 records the decision and the alternatives it turned
// down.
//
// Entering a mode and replacing the configuration settle where they happen,
// because both are entry points the handler takes anyway and both are moments
// the user is already waiting on. A focused-app change settles on the next read
// instead, from what the watcher published — which is why no keystroke has
// anything to ask.

import (
	"sync/atomic"

	"go.uber.org/zap"

	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
)

// focusedAppCell holds the focused app the application watcher last announced.
//
// It is written without any lock and read under h.mu, which is the whole point
// of it: the watcher runs its callbacks inline under its own read lock, and on
// macOS the notification arrives on the main queue, where taking h.mu is
// forbidden (internal/app/modes/AGENTS.md). An atomic cell is what lets the
// publisher and the handler meet without one waiting on the other.
//
// What it holds is for settling the keymap, and is stale by however long the
// watcher took to notice. A caller that needs the truth about the focused app
// asks the accessibility service with its own context, as hint collection and
// the hints debug probe do.
type focusedAppCell struct {
	// identifier is nil until something publishes. A platform with no focus
	// watcher never does; macOS publishes when the user first switches
	// application, and the Linux watcher samples once as it starts.
	//
	// An empty identifier is a legitimate publication, not an absent one: an
	// application the platform cannot name is one no per-app override can
	// match, and saying so beats leaving the app before it in force.
	identifier atomic.Pointer[string]
}

// publish records the focused app. It takes no lock — see the type comment
// before making it take one.
//
// A handler assembled without a cell (only a test does that) publishes
// nothing, which reads downstream as an app nobody has announced.
func (c *focusedAppCell) publish(focusedApp string) {
	if c == nil {
		return
	}

	c.identifier.Store(&focusedApp)
}

// published reports the focused app last published, and false when nothing has
// published one.
func (c *focusedAppCell) published() (string, bool) {
	if c == nil {
		return "", false
	}

	focusedApp := c.identifier.Load()
	if focusedApp == nil {
		return "", false
	}

	return *focusedApp, true
}

// PublishFocusedApp tells the handler which application the operating system
// now routes keystrokes to, so the keymap can be settled against it without
// anything asking the platform on a keystroke.
//
// It takes no lock, and that is a requirement rather than an optimization. The
// application watcher calls this inline under its own read lock and, on macOS,
// on the main queue — and nothing running on the main queue may take h.mu, or
// the dispatch_sync this handler makes while holding it deadlocks
// (internal/app/modes/AGENTS.md). So this writes an atomic cell and the handler
// settles the keymap lazily, inside a locked entry point it was going to enter
// anyway. Do not turn it into a locked setter.
func (h *Handler) PublishFocusedApp(focusedApp string) {
	h.focusedApp.publish(focusedApp)
}

// keymapInputs is everything the settled keymap depends on: change one of them
// and the bindings in force can differ, change none and the keymap in hand is
// still the answer. It is comparable, so settling is one equality rather than
// an invalidation flag per trigger.
type keymapInputs struct {
	mode domain.Mode
	// name is the declared mode a ModeCustom session is in, and empty
	// otherwise. Two declared modes share the enum value and bind different
	// tables, so the name is part of what the keymap is settled for.
	name   string
	config *configpkg.Config
	// focusedApp is the app whose overrides apply, and mustAsk says settling
	// has to learn it from the platform because nothing published it. It stays
	// false when the active mode declares no per-app overrides, because then no
	// answer could change what is bound and there is nothing to learn.
	//
	// Both are part of the comparison on purpose: while nothing publishes, the
	// inputs stay equal and the answer already asked for stands, so a keystroke
	// never asks again — see resolveFocusedApp.
	focusedApp string
	mustAsk    bool
}

// keymapModeName is the name the configuration files a mode's table under: the
// mode's own for a built-in mode, the declared name for a custom one.
func (inputs keymapInputs) keymapModeName() string {
	if inputs.mode == domain.ModeCustom {
		return inputs.name
	}

	return domain.ModeString(inputs.mode)
}

// settledKeymap returns the mode's own bindings in force, settling them first if
// the mode, the focused app or the configuration has changed since they last
// were.
//
// This is what key dispatch consults first, and it is deliberately the only
// place the three merge sites became: whoever needs to know what a mode binds
// reads this rather than merging a copy of their own.
//
// Caller must hold h.mu.
func (h *handlerState) settledKeymap() configpkg.Keymap {
	h.settleKeymaps()

	return h.keymap
}

// settledKeymaps returns both tables in force: the active mode's own, and the
// global chords it falls back to.
//
// The first return is the active mode's own table and the second is the global
// one it falls back to, in the order dispatch asks them.
//
// It exists so a caller that needs the pair settles once. Key dispatch is that
// caller, and asking twice put a second keymapInputs — an [[app_configs]] scan
// and a mode-extension assertion among it — on every keystroke, which is the
// kind of cost the event tap's position on every keystroke makes it wrong to pay
// (root AGENTS.md, Product Direction).
//
// Caller must hold h.mu.
func (h *handlerState) settledKeymaps() (configpkg.Keymap, configpkg.Keymap) {
	h.settleKeymaps()

	return h.keymap, h.globalHotkeys
}

// settleKeymaps settles both tables in force — the mode's own and the global
// fallback — if any of their shared inputs has changed since they last were.
//
// One settling for the two of them is what keeps the focused app resolved once:
// asking the platform for it is the call ADR 0005 keeps off the keystroke path,
// so a second table may not mean a second ask.
//
// Caller must hold h.mu.
func (h *handlerState) settleKeymaps() {
	inputs := h.keymapInputs()
	if h.keymapSettled && h.keymapSettledFor == inputs {
		return
	}

	h.keymap = configpkg.Keymap{}
	h.globalHotkeys = configpkg.Keymap{}

	if inputs.config != nil {
		focusedApp := h.resolveFocusedApp(inputs)

		h.keymap = inputs.config.ResolveKeymap(inputs.keymapModeName(), focusedApp)

		// Idle takes no fallback. Nothing is captured there, so the platform's
		// own hotkey mechanism is what runs a global binding — and a key fed
		// over IPC into idle must not fire one behind its back.
		if inputs.mode != domain.ModeIdle {
			h.globalHotkeys = inputs.config.ResolveGlobalKeymap(focusedApp).ModifierChords()
		}
	}

	h.keymapSettledFor = inputs
	h.keymapSettled = true

	h.logger.Debug("Keymap settled",
		zap.String("mode", domain.ModeString(inputs.mode)),
		zap.Int("binding_count", h.keymap.Len()),
		zap.Int("global_fallback_count", h.globalHotkeys.Len()),
		zap.Bool("asked_the_platform", inputs.mustAsk))
}

// keymapInputs reads what the keymap depends on. It asks the platform nothing,
// so it is safe to call on every keystroke: that is what makes "has anything
// changed" cheap enough to answer before every dispatch.
//
// Caller must hold h.mu.
func (h *handlerState) keymapInputs() keymapInputs {
	inputs := keymapInputs{
		mode:   h.appState.CurrentMode(),
		name:   h.customModeName,
		config: h.config,
	}

	if !h.focusedAppCanChangeWhatIsBound(inputs.mode) {
		// No override can apply, so which application is focused cannot change
		// what is bound and is not worth learning.
		return inputs
	}

	focusedApp, published := h.focusedApp.published()
	inputs.focusedApp = focusedApp
	inputs.mustAsk = !published

	return inputs
}

// focusedAppCanChangeWhatIsBound reports whether which application is focused
// can change the answer a keystroke gets.
//
// Two tables are in force while a mode is open — the mode's own and the global
// one it falls back to — and either of them carrying a per-app override is
// enough to make the focused app worth learning. The global half is asked only
// outside idle, because that is where the fallback exists at all.
//
// Caller must hold h.mu.
func (h *handlerState) focusedAppCanChangeWhatIsBound(mode domain.Mode) bool {
	if h.activeModeHasAppHotkeyOverrides() {
		return true
	}

	return mode != domain.ModeIdle &&
		h.config != nil &&
		h.config.HasGlobalAppHotkeyOverrides()
}

// resolveFocusedApp answers which application's overrides the keymap is being
// settled for: the published cell when something has fed it, the platform when
// nothing has.
//
// Asking happens only here, and only while a keymap settles for a mode being
// entered or a configuration being replaced — never on a keystroke, which is
// the whole point. A keystroke can only re-settle from something the watcher
// published, and something published is never asked for. Where a watcher exists
// the platform is asked at most until it first fires; where none does — a Linux
// session whose compositor exposes no focused-app source — it is asked once per
// mode opened, and there overrides settle when the mode opens rather than the
// instant the user switches apps.
//
// Caller must hold h.mu.
func (h *handlerState) resolveFocusedApp(inputs keymapInputs) string {
	if !inputs.mustAsk {
		return inputs.focusedApp
	}

	if h.actionService == nil {
		return ""
	}

	focusedApp, err := h.actionService.FocusedAppBundleID(h.ctx)
	if err != nil {
		h.logger.Debug("Failed to get the focused app for the keymap", zap.Error(err))

		return ""
	}

	return focusedApp
}
