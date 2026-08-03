package ipcctrl

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// Flag parsing for the mode commands.
//
// Both entry points reach here: the CLI sends the mode name as the first
// argument, the hotkey path omits it, and extractModeOptions normalizes that
// before reading the flags.

//nolint:funlen
func (h *ModesHandler) extractModeOptions(
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
		case strings.HasPrefix(arg, "--zoom-to-depth="):
			depthVal, err := strconv.Atoi(strings.TrimPrefix(arg, "--zoom-to-depth="))
			if err != nil || depthVal < 0 {
				resp := ipc.Response{
					Success: false,
					Message: "--zoom-to-depth requires a non-negative integer",
					Code:    ipc.CodeInvalidInput,
				}

				return opts, &resp
			}

			opts.ZoomToDepth = &depthVal
		case arg == "--zoom-to-depth":
			if startIdx+1 >= len(cmd.Args) {
				resp := ipc.Response{
					Success: false,
					Message: "--zoom-to-depth requires a value",
					Code:    ipc.CodeInvalidInput,
				}

				return opts, &resp
			}

			startIdx++

			depthVal, err := strconv.Atoi(cmd.Args[startIdx])
			if err != nil || depthVal < 0 {
				resp := ipc.Response{
					Success: false,
					Message: "--zoom-to-depth requires a non-negative integer",
					Code:    ipc.CodeInvalidInput,
				}

				return opts, &resp
			}

			opts.ZoomToDepth = &depthVal
		case arg == "--search" || arg == "-s":
			searchTrue := true
			opts.Search = &searchTrue
		case arg == "--hide-on-empty-search":
			hideTrue := true
			opts.HideOnEmptySearch = &hideTrue
		case arg == "--debug" || arg == "-d":
			debugTrue := true
			opts.Debug = &debugTrue
		case strings.HasPrefix(arg, "--modifier="):
			modifierArg := strings.TrimPrefix(arg, "--modifier=")
			opts.Modifier = &modifierArg
		case arg == "--modifier":
			if startIdx+1 >= len(cmd.Args) {
				resp := ipc.Response{
					Success: false,
					Message: "--modifier requires a value",
					Code:    ipc.CodeInvalidInput,
				}

				return opts, &resp
			}

			startIdx++
			modifierArg := cmd.Args[startIdx]
			opts.Modifier = &modifierArg
		case arg == "--split-word":
			splitWordTrue := true
			opts.SplitWord = &splitWordTrue
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
		// --on-exit is repeatable: each occurrence appends one step to the
		// sequence that runs once the pending action is fulfilled.
		case strings.HasPrefix(arg, config.OnExitFlag+"="):
			opts.OnExit = append(opts.OnExit, strings.TrimPrefix(arg, config.OnExitFlag+"="))
		case arg == config.OnExitFlag:
			if startIdx+1 >= len(cmd.Args) {
				resp := ipc.Response{
					Success: false,
					Message: "--on-exit requires a value",
					Code:    ipc.CodeInvalidInput,
				}

				return opts, &resp
			}

			startIdx++
			opts.OnExit = append(opts.OnExit, cmd.Args[startIdx])
		case strings.HasPrefix(arg, "--cursor-selection-mode="):
			val, resp := parseCursorSelectionModeValue(
				strings.TrimPrefix(arg, "--cursor-selection-mode="),
			)
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
				parseCSV(strings.TrimPrefix(arg, "--role="))...,
			)
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

			actType, err := action.Name(trimmed).ToType()
			if err != nil || !actType.IsMouseButton() {
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

	if opts.HideOnEmptySearch != nil && *opts.HideOnEmptySearch &&
		(opts.Search == nil || !*opts.Search) {
		resp := ipc.Response{
			Success: false,
			Message: "--hide-on-empty-search requires --search",
			Code:    ipc.CodeInvalidInput,
		}

		return opts, &resp
	}

	if opts.Modifier != nil {
		if opts.Action == nil {
			resp := ipc.Response{
				Success: false,
				Message: "--modifier requires an action",
				Code:    ipc.CodeInvalidInput,
			}

			return opts, &resp
		}

		mods, modErr := action.ParseModifiers(*opts.Modifier)
		if modErr != nil {
			resp := ipc.Response{
				Success: false,
				Message: modErr.Error(),
				Code:    ipc.CodeInvalidInput,
			}

			return opts, &resp
		}

		if mods == 0 {
			resp := ipc.Response{
				Success: false,
				Message: "modifier values cannot be empty",
				Code:    ipc.CodeInvalidInput,
			}

			return opts, &resp
		}
	}

	return opts, nil
}

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

// OverlayHandler handles overlay-related IPC commands.
type OverlayHandler struct {
	appState *state.AppState
	logger   *zap.Logger
}

// NewOverlayHandler creates a new overlay command handler.
func NewOverlayHandler(appState *state.AppState, logger *zap.Logger) *OverlayHandler {
	return &OverlayHandler{
		appState: appState,
		logger:   logger,
	}
}

// RegisterHandlers registers overlay command handlers.
func (h *OverlayHandler) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers[domain.CommandToggleScreenShare] = h.handleToggleScreenShare
}

func (h *OverlayHandler) handleToggleScreenShare(
	_ context.Context,
	cmd ipc.Command,
) ipc.Response {
	desired, errResponse := parseToggleState(domain.CommandToggleScreenShare, cmd.Args)
	if errResponse != nil {
		return *errResponse
	}

	// --state names the reported state, so "on" is hidden. Naming it after the
	// visibility instead would invert between the flag and the status field.
	newState := applyToggleState(
		desired,
		// Atomically toggle to avoid check-then-act race
		h.appState.ToggleHiddenForScreenShare,
		h.appState.SetHiddenForScreenShare,
	)

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

// ScrollHandler handles scroll-related IPC commands.
type ScrollHandler struct {
	appState      *state.AppState
	scrollService *services.ScrollService
	logger        *zap.Logger
}

// NewScrollHandler creates a new scroll command handler.
func NewScrollHandler(
	appState *state.AppState,
	scrollService *services.ScrollService,
	logger *zap.Logger,
) *ScrollHandler {
	return &ScrollHandler{
		appState:      appState,
		scrollService: scrollService,
		logger:        logger,
	}
}

// RegisterHandlers registers scroll command handlers.
func (h *ScrollHandler) RegisterHandlers(
	handlers map[string]func(context.Context, ipc.Command) ipc.Response,
) {
	handlers[domain.CommandToggleScrollInvert] = h.handleToggleScrollInvert
}

func (h *ScrollHandler) handleToggleScrollInvert(
	_ context.Context,
	cmd ipc.Command,
) ipc.Response {
	desired, errResponse := parseToggleState(domain.CommandToggleScrollInvert, cmd.Args)
	if errResponse != nil {
		return *errResponse
	}

	newState := applyToggleState(
		desired,
		// Atomically toggle to avoid check-then-act race
		h.appState.ToggleScrollInverted,
		h.appState.SetScrollInverted,
	)

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
