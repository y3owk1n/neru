package modes

import (
	"image"

	"go.uber.org/zap"

	componentrecursivegrid "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/geometry"
)

// CurrentSelectionPoint returns the active selection point for the current mode, if any.
func (h *Handler) CurrentSelectionPoint() (image.Point, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch h.appState.CurrentMode() {
	case domain.ModeIdle:
		return image.Point{}, false
	case domain.ModeHints:
		return image.Point{}, false
	case domain.ModeGrid:
		if h.grid == nil || h.grid.Context == nil {
			return image.Point{}, false
		}

		return h.grid.Context.SelectionPoint()
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid == nil || h.recursiveGrid.Context == nil {
			return image.Point{}, false
		}

		return h.recursiveGrid.Context.SelectionPoint()
	case domain.ModeScroll:
		return image.Point{}, false
	case domain.ModeMonitorSelect:
		return image.Point{}, false
	}

	return image.Point{}, false
}

// ClearCurrentSelectionPoint removes the active selection point for the current mode.
func (h *Handler) ClearCurrentSelectionPoint() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch h.appState.CurrentMode() {
	case domain.ModeGrid:
		if h.grid == nil || h.grid.Context == nil {
			return false
		}

		h.grid.Context.ClearSelectionPoint()
		h.refreshGridVirtualPointer()

		return true
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid == nil || h.recursiveGrid.Context == nil {
			return false
		}

		h.recursiveGrid.Context.ClearSelectionPoint()
		h.refreshRecursiveGridVirtualPointer()

		return true
	case domain.ModeHints:
		return false
	case domain.ModeIdle:
		return false
	case domain.ModeScroll:
		return false
	case domain.ModeMonitorSelect:
		return false
	}

	return false
}

// cursorFollowContext is the part of a mode's context that carries the
// session's cursor-follow-selection preference. Hints, grid, and recursive grid
// each have their own context type; this is the shape they share, so the
// preference can be read and written without knowing which mode is active.
type cursorFollowContext interface {
	CursorFollowSelection() bool
	SetCursorFollowSelection(cursorFollowSelection bool)
	ToggleCursorFollowSelection() bool
}

// CursorFollowSelection reports whether the active mode's session follows the
// selection with the real cursor. The second result is false when no mode that
// carries the preference is active, which is the same condition under which
// toggling it is refused.
func (h *Handler) CursorFollowSelection() (bool, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	modeContext, ok := h.cursorFollowContext()
	if !ok {
		return false, false
	}

	return modeContext.CursorFollowSelection(), true
}

// ToggleCursorFollowSelection toggles cursor-follow-selection for the active mode.
func (h *Handler) ToggleCursorFollowSelection() (bool, bool) {
	return h.applyCursorFollowSelection(nil)
}

// SetCursorFollowSelection turns cursor-follow-selection on or off for the
// active mode, so a caller that knows which state it wants can converge on it
// rather than flipping whatever is there.
func (h *Handler) SetCursorFollowSelection(enabled bool) (bool, bool) {
	return h.applyCursorFollowSelection(&enabled)
}

// applyCursorFollowSelection sets the preference to desired, or toggles it when
// desired is nil, and reports the resulting value.
//
// Setting the preference to the value it already holds still runs the
// after-effects below. That is what makes the setter idempotent in the way a
// caller needs: "on" always ends with the cursor on the selection, whether or
// not it was already following.
func (h *Handler) applyCursorFollowSelection(desired *bool) (bool, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	modeContext, ok := h.cursorFollowContext()
	if !ok {
		return false, false
	}

	var enabled bool

	if desired == nil {
		enabled = modeContext.ToggleCursorFollowSelection()
	} else {
		modeContext.SetCursorFollowSelection(*desired)

		enabled = *desired
	}

	// Grid and recursive grid draw a virtual pointer that is hidden while the
	// real cursor follows the selection, and both carry a selection point the
	// cursor should jump to when it starts following. Hints has neither: its
	// preference only affects the selections made after it.
	switch h.appState.CurrentMode() {
	case domain.ModeGrid:
		h.refreshGridVirtualPointer()
		h.moveCursorToSelection(enabled, h.grid.Context.SelectionPoint)
	case domain.ModeRecursiveGrid:
		h.refreshRecursiveGridVirtualPointer()
		h.moveCursorToSelection(enabled, h.recursiveGrid.Context.SelectionPoint)
	case domain.ModeHints, domain.ModeIdle, domain.ModeScroll, domain.ModeMonitorSelect:
	}

	return enabled, true
}

// cursorFollowContext returns the active mode's cursor-follow context, or
// false when the active mode does not carry the preference.
func (h *handlerState) cursorFollowContext() (cursorFollowContext, bool) {
	switch h.appState.CurrentMode() {
	case domain.ModeHints:
		if h.hints == nil || h.hints.Context == nil {
			return nil, false
		}

		return h.hints.Context, true
	case domain.ModeGrid:
		if h.grid == nil || h.grid.Context == nil {
			return nil, false
		}

		return h.grid.Context, true
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid == nil || h.recursiveGrid.Context == nil {
			return nil, false
		}

		return h.recursiveGrid.Context, true
	case domain.ModeIdle:
		return nil, false
	case domain.ModeScroll:
		return nil, false
	case domain.ModeMonitorSelect:
		return nil, false
	}

	return nil, false
}

// moveCursorToSelection moves the real cursor onto the mode's stored
// selection point when the mode is following the selection. Turning the
// preference off leaves the cursor where it is.
func (h *handlerState) moveCursorToSelection(
	enabled bool,
	selectionPoint func() (image.Point, bool),
) {
	if !enabled || h.actionService == nil {
		return
	}

	target, ok := selectionPoint()
	if !ok {
		return
	}

	moveCursorErr := h.actionService.MoveCursorToPoint(h.ctx, target)
	if moveCursorErr != nil {
		h.logger.Error("Failed to move cursor", zap.Error(moveCursorErr))
	}
}

func (h *handlerState) refreshGridVirtualPointer() {
	if h.grid == nil || h.grid.Context == nil || h.grid.Overlay == nil {
		return
	}

	point, ok := h.grid.Context.SelectionPoint()

	style, enabled := h.virtualPointerStyle()
	if !ok || h.grid.Context.CursorFollowSelection() || !enabled {
		h.grid.Overlay.HideVirtualPointer()

		return
	}

	localPoint := geometry.ConvertToLocalCoordinates(point, h.screenBounds)
	h.grid.Overlay.ShowVirtualPointer(localPoint, style.fontSize, style.fillColor)
}

func (h *handlerState) refreshRecursiveGridVirtualPointer() {
	if h.recursiveGrid == nil || h.recursiveGrid.Context == nil || h.recursiveGrid.Overlay == nil {
		return
	}

	state := h.currentRecursiveGridVirtualPointerState()
	if !state.Visible {
		h.recursiveGrid.Overlay.HideVirtualPointer()

		return
	}

	h.recursiveGrid.Overlay.ShowVirtualPointer(state.Position, state.Size, state.FillColor)
}

func (h *handlerState) currentRecursiveGridVirtualPointerState() componentrecursivegrid.VirtualPointerState {
	if h.recursiveGrid == nil || h.recursiveGrid.Context == nil {
		return componentrecursivegrid.VirtualPointerState{}
	}

	point, ok := h.recursiveGrid.Context.SelectionPoint()

	style, enabled := h.virtualPointerStyle()
	if !ok || h.recursiveGrid.Context.CursorFollowSelection() || !enabled {
		return componentrecursivegrid.VirtualPointerState{}
	}

	return componentrecursivegrid.VirtualPointerState{
		Visible:   true,
		Position:  geometry.ConvertToLocalCoordinates(point, h.screenBounds),
		Size:      style.fontSize,
		FillColor: style.fillColor,
		Char:      style.char,
		FontName:  style.fontName,
	}
}

type virtualPointerStyle struct {
	fontSize  int
	fillColor string
	char      string
	fontName  string
}

func (h *handlerState) virtualPointerStyle() (virtualPointerStyle, bool) {
	cfg := h.config.VirtualPointer

	fillColor := cfg.UI.TextColor.ForTheme(
		h.themeProvider,
		config.VirtualPointerTextColorLight,
		config.VirtualPointerTextColorDark,
	)

	size := cfg.UI.FontSize
	if size < 1 {
		size = config.DefaultVirtualPointerFontSize
	}

	char := cfg.UI.Char
	if char == "" {
		char = config.DefaultVirtualPointerChar
	}

	return virtualPointerStyle{
		fontSize:  size,
		fillColor: fillColor,
		char:      char,
		fontName:  cfg.UI.FontFamily,
	}, true
}
