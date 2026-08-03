package ipcctrl

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/domain"
)

// ModesHandler activates and exits the navigation modes on request.
//
// It holds no mode state of its own: the mode handler owns that, and this
// translates a command and its flags into a call on it. Flags are parsed by
// extractModeOptions in modeoptions.go.
type ModesHandler struct {
	modes  *modes.Handler
	logger *zap.Logger // Reserved for future logging needs (maintains consistency with other IPC controllers)
}

// NewModesHandler creates a new mode command handler.
func NewModesHandler(modes *modes.Handler, logger *zap.Logger) *ModesHandler {
	return &ModesHandler{
		modes:  modes,
		logger: logger,
	}
}

// RegisterHandlers registers mode command handlers.
func (h *ModesHandler) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers["hints"] = h.handleHints
	handlers["grid"] = h.handleGrid
	handlers["recursive_grid"] = h.handleRecursiveGrid
	handlers["scroll"] = h.handleScroll
	handlers["monitor_select"] = h.handleMonitorSelect
	handlers["idle"] = h.handleIdle
	handlers[domain.CommandToggleCursorFollowSelection] = h.handleToggleCursorFollowSelection
}

const msgCursorSelectionModeRequires = "--cursor-selection-mode requires follow or hold"

// modesUnavailableResponse returns a standardized response when modes handler is not available.
func (h *ModesHandler) modesUnavailableResponse() ipc.Response {
	return ipc.Response{
		Success: false,
		Message: msgModesHandlerNotAvailable,
		Code:    ipc.CodeActionFailed,
	}
}

// ModeActivationOptions holds the parsed options for activating a navigation mode.
type ModeActivationOptions struct {
	Action                *string
	Modifier              *string
	OnExit                []string
	Repeat                *bool
	CursorFollowSelection *bool
	ZoomToDepth           *int
	FilterRoles           []string
	FilterTextContains    []string
	Search                *bool
	HideOnEmptySearch     *bool
	Strategy              *string
	LabelDirection        *string
	Toggle                *bool
	Debug                 *bool
	SplitWord             *bool
}

// parseCursorSelectionModeValue resolves a --cursor-selection-mode value into a
// cursor-follow override, or returns an error response for invalid input.
func parseCursorSelectionModeValue(value string) (*bool, *ipc.Response) {
	switch value {
	case "follow":
		follow := true

		return &follow, nil
	case "hold":
		follow := false

		return &follow, nil
	default:
		return nil, &ipc.Response{
			Success: false,
			Message: msgCursorSelectionModeRequires,
			Code:    ipc.CodeInvalidInput,
		}
	}
}

func (h *ModesHandler) handleHints(ctx context.Context, cmd ipc.Command) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	opts, errResp := h.extractModeOptions(cmd)
	if errResp != nil {
		return *errResp
	}

	// --debug short-circuits to a read-only probe: report what would be hinted
	// for the focused window (count + sample) without drawing the overlay.
	if opts.Debug != nil && *opts.Debug {
		strategy := ""
		if opts.Strategy != nil {
			strategy = *opts.Strategy
		}

		splitWord := false
		if opts.SplitWord != nil {
			splitWord = *opts.SplitWord
		}

		summary, probeErr := h.modes.DebugProbeHints(
			ctx,
			opts.FilterRoles,
			opts.FilterTextContains,
			strategy,
			splitWord,
		)
		if probeErr != nil {
			return ipc.Response{
				Success: false,
				Message: "hints debug probe failed: " + probeErr.Error(),
				Code:    ipc.CodeActionFailed,
			}
		}

		return ipc.Response{Success: true, Message: summary, Code: ipc.CodeOK}
	}

	h.modes.ActivateModeWithOptions(domain.ModeHints, modes.ModeActivationOptions{
		Action:                opts.Action,
		Modifier:              opts.Modifier,
		OnExit:                opts.OnExit,
		Repeat:                opts.Repeat,
		CursorFollowSelection: opts.CursorFollowSelection,
		FilterRoles:           opts.FilterRoles,
		FilterTextContains:    opts.FilterTextContains,
		Search:                opts.Search,
		HideOnEmptySearch:     opts.HideOnEmptySearch,
		Strategy:              opts.Strategy,
		LabelDirection:        opts.LabelDirection,
		Toggle:                opts.Toggle,
		SplitWord:             opts.SplitWord,
	})

	return ipc.Response{Success: true, Message: "hints mode activated", Code: ipc.CodeOK}
}

func (h *ModesHandler) handleGrid(_ context.Context, cmd ipc.Command) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	opts, errResp := h.extractModeOptions(cmd)
	if errResp != nil {
		return *errResp
	}

	h.modes.ActivateModeWithOptions(domain.ModeGrid, modes.ModeActivationOptions{
		Action:                opts.Action,
		Modifier:              opts.Modifier,
		OnExit:                opts.OnExit,
		Repeat:                opts.Repeat,
		CursorFollowSelection: opts.CursorFollowSelection,
		Toggle:                opts.Toggle,
	})

	return ipc.Response{Success: true, Message: "grid mode activated", Code: ipc.CodeOK}
}

func (h *ModesHandler) handleRecursiveGrid(_ context.Context, cmd ipc.Command) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	opts, errResp := h.extractModeOptions(cmd)
	if errResp != nil {
		return *errResp
	}

	h.modes.ActivateModeWithOptions(domain.ModeRecursiveGrid, modes.ModeActivationOptions{
		Action:                opts.Action,
		Modifier:              opts.Modifier,
		OnExit:                opts.OnExit,
		Repeat:                opts.Repeat,
		CursorFollowSelection: opts.CursorFollowSelection,
		ZoomToDepth:           opts.ZoomToDepth,
		Toggle:                opts.Toggle,
	})

	return ipc.Response{Success: true, Message: "recursive-grid mode activated", Code: ipc.CodeOK}
}

func (h *ModesHandler) handleScroll(_ context.Context, cmd ipc.Command) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	opts, errResp := h.extractModeOptions(cmd)
	if errResp != nil {
		return *errResp
	}

	h.modes.ActivateModeWithOptions(domain.ModeScroll, modes.ModeActivationOptions{
		Toggle: opts.Toggle,
	})

	return ipc.Response{Success: true, Message: "scroll mode activated", Code: ipc.CodeOK}
}

func (h *ModesHandler) handleMonitorSelect(_ context.Context, cmd ipc.Command) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	opts, errResp := h.extractModeOptions(cmd)
	if errResp != nil {
		return *errResp
	}

	if opts.Action != nil || opts.Repeat != nil || opts.CursorFollowSelection != nil ||
		len(opts.FilterRoles) > 0 || len(opts.FilterTextContains) > 0 ||
		opts.Search != nil || opts.Strategy != nil || opts.LabelDirection != nil || opts.Debug != nil {
		return ipc.Response{
			Success: false,
			Message: "monitor_select only supports --toggle",
			Code:    ipc.CodeInvalidInput,
		}
	}

	h.modes.ActivateModeWithOptions(domain.ModeMonitorSelect, modes.ModeActivationOptions{
		Toggle: opts.Toggle,
	})

	return ipc.Response{Success: true, Message: "monitor_select mode activated", Code: ipc.CodeOK}
}

func (h *ModesHandler) handleIdle(_ context.Context, _ ipc.Command) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	h.modes.ActivateMode(domain.ModeIdle)

	return ipc.Response{Success: true, Message: "idle mode activated", Code: ipc.CodeOK}
}

func (h *ModesHandler) handleToggleCursorFollowSelection(
	_ context.Context,
	cmd ipc.Command,
) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	desired, errResponse := parseToggleState(
		domain.CommandToggleCursorFollowSelection,
		cmd.Args,
	)
	if errResponse != nil {
		return *errResponse
	}

	// The preference belongs to the active mode's session rather than to the
	// daemon, so unlike the other two toggles this one has nothing to set when
	// no mode is running — including when --state named the state it wants.
	enabled, ok := applyToggleStateWithResult(
		desired,
		h.modes.ToggleCursorFollowSelection,
		h.modes.SetCursorFollowSelection,
	)
	if !ok {
		return ipc.Response{
			Success: false,
			Message: "toggle-cursor-follow-selection is only available in hints, grid, and recursive_grid modes",
			Code:    ipc.CodeInvalidInput,
		}
	}

	state := "disabled"
	if enabled {
		state = "enabled"
	}

	return ipc.Response{
		Success: true,
		Message: "cursor_follow_selection " + state,
		Code:    ipc.CodeOK,
		Data:    map[string]bool{"following": enabled},
	}
}
