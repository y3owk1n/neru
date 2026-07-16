package app

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

func parseCSV(input string) []string {
	if input == "" {
		return nil
	}

	return strings.Split(input, ",")
}

// IPCControllerLifecycle handles lifecycle-related IPC commands.
type IPCControllerLifecycle struct {
	appState *state.AppState
	modes    *modes.Handler
	logger   *zap.Logger
}

// NewIPCControllerLifecycle creates a new lifecycle command handler.
func NewIPCControllerLifecycle(
	appState *state.AppState,
	modes *modes.Handler,
	logger *zap.Logger,
) *IPCControllerLifecycle {
	if appState == nil {
		panic("appState cannot be nil")
	}

	if logger == nil {
		panic("logger cannot be nil")
	}

	return &IPCControllerLifecycle{
		appState: appState,
		modes:    modes,
		logger:   logger,
	}
}

// RegisterHandlers registers lifecycle command handlers.
func (h *IPCControllerLifecycle) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers[domain.CommandPing] = h.handlePing
	handlers[domain.CommandStart] = h.handleStart
	handlers[domain.CommandStop] = h.handleStop
}

func (h *IPCControllerLifecycle) handlePing(_ context.Context, _ ipc.Command) ipc.Response {
	h.logger.Debug("Received ping command")

	return ipc.Response{Success: true, Message: "pong", Code: ipc.CodeOK}
}

func (h *IPCControllerLifecycle) handleStart(_ context.Context, _ ipc.Command) ipc.Response {
	h.logger.Info("Received start command")

	if h.appState.IsEnabled() {
		h.logger.Warn("Attempted to start neru when already running")

		return ipc.Response{
			Success: false,
			Message: "neru is already running",
			Code:    ipc.CodeAlreadyRunning,
		}
	}

	h.appState.SetEnabled(true)
	h.logger.Info("Neru started successfully", zap.Bool("enabled", true))

	return ipc.Response{Success: true, Message: "neru started", Code: ipc.CodeOK}
}

func (h *IPCControllerLifecycle) handleStop(_ context.Context, _ ipc.Command) ipc.Response {
	h.logger.Info("Received stop command")

	if !h.appState.IsEnabled() {
		h.logger.Warn("Attempted to stop neru when already stopped")

		return ipc.Response{
			Success: false,
			Message: "neru is already stopped",
			Code:    ipc.CodeNotRunning,
		}
	}

	h.appState.SetEnabled(false)

	if h.modes != nil {
		h.modes.ExitMode()
	}

	h.logger.Info("Neru stopped successfully")

	return ipc.Response{Success: true, Message: "neru stopped", Code: ipc.CodeOK}
}

// IPCControllerModes handles mode-related IPC commands.
type IPCControllerModes struct {
	modes  *modes.Handler
	logger *zap.Logger // Reserved for future logging needs (maintains consistency with other IPC controllers)
}

// NewIPCControllerModes creates a new mode command handler.
func NewIPCControllerModes(modes *modes.Handler, logger *zap.Logger) *IPCControllerModes {
	return &IPCControllerModes{
		modes:  modes,
		logger: logger,
	}
}

// RegisterHandlers registers mode command handlers.
func (h *IPCControllerModes) RegisterHandlers(
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
func (h *IPCControllerModes) modesUnavailableResponse() ipc.Response {
	return ipc.Response{
		Success: false,
		Message: msgModesHandlerNotAvailable,
		Code:    ipc.CodeActionFailed,
	}
}

// ModeActivationOptions holds the parsed options for activating a navigation mode.
type ModeActivationOptions struct {
	Action                *string
	Repeat                *bool
	CursorFollowSelection *bool
	FilterRoles           []string
	FilterTextContains    []string
	Search                *bool
	Strategy              *string
	LabelDirection        *string
	Toggle                *bool
	Debug                 *bool
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

// extractModeOptions extracts and validates the optional action and repeat
// parameters from a mode IPC command. It returns the options and an optional
// error response. If the response is non-nil the caller should return it
// immediately.
func (h *IPCControllerModes) extractModeOptions(
	cmd ipc.Command,
) (ModeActivationOptions, *ipc.Response) {
	var opts ModeActivationOptions

	if len(cmd.Args) == 0 {
		return opts, nil
	}

	// The CLI sends the mode name as Args[0] (e.g. ["grid", "--action", ...])
	// while the hotkey path omits it (e.g. ["--cursor-selection-mode", "hold"]).
	// Skip the leading mode name when present so both paths are handled.
	start := 0
	if cmd.Args[0] == cmd.Action {
		start = 1
	}

	if start >= len(cmd.Args) {
		return opts, nil
	}

	// Parse positional action arg and flag-style options from remaining args.
	for startIdx := start; startIdx < len(cmd.Args); startIdx++ {
		arg := cmd.Args[startIdx]

		switch {
		case arg == "--repeat" || arg == "-r":
			repeatTrue := true
			opts.Repeat = &repeatTrue
		case arg == "--toggle" || arg == "-t":
			toggleTrue := true
			opts.Toggle = &toggleTrue
		case arg == "--search" || arg == "-s":
			searchTrue := true
			opts.Search = &searchTrue
		case arg == "--debug" || arg == "-d":
			debugTrue := true
			opts.Debug = &debugTrue
		case strings.HasPrefix(arg, "--action="):
			actionArg := strings.TrimPrefix(arg, "--action=")
			opts.Action = &actionArg
		case arg == "--action" || arg == "-a":
			if startIdx+1 >= len(cmd.Args) {
				resp := ipc.Response{
					Success: false,
					Message: "--action requires a value",
					Code:    ipc.CodeInvalidInput,
				}

				return opts, &resp
			}

			startIdx++
			actionArg := cmd.Args[startIdx]
			opts.Action = &actionArg
		case strings.HasPrefix(arg, "--cursor-selection-mode="):
			val, resp := parseCursorSelectionModeValue(
				strings.TrimPrefix(arg, "--cursor-selection-mode="))
			if resp != nil {
				return opts, resp
			}

			opts.CursorFollowSelection = val
		case arg == "--cursor-selection-mode":
			if startIdx+1 >= len(cmd.Args) {
				return opts, &ipc.Response{
					Success: false,
					Message: msgCursorSelectionModeRequires,
					Code:    ipc.CodeInvalidInput,
				}
			}

			startIdx++

			val, resp := parseCursorSelectionModeValue(cmd.Args[startIdx])
			if resp != nil {
				return opts, resp
			}

			opts.CursorFollowSelection = val
		case strings.HasPrefix(arg, "--role="):
			opts.FilterRoles = append(
				opts.FilterRoles,
				parseCSV(strings.TrimPrefix(arg, "--role="))...)
		case arg == "--role":
			if startIdx+1 >= len(cmd.Args) || cmd.Args[startIdx+1] == "--role" {
				resp := ipc.Response{
					Success: false,
					Message: "--role requires a value (use comma-separated: --role=AXButton,AXLink)",
					Code:    ipc.CodeInvalidInput,
				}

				return opts, &resp
			}

			startIdx++
			opts.FilterRoles = append(opts.FilterRoles, parseCSV(cmd.Args[startIdx])...)
		case strings.HasPrefix(arg, "--text="):
			texts := parseCSV(strings.TrimPrefix(arg, "--text="))
			opts.FilterTextContains = append(opts.FilterTextContains, texts...)
		case arg == "--text":
			if startIdx+1 >= len(cmd.Args) || cmd.Args[startIdx+1] == "--text" {
				resp := ipc.Response{
					Success: false,
					Message: "--text requires a value (use comma-separated: --text=foo,bar)",
					Code:    ipc.CodeInvalidInput,
				}

				return opts, &resp
			}

			startIdx++
			texts := parseCSV(cmd.Args[startIdx])
			opts.FilterTextContains = append(opts.FilterTextContains, texts...)
		case strings.HasPrefix(arg, "--strategy="):
			val, resp := parseStrategyEqual(arg)
			if resp != nil {
				return opts, resp
			}

			opts.Strategy = val
		case arg == "--strategy":
			if startIdx+1 >= len(cmd.Args) {
				resp := ipc.Response{
					Success: false,
					Message: "--strategy requires a value: axtree or vision",
					Code:    ipc.CodeInvalidInput,
				}

				return opts, &resp
			}

			startIdx++

			val, resp := parseStrategyValue(cmd.Args[startIdx])
			if resp != nil {
				return opts, resp
			}

			opts.Strategy = val
		case strings.HasPrefix(arg, "--label-direction="):
			val, resp := parseLabelDirectionEqual(arg)
			if resp != nil {
				return opts, resp
			}

			opts.LabelDirection = val
		case arg == "--label-direction":
			if startIdx+1 >= len(cmd.Args) {
				resp := ipc.Response{
					Success: false,
					Message: "--label-direction requires a value: reverse or normal",
					Code:    ipc.CodeInvalidInput,
				}

				return opts, &resp
			}

			startIdx++

			val, resp := parseLabelDirectionValue(cmd.Args[startIdx])
			if resp != nil {
				return opts, resp
			}

			opts.LabelDirection = val
		case opts.Action == nil:
			actionArg := arg
			opts.Action = &actionArg
		default:
			resp := ipc.Response{
				Success: false,
				Message: "unexpected argument: " + arg,
				Code:    ipc.CodeInvalidInput,
			}

			return opts, &resp
		}
	}

	if opts.Action != nil {
		// Split comma-separated actions and validate each one.
		// This enables multi-click sequences like:
		//   hints --action left_click,left_click
		// which produce a double-click via the native click-counting layer.
		actions := strings.Split(*opts.Action, ",")
		for actionIdx, a := range actions {
			trimmed := strings.TrimSpace(a)
			if trimmed == "" {
				resp := ipc.Response{
					Success: false,
					Message: fmt.Sprintf(
						"invalid --action at position %d: empty action in comma-separated list",
						actionIdx,
					),
					Code: ipc.CodeInvalidInput,
				}

				return opts, &resp
			}

			// Validate that the action name is recognized so direct IPC callers
			// get the same immediate feedback as the CLI (which checks via
			// action.IsKnownName in mode_commands.go).
			if !action.IsKnownName(action.Name(trimmed)) {
				resp := ipc.Response{
					Success: false,
					Message: fmt.Sprintf(
						"invalid action: %s. Supported actions: %s",
						trimmed,
						action.SupportedNamesString(),
					),
					Code: ipc.CodeInvalidInput,
				}

				return opts, &resp
			}

			// Scroll sub-actions (scroll_up, page_down, etc.) are IPC/CLI-only and
			// cannot be used as pending mode actions. Reject them here so that
			// direct IPC callers get the same validation as the CLI.
			if action.IsScrollSubAction(trimmed) {
				resp := ipc.Response{
					Success: false,
					Message: fmt.Sprintf(
						"scroll sub-action %q cannot be used as a mode action; use 'action %s' instead",
						trimmed,
						trimmed,
					),
					Code: ipc.CodeInvalidInput,
				}

				return opts, &resp
			}

			_, err := action.Name(trimmed).ToType()
			if err != nil {
				resp := ipc.Response{
					Success: false,
					Message: fmt.Sprintf(
						"mode action %q is not allowed; use 'action %s' instead",
						trimmed,
						trimmed,
					),
					Code: ipc.CodeInvalidInput,
				}

				return opts, &resp
			}
		}
	}

	if opts.Repeat != nil && *opts.Repeat && opts.Action == nil {
		resp := ipc.Response{
			Success: false,
			Message: "--repeat requires an action",
			Code:    ipc.CodeInvalidInput,
		}

		return opts, &resp
	}

	return opts, nil
}

func (h *IPCControllerModes) handleHints(ctx context.Context, cmd ipc.Command) ipc.Response {
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

		summary, probeErr := h.modes.DebugProbeHints(
			ctx,
			opts.FilterRoles,
			opts.FilterTextContains,
			strategy,
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
		Repeat:                opts.Repeat,
		CursorFollowSelection: opts.CursorFollowSelection,
		FilterRoles:           opts.FilterRoles,
		FilterTextContains:    opts.FilterTextContains,
		Search:                opts.Search,
		Strategy:              opts.Strategy,
		LabelDirection:        opts.LabelDirection,
		Toggle:                opts.Toggle,
	})

	return ipc.Response{Success: true, Message: "hints mode activated", Code: ipc.CodeOK}
}

func (h *IPCControllerModes) handleGrid(_ context.Context, cmd ipc.Command) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	opts, errResp := h.extractModeOptions(cmd)
	if errResp != nil {
		return *errResp
	}

	h.modes.ActivateModeWithOptions(domain.ModeGrid, modes.ModeActivationOptions{
		Action:                opts.Action,
		Repeat:                opts.Repeat,
		CursorFollowSelection: opts.CursorFollowSelection,
		Toggle:                opts.Toggle,
	})

	return ipc.Response{Success: true, Message: "grid mode activated", Code: ipc.CodeOK}
}

func (h *IPCControllerModes) handleRecursiveGrid(_ context.Context, cmd ipc.Command) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	opts, errResp := h.extractModeOptions(cmd)
	if errResp != nil {
		return *errResp
	}

	h.modes.ActivateModeWithOptions(domain.ModeRecursiveGrid, modes.ModeActivationOptions{
		Action:                opts.Action,
		Repeat:                opts.Repeat,
		CursorFollowSelection: opts.CursorFollowSelection,
		Toggle:                opts.Toggle,
	})

	return ipc.Response{Success: true, Message: "recursive-grid mode activated", Code: ipc.CodeOK}
}

func (h *IPCControllerModes) handleScroll(_ context.Context, cmd ipc.Command) ipc.Response {
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

func (h *IPCControllerModes) handleMonitorSelect(_ context.Context, cmd ipc.Command) ipc.Response {
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

func (h *IPCControllerModes) handleIdle(_ context.Context, _ ipc.Command) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	h.modes.ActivateMode(domain.ModeIdle)

	return ipc.Response{Success: true, Message: "idle mode activated", Code: ipc.CodeOK}
}

func (h *IPCControllerModes) handleToggleCursorFollowSelection(
	_ context.Context,
	_ ipc.Command,
) ipc.Response {
	if h.modes == nil {
		return h.modesUnavailableResponse()
	}

	enabled, ok := h.modes.ToggleCursorFollowSelection()
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
	}
}

// isValidStrategy checks that the given strategy value is one of the accepted
// values: "axtree" (default), "vision".
func isValidStrategy(v string) bool {
	return v == config.StrategyAXTree || v == config.StrategyVision
}

func parseStrategyEqual(arg string) (*string, *ipc.Response) {
	val := strings.TrimPrefix(arg, "--strategy=")
	if !isValidStrategy(val) {
		resp := ipc.Response{
			Success: false,
			Message: "invalid --strategy value: must be 'axtree' or 'vision'",
			Code:    ipc.CodeInvalidInput,
		}

		return nil, &resp
	}

	return &val, nil
}

func parseStrategyValue(val string) (*string, *ipc.Response) {
	if !isValidStrategy(val) {
		resp := ipc.Response{
			Success: false,
			Message: "invalid --strategy value: must be 'axtree' or 'vision'",
			Code:    ipc.CodeInvalidInput,
		}

		return nil, &resp
	}

	return &val, nil
}

// isValidLabelDirection checks that the given label direction value is one of
// the accepted values: "normal" (default) or "reverse".
func isValidLabelDirection(v string) bool {
	return v == config.LabelDirectionReverse || v == config.LabelDirectionNormal
}

func parseLabelDirectionEqual(arg string) (*string, *ipc.Response) {
	val := strings.TrimPrefix(arg, "--label-direction=")
	if !isValidLabelDirection(val) {
		resp := ipc.Response{
			Success: false,
			Message: "invalid --label-direction value: must be 'reverse' or 'normal'",
			Code:    ipc.CodeInvalidInput,
		}

		return nil, &resp
	}

	return &val, nil
}

func parseLabelDirectionValue(val string) (*string, *ipc.Response) {
	if !isValidLabelDirection(val) {
		resp := ipc.Response{
			Success: false,
			Message: "invalid --label-direction value: must be 'reverse' or 'normal'",
			Code:    ipc.CodeInvalidInput,
		}

		return nil, &resp
	}

	return &val, nil
}

// IPCControllerOverlay handles overlay-related IPC commands.
type IPCControllerOverlay struct {
	appState *state.AppState
	logger   *zap.Logger
}

// NewIPCControllerOverlay creates a new overlay command handler.
func NewIPCControllerOverlay(appState *state.AppState, logger *zap.Logger) *IPCControllerOverlay {
	return &IPCControllerOverlay{
		appState: appState,
		logger:   logger,
	}
}

// RegisterHandlers registers overlay command handlers.
func (h *IPCControllerOverlay) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers[domain.CommandToggleScreenShare] = h.handleToggleScreenShare
}

func (h *IPCControllerOverlay) handleToggleScreenShare(
	_ context.Context,
	_ ipc.Command,
) ipc.Response {
	// Atomically toggle to avoid check-then-act race
	newState := h.appState.ToggleHiddenForScreenShare()

	status := "visible"
	if newState {
		status = "hidden"
	}

	return ipc.Response{
		Success: true,
		Message: "screen share visibility: " + status,
		Code:    ipc.CodeOK,
		Data:    map[string]bool{"hidden": newState},
	}
}

// IPCControllerScroll handles scroll-related IPC commands.
type IPCControllerScroll struct {
	appState      *state.AppState
	scrollService *services.ScrollService
	logger        *zap.Logger
}

// NewIPCControllerScroll creates a new scroll command handler.
func NewIPCControllerScroll(
	appState *state.AppState,
	scrollService *services.ScrollService,
	logger *zap.Logger,
) *IPCControllerScroll {
	return &IPCControllerScroll{
		appState:      appState,
		scrollService: scrollService,
		logger:        logger,
	}
}

// RegisterHandlers registers scroll command handlers.
func (h *IPCControllerScroll) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers[domain.CommandToggleScrollInvert] = h.handleToggleScrollInvert
}

func (h *IPCControllerScroll) handleToggleScrollInvert(
	_ context.Context,
	_ ipc.Command,
) ipc.Response {
	// Atomically toggle to avoid check-then-act race
	newState := h.appState.ToggleScrollInverted()

	if h.scrollService != nil {
		h.scrollService.SetInvertScroll(newState)
	}

	status := "off"
	if newState {
		status = "on"
	}

	return ipc.Response{
		Success: true,
		Message: "scroll invert: " + status,
		Code:    ipc.CodeOK,
		Data:    map[string]bool{"inverted": newState},
	}
}
