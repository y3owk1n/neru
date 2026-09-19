package modes

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/bisect"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// Compile-time interface compliance checks: the core interface, then every
// optional extension bisect mode opts into (extensions.go).
var (
	_ Mode                   = (*BisectMode)(nil)
	_ bisector               = (*BisectMode)(nil)
	_ selectionTracker       = (*BisectMode)(nil)
	_ cursorFollowSelector   = (*BisectMode)(nil)
	_ inputEditor            = (*BisectMode)(nil)
	_ hotkeyOverrideReporter = (*BisectMode)(nil)
	_ themeRefresher         = (*BisectMode)(nil)
	_ screenRefresher        = (*BisectMode)(nil)
	_ captureScopeReporter   = (*BisectMode)(nil)
)

// BisectMode implements the Mode interface for bisection navigation.
type BisectMode struct {
	baseMode
}

// NewBisectMode creates a new bisect mode implementation.
func NewBisectMode(handler *handlerState) *BisectMode {
	return &BisectMode{
		baseMode: newBaseMode(handler, domain.ModeBisect, "BisectMode"),
	}
}

// Activate enters bisect mode over the screen or the focused window.
func (m *BisectMode) Activate(activation modecmd.Activation) {
	m.handler.startBisect(activation)
}

// HandleKey processes a key press within bisect mode. Every key the mode
// answers is bound in its hotkey table, so a bare key does nothing here.
func (m *BisectMode) HandleKey(_ string) {}

// RefreshForMonitorMove remaps the region onto the display the cursor landed
// on and hands it over as a Frame there.
func (m *BisectMode) RefreshForMonitorMove(_ context.Context, targetBounds image.Rectangle) {
	m.handler.refreshBisectForMonitorMove(targetBounds)
}

// RefreshForThemeChange draws the region again so it picks up the colors the
// overlay just re-resolved. A mode with no session has nothing to draw.
func (m *BisectMode) RefreshForThemeChange() bool {
	if m.handler.bisect == nil || m.handler.bisect.Region == nil {
		return false
	}

	m.handler.updateBisectOverlay()

	return true
}

// RefreshForScreenChange remaps the region onto the display as it now is and
// hands it over as a transition onto that display. Bisect switched off in
// configuration leaves the overlay for the caller to resize.
func (m *BisectMode) RefreshForScreenChange(_ context.Context) bool {
	if m.handler.config == nil || !m.handler.config.Bisect.Enabled || m.handler.bisect == nil {
		return false
	}

	m.handler.refreshBisectForScreenChange()

	return true
}

// Exit tears bisect mode down.
func (m *BisectMode) Exit() {
	m.handler.cleanupBisectMode()
}

// Bisect keeps the half or quadrant cut names, count times over as one
// press, and moves the cursor to the center of what is left.
func (m *BisectMode) Bisect(cut bisect.Cut, count int) {
	m.handler.bisectCut(cut, count)
}

// ResetInput puts the region back over the whole capture area.
func (m *BisectMode) ResetInput() {
	if m.handler.bisect == nil || m.handler.bisect.Region == nil {
		return
	}

	m.handler.bisect.Region.Reset()
	m.handler.settleBisect("Failed to move cursor after bisect reset")
}

// Backspace takes the last cut back. Nothing to take back leaves the region,
// and the cursor, where they are.
func (m *BisectMode) Backspace() {
	if m.handler.bisect == nil || m.handler.bisect.Region == nil ||
		!m.handler.bisect.Region.Backtrack() {
		return
	}

	m.handler.settleBisect("Failed to move cursor after bisect backspace")
}

// SelectionPoint reports the region center bisect mode has selected.
func (m *BisectMode) SelectionPoint() (image.Point, bool) {
	if m.handler.bisect == nil || m.handler.bisect.Context == nil {
		return image.Point{}, false
	}

	return m.handler.bisect.Context.SelectionPoint()
}

// ClearSelectionPoint forgets the selected center and takes the virtual
// pointer that stood on it off the screen.
func (m *BisectMode) ClearSelectionPoint() bool {
	if m.handler.bisect == nil || m.handler.bisect.Context == nil {
		return false
	}

	m.handler.bisect.Context.ClearSelectionPoint()
	m.handler.refreshBisectVirtualPointer()

	return true
}

// SelectionAnchor anchors to the region center whenever the real cursor is
// not following the selection itself.
func (m *BisectMode) SelectionAnchor() (image.Point, bool) {
	if m.handler.bisect == nil || m.handler.bisect.Context == nil ||
		m.handler.bisect.Context.CursorFollowSelection() {
		return image.Point{}, false
	}

	return m.handler.bisect.Context.SelectionPoint()
}

// CursorFollowSelection reports whether the real cursor rides along with the
// region's center.
func (m *BisectMode) CursorFollowSelection() (bool, bool) {
	if m.handler.bisect == nil || m.handler.bisect.Context == nil {
		return false, false
	}

	return m.handler.bisect.Context.CursorFollowSelection(), true
}

// ApplyCursorFollowSelection sets or toggles the preference and then settles
// what the change owes: the virtual pointer stands in only while the cursor
// is held back, and turning following on puts the cursor on the center.
func (m *BisectMode) ApplyCursorFollowSelection(desired *bool) (bool, bool) {
	if m.handler.bisect == nil || m.handler.bisect.Context == nil {
		return false, false
	}

	enabled := applyCursorFollow(m.handler.bisect.Context, desired)

	m.handler.updateBisectOverlay()
	m.handler.moveCursorToSelection(enabled, m.handler.bisect.Context.SelectionPoint)

	return enabled, true
}

// HasAppHotkeyOverrides reports whether [bisect.app_configs] binds any
// per-app hotkey.
func (m *BisectMode) HasAppHotkeyOverrides() bool {
	if m.handler.config == nil {
		return false
	}

	return m.handler.config.Bisect.HasAppHotkeyOverrides()
}

// ActiveCaptureScope reports the scope the open bisect session started from.
func (m *BisectMode) ActiveCaptureScope() string {
	if m.handler.bisect == nil || m.handler.bisect.Context == nil {
		return ""
	}

	return m.handler.bisect.Context.CaptureScope()
}
