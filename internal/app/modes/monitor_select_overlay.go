package modes

import (
	"strings"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/ports"
)

// monitorSelectFrame describes the monitor picker as it should be on screen
// for the active session: one target per selectable display, each carrying how
// far the user's input has matched its label.
//
// It carries domain values only. Whether this backend can draw the picker at
// all is an optional capability the overlay answers for itself, so no mode
// asks a backend what it supports before describing what it wants.
func (h *handlerState) monitorSelectFrame() ports.MonitorSelectFrame {
	input := h.monitorSelect.Input()

	selectedName := ""
	if selected := h.monitorSelect.Selected(); selected != nil {
		selectedName = selected.Name
	}

	sessionTargets := h.monitorSelect.Targets()

	targets := make([]ports.MonitorSelectTarget, 0, len(sessionTargets))
	for _, target := range sessionTargets {
		targets = append(targets, ports.MonitorSelectTarget{
			Bounds:           target.Bounds,
			Label:            target.Label,
			Name:             target.Name,
			Selected:         target.Name == selectedName,
			MatchedPrefixLen: matchedPrefixLength(target.Label, input),
		})
	}

	return ports.MonitorSelectFrame{Targets: targets}
}

// matchedPrefixLength returns how many leading runes of label are matched by
// the current (case-insensitive) input, or 0 when it is not a prefix.
func matchedPrefixLength(label, input string) int {
	if input == "" {
		return 0
	}

	labelRunes := []rune(label)
	inputRunes := []rune(strings.ToLower(input))
	labelFolded := []rune(strings.ToLower(label))

	if len(inputRunes) > len(labelRunes) {
		return 0
	}

	for idx := range inputRunes {
		if labelFolded[idx] != inputRunes[idx] {
			return 0
		}
	}

	return len(inputRunes)
}

// redrawMonitorSelect repaints the picker over the panels already on screen.
// Every keystroke redraws the whole surface — the picker has no incremental
// path — so a keystroke hands over a frame, as recursive grid does.
func (h *handlerState) redrawMonitorSelect() {
	if h.monitorSelect == nil {
		return
	}

	h.redrawFrame(h.monitorSelectFrame(), "redraw monitor_select overlay")
}

// RefreshMonitorSelectForThemeChange redraws the monitor_select overlay when
// the mode is active. The colors come from the Style the overlay resolved, so
// a redraw is all a theme change needs.
//
// Returns true if the mode was in a state to refresh, the way the three other
// theme refreshers already report it: the picker is the odd one out of four
// otherwise identical calls, and one uniform signature is what lets the four
// become one thing a mode answers for.
func (h *Handler) RefreshMonitorSelectForThemeChange() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() != domain.ModeMonitorSelect || h.monitorSelect == nil {
		return false
	}

	h.redrawMonitorSelect()

	return true
}
