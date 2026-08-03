package ipcctrl

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/modeflag"
)

// extractModeOptions reads a mode command's flags into activation options.
//
// Both entry points reach it: the CLI sends the mode name as the first
// argument and the hotkey path omits it, which newModeArgs normalizes. The flag
// names come from modeflag, which is where the CLI writes them from too.
func (h *ModesHandler) extractModeOptions(
	cmd ipc.Command,
) (ModeActivationOptions, *ipc.Response) {
	var opts ModeActivationOptions

	args := newModeArgs(cmd)
	for ; args.more(); args.next() {
		resp := readModeFlag(args, &opts)
		if resp != nil {
			return opts, resp
		}
	}

	return opts, validateModeOptions(opts)
}

// readModeFlag reads the flag the reader is positioned on into opts.
//
// An argument matching no flag is taken as the positional action when none has
// been set, and refused otherwise.
func readModeFlag(args *modeArgs, opts *ModeActivationOptions) *ipc.Response {
	switch {
	case args.is(modeflag.Repeat):
		return setTrue(&opts.Repeat)

	case args.is(modeflag.Toggle):
		return setTrue(&opts.Toggle)

	case args.is(modeflag.Search):
		return setTrue(&opts.Search)

	case args.is(modeflag.HideOnEmptySearch):
		return setTrue(&opts.HideOnEmptySearch)

	case args.is(modeflag.Debug):
		return nil

	case args.is(modeflag.SplitWord):
		return setTrue(&opts.SplitWord)

	case args.is(modeflag.Action):
		return readStringFlag(args, &opts.Action, "--action requires a value")

	case args.is(modeflag.Modifier):
		return readStringFlag(args, &opts.Modifier, "--modifier requires a value")

	case args.is(modeflag.ZoomToDepth):
		return readZoomToDepth(args, opts)

	case args.is(modeflag.OnExit):
		value, resp := args.take("--on-exit requires a value")
		if resp != nil {
			return resp
		}

		// Repeatable: each occurrence appends one step to the sequence that
		// runs once the pending action is fulfilled.
		opts.OnExit = append(opts.OnExit, value)

		return nil

	case args.is(modeflag.CursorSelectionMode):
		value, resp := args.take(msgCursorSelectionModeRequires)
		if resp != nil {
			return resp
		}

		parsed, badValue := parseCursorSelectionModeValue(value)
		if badValue != nil {
			return badValue
		}

		opts.CursorFollowSelection = parsed

		return nil

	case args.is(modeflag.Role):
		return readListFlag(args, &opts.FilterRoles,
			"--role requires a value (use comma-separated: --role=AXButton,AXLink)")

	case args.is(modeflag.Text):
		return readListFlag(args, &opts.FilterTextContains,
			"--text requires a value (use comma-separated: --text=foo,bar)")

	case args.is(modeflag.Strategy):
		value, resp := args.take("--strategy requires a value: axtree or vision")
		if resp != nil {
			return resp
		}

		parsed, badValue := parseStrategyValue(value)
		if badValue != nil {
			return badValue
		}

		opts.Strategy = parsed

		return nil

	case args.is(modeflag.LabelDirection):
		value, resp := args.take("--label-direction requires a value: reverse or normal")
		if resp != nil {
			return resp
		}

		parsed, badValue := parseLabelDirectionValue(value)
		if badValue != nil {
			return badValue
		}

		opts.LabelDirection = parsed

		return nil

	// An argument matching no flag is the positional action, which is how
	// `neru hints left_click` is written. Once an action is set there is
	// nothing left for a stray argument to mean.
	case opts.Action == nil:
		value := args.arg()
		opts.Action = &value

		return nil

	default:
		return refuse("unexpected argument: " + args.arg())
	}
}

// setTrue marks a presence-only flag. It returns a response so every branch of
// readModeFlag reads the same way.
func setTrue(field **bool) *ipc.Response {
	value := true
	*field = &value

	return nil
}

// readStringFlag stores a flag's value as-is.
func readStringFlag(args *modeArgs, field **string, missing string) *ipc.Response {
	value, resp := args.take(missing)
	if resp != nil {
		return resp
	}

	*field = &value

	return nil
}

// readListFlag appends a flag's comma-separated value. The flag is repeatable,
// so entries accumulate across occurrences.
func readListFlag(args *modeArgs, field *[]string, missing string) *ipc.Response {
	value, resp := args.take(missing)
	if resp != nil {
		return resp
	}

	*field = append(*field, parseCSV(value)...)

	return nil
}

// readZoomToDepth reads the one numeric flag.
func readZoomToDepth(args *modeArgs, opts *ModeActivationOptions) *ipc.Response {
	value, resp := args.take("--zoom-to-depth requires a value")
	if resp != nil {
		return resp
	}

	depth, err := strconv.Atoi(value)
	if err != nil || depth < 0 {
		return refuse("--zoom-to-depth requires a non-negative integer")
	}

	opts.ZoomToDepth = &depth

	return nil
}

// validateModeOptions applies the rules that need the whole option set: an
// action's own vocabulary, and the flags that are only meaningful alongside
// another one. They run after parsing because a flag may appear before the
// one it depends on.
func validateModeOptions(opts ModeActivationOptions) *ipc.Response {
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

				return &resp
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

				return &resp
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

				return &resp
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

				return &resp
			}
		}
	}

	if opts.Repeat != nil && *opts.Repeat && opts.Action == nil {
		resp := ipc.Response{
			Success: false,
			Message: "--repeat requires an action",
			Code:    ipc.CodeInvalidInput,
		}

		return &resp
	}

	if opts.HideOnEmptySearch != nil && *opts.HideOnEmptySearch &&
		(opts.Search == nil || !*opts.Search) {
		resp := ipc.Response{
			Success: false,
			Message: "--hide-on-empty-search requires --search",
			Code:    ipc.CodeInvalidInput,
		}

		return &resp
	}

	if opts.Modifier != nil {
		if opts.Action == nil {
			resp := ipc.Response{
				Success: false,
				Message: "--modifier requires an action",
				Code:    ipc.CodeInvalidInput,
			}

			return &resp
		}

		mods, modErr := action.ParseModifiers(*opts.Modifier)
		if modErr != nil {
			resp := ipc.Response{
				Success: false,
				Message: modErr.Error(),
				Code:    ipc.CodeInvalidInput,
			}

			return &resp
		}

		if mods == 0 {
			resp := ipc.Response{
				Success: false,
				Message: "modifier values cannot be empty",
				Code:    ipc.CodeInvalidInput,
			}

			return &resp
		}
	}

	return nil
}

// isValidStrategy reports whether v names a detection strategy: "axtree", the
// default, or "vision".
func isValidStrategy(v string) bool {
	return v == config.StrategyAXTree || v == config.StrategyVision
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
