package modes

import (
	"context"
	"image"
	"runtime/debug"

	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// Compile-time interface compliance checks: the core interface, then every
// optional extension vim mode opts into (extensions.go).
var (
	_ Mode                   = (*VimMode)(nil)
	_ cursorFollowSelector   = (*VimMode)(nil)
	_ themeRefresher         = (*VimMode)(nil)
	_ screenRefresher        = (*VimMode)(nil)
	_ captureScopeReporter   = (*VimMode)(nil)
	_ hotkeyOverrideReporter = (*VimMode)(nil)
)

// VimMode implements the Mode interface for moving over the screen's text
// with vim's keys.
type VimMode struct {
	baseMode
}

// NewVimMode creates a new vim mode implementation.
func NewVimMode(handler *handlerState) *VimMode {
	return &VimMode{
		baseMode: newBaseMode(handler, "VimMode"),
	}
}

// Activate reads the screen and opens vim mode over it.
func (m *VimMode) Activate(activation modecmd.Activation) {
	m.handler.startVim(activation)
}

// HandleKey feeds every key to the vim machine. Vim mode binds no hotkeys, so
// every key reaches it.
func (m *VimMode) HandleKey(key string) {
	m.handler.handleVimKey(key)
}

// RefreshForMonitorMove reads the display the cursor landed on, since the
// text read before is on a screen the user has left.
func (m *VimMode) RefreshForMonitorMove(_ context.Context, _ image.Rectangle) {
	if m.handler.vim != nil {
		m.handler.startVim(m.handler.vimSessionActivation())
	}
}

// RefreshForThemeChange draws the session again so it picks up the colors the
// overlay just re-resolved.
func (m *VimMode) RefreshForThemeChange() bool {
	if m.handler.vim == nil {
		return false
	}

	m.handler.redrawFrame(m.handler.vimFrame(), "refresh vim after theme change")

	return true
}

// RefreshForScreenChange reads the screen again, since the arrangement
// changed under the text. Vim switched off in configuration leaves the
// overlay for the caller to resize.
func (m *VimMode) RefreshForScreenChange(_ context.Context) bool {
	if m.handler.config == nil || !m.handler.config.Vim.Enabled || m.handler.vim == nil {
		return false
	}

	m.handler.startVim(m.handler.vimSessionActivation())

	return true
}

// Exit tears vim mode down. The machine goes with it, so registers and
// macros last one session. A read of a whole screen copies its pixels several
// times, so Exit has the vision port forget the frame it kept, and gives the
// memory the session freed back to the system at once, off the lock. The
// runtime would otherwise take minutes.
func (m *VimMode) Exit() {
	if m.handler.vim != nil {
		m.handler.stopScrolledRead()
		m.handler.cancelYoungRefine()

		if m.handler.vision != nil {
			m.handler.vision.ForgetLastRead()
		}

		go debug.FreeOSMemory()
	}

	m.handler.vim = nil
	m.handler.stopIndicatorPolling()
}

// CursorFollowSelection reports whether the real cursor follows the vim
// cursor.
func (m *VimMode) CursorFollowSelection() (bool, bool) {
	if m.handler.vim == nil {
		return false, false
	}

	return m.handler.vim.follow, true
}

// ApplyCursorFollowSelection sets or toggles the preference, and turning it on
// puts the real cursor on the vim cursor.
func (m *VimMode) ApplyCursorFollowSelection(desired *bool) (bool, bool) {
	session := m.handler.vim
	if session == nil {
		return false, false
	}

	if desired == nil {
		session.follow = !session.follow
	} else {
		session.follow = *desired
	}

	m.handler.followVimCursor()

	return session.follow, true
}

// ActiveCaptureScope reports the scope the open session read.
func (m *VimMode) ActiveCaptureScope() string {
	if m.handler.vim == nil {
		return ""
	}

	return m.handler.vim.scope
}

// HasAppHotkeyOverrides reports whether [[vim.app_configs]] binds any per-app
// hotkey.
func (m *VimMode) HasAppHotkeyOverrides() bool {
	if m.handler.config == nil {
		return false
	}

	return m.handler.config.Vim.HasAppHotkeyOverrides()
}
