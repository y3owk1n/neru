package app

import (
	"strconv"
	"strings"

	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

// parsedActionArgs holds the parsed arguments from an action IPC command.
type parsedActionArgs struct {
	xVal, yVal     int
	deltaX, deltaY int
	hasX, hasY     bool
	hasDX, hasDY   bool
	hasCenter      bool
	hasWindow      bool
	useSelection   bool
	useBare        bool
	monitorName    string
	hasMonitorName bool
	usePrevious    bool
	useBackward    bool
	modifierStr    string
	stepsOverride  int
	hasSteps       bool
	useBail        bool
	stateStr       string
	hasState       bool
	useToggle      bool
}

func shouldClearSelectionAfterMoveMouse(parsed parsedActionArgs, targetsSelection bool) bool {
	if targetsSelection {
		return false
	}

	return (parsed.hasX && parsed.hasY) ||
		parsed.hasCenter ||
		parsed.hasWindow ||
		(parsed.hasDX && parsed.hasDY) ||
		parsed.useBare
}

// extractStringFlag extracts a string value from --flag=value or --flag value form.
// It returns the value, the updated index, and whether the extraction succeeded.
func extractStringFlag(rawArgs []string, idx int, prefix string) (string, int, bool) {
	arg := rawArgs[idx]
	if after, ok := strings.CutPrefix(arg, prefix+"="); ok {
		return after, idx, true
	}

	if arg == prefix {
		if idx+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[idx+1], "--") {
			return rawArgs[idx+1], idx + 1, true
		}

		return "", idx, false
	}

	return "", idx, false
}

// extractIntFlag extracts an integer value from --flag=value or --flag value form.
// It returns the value, the updated index, and whether the extraction succeeded.
func extractIntFlag(rawArgs []string, idx int, prefix string) (int, int, bool) {
	s, newIdx, ok := extractStringFlag(rawArgs, idx, prefix)
	if !ok {
		return 0, newIdx, false
	}

	val, err := strconv.Atoi(s)
	if err != nil {
		return 0, newIdx, false
	}

	return val, newIdx, true
}

// parseActionArgs parses flag arguments from an action IPC command.
// Supports both --flag=value and --flag value forms.
func parseActionArgs(rawArgs []string) (parsedActionArgs, bool) {
	var parsed parsedActionArgs

	parseErr := false
	for idx := 0; idx < len(rawArgs); idx++ {
		arg := rawArgs[idx]
		switch {
		case strings.HasPrefix(arg, "--modifier") && (arg == "--modifier" || arg[len("--modifier")] == '='):
			val, newIdx, ok := extractStringFlag(rawArgs, idx, "--modifier")
			idx = newIdx

			if !ok || val == "" {
				parseErr = true

				break
			}

			parsed.modifierStr = val
		case strings.HasPrefix(arg, "--x") && (arg == "--x" || arg[len("--x")] == '='):
			val, newIdx, ok := extractIntFlag(rawArgs, idx, "--x")
			idx = newIdx

			if !ok {
				parseErr = true

				break
			}

			parsed.xVal = val
			parsed.hasX = true
		case strings.HasPrefix(arg, "--y") && (arg == "--y" || arg[len("--y")] == '='):
			val, newIdx, ok := extractIntFlag(rawArgs, idx, "--y")
			idx = newIdx

			if !ok {
				parseErr = true

				break
			}

			parsed.yVal = val
			parsed.hasY = true
		case strings.HasPrefix(arg, "--dx") && (arg == "--dx" || arg[len("--dx")] == '='):
			val, newIdx, ok := extractIntFlag(rawArgs, idx, "--dx")
			idx = newIdx

			if !ok {
				parseErr = true

				break
			}

			parsed.deltaX = val
			parsed.hasDX = true
		case strings.HasPrefix(arg, "--dy") && (arg == "--dy" || arg[len("--dy")] == '='):
			val, newIdx, ok := extractIntFlag(rawArgs, idx, "--dy")
			idx = newIdx

			if !ok {
				parseErr = true

				break
			}

			parsed.deltaY = val
			parsed.hasDY = true
		case arg == flagCenter:
			parsed.hasCenter = true
		case arg == flagWindow:
			parsed.hasWindow = true
		case arg == flagSelection:
			parsed.useSelection = true
		case arg == flagBare:
			parsed.useBare = true
		case strings.HasPrefix(arg, flagState) && (arg == flagState || arg[len(flagState)] == '='):
			val, newIdx, ok := extractStringFlag(rawArgs, idx, flagState)
			idx = newIdx

			if !ok || val == "" {
				parseErr = true

				break
			}

			parsed.stateStr = val
			parsed.hasState = true
		case arg == flagToggle:
			parsed.useToggle = true
		case arg == flagPrevious:
			parsed.usePrevious = true
		case arg == "--backward":
			parsed.useBackward = true
		case arg == flagBail:
			parsed.useBail = true
		case strings.HasPrefix(arg, flagName) && (arg == flagName || arg[len(flagName)] == '='):
			val, newIdx, ok := extractStringFlag(rawArgs, idx, flagName)
			idx = newIdx

			if !ok || val == "" {
				parseErr = true

				break
			}

			parsed.monitorName = val
			parsed.hasMonitorName = true
		case strings.HasPrefix(arg, "--steps") && (arg == "--steps" || arg[len("--steps")] == '='):
			val, newIdx, ok := extractIntFlag(rawArgs, idx, "--steps")
			idx = newIdx

			if !ok || val <= 0 {
				parseErr = true

				break
			}

			parsed.stepsOverride = val
			parsed.hasSteps = true
		default:
			if strings.HasPrefix(arg, "--") {
				parseErr = true
			}
		}
	}

	return parsed, parseErr
}

// validateActionFlags rejects flag combinations the named action cannot honor.
//
// Validation order matters:
//  1. Reject coordinate flags on non-mouse-move actions.
//  2. Reject --modifier on non-mouse-button actions.
//  3. Reject --x/--y mixed with --dx/--dy (always invalid).
//  4. Reject --center mixed with --dx/--dy (center uses --x/--y as offsets, not deltas).
//  5. Reject --center on non-move_mouse actions.
//  6. Reject --selection mixed with explicit move targeting.
//  7. Require --x AND --y when --center is absent for move_mouse.
//
// Note: --center with --x/--y is intentionally allowed — x/y act as offsets from center.
//
// modifiers are the explicitly requested ones; sticky modifiers are merged by
// the caller afterwards so they cannot trip the --modifier check.
func validateActionFlags(
	actionName string,
	parsed parsedActionArgs,
	modifiers action.Modifiers,
) *ipc.Response {
	isMoveMouse := actionName == string(action.NameMoveMouse)
	isMoveMouseRelative := actionName == string(action.NameMoveMouseRelative)
	isMouseButton := isMouseButtonActionName(actionName)
	isPointTargetedAction := isMoveMouse || isMouseButton

	// 1. Reject coordinate flags on non-mouse-move actions.
	// 2. Reject --modifier on non-mouse-button actions.
	// 3. Reject --x/--y mixed with --dx/--dy (always invalid).
	// 4. Reject --center mixed with --dx/--dy (center uses --x/--y as offsets, not deltas).
	// 5. Reject --center on non-move_mouse actions.
	// 6. Reject --selection mixed with explicit move targeting.
	// 7. Require --x AND --y when --center is absent for move_mouse.
	// Note: --center with --x/--y is intentionally allowed — x/y act as offsets from center.

	if !isMoveMouse && !isMoveMouseRelative &&
		(parsed.hasX || parsed.hasY || parsed.hasDX || parsed.hasDY) {
		return &ipc.Response{
			Success: false,
			Message: "--x/--y/--dx/--dy flags are only supported with move_mouse or move_mouse_relative",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.usePrevious || parsed.hasMonitorName {
		return &ipc.Response{
			Success: false,
			Message: "--previous and --name are only supported with move_monitor",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if modifiers != 0 && !isMouseButton {
		return &ipc.Response{
			Success: false,
			Message: "--modifier is only supported with click and mouse button actions",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if (isMoveMouse || isMoveMouseRelative) && (parsed.hasX || parsed.hasY) &&
		(parsed.hasDX || parsed.hasDY) {
		return &ipc.Response{
			Success: false,
			Message: "use either --x/--y or --dx/--dy, not both",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.hasCenter && (parsed.hasDX || parsed.hasDY) {
		return &ipc.Response{
			Success: false,
			Message: "use either --center or --dx/--dy, not both",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.hasWindow && (parsed.hasDX || parsed.hasDY) {
		return &ipc.Response{
			Success: false,
			Message: "use either --window or --dx/--dy, not both",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.hasCenter && !isMoveMouse {
		return &ipc.Response{
			Success: false,
			Message: "--center is only supported with move_mouse",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.hasWindow && !isMoveMouse {
		return &ipc.Response{
			Success: false,
			Message: "--window is only supported with move_mouse",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.hasCenter && parsed.hasWindow {
		return &ipc.Response{
			Success: false,
			Message: "--center and --window cannot be used together",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.useSelection && parsed.useBare {
		return &ipc.Response{
			Success: false,
			Message: msgSelectionAndBareCannotBeUsedTogether,
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.useSelection && (!isMoveMouse && !isMouseButton) {
		return &ipc.Response{
			Success: false,
			Message: "--selection is only supported with move_mouse, scroll, and mouse button actions",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.useBare && !isPointTargetedAction {
		return &ipc.Response{
			Success: false,
			Message: "--bare is only supported with move_mouse, scroll, and mouse button actions",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.useSelection &&
		(parsed.hasCenter || parsed.hasWindow || parsed.hasX || parsed.hasY) {
		return &ipc.Response{
			Success: false,
			Message: "--selection cannot be combined with --x, --y, --center, or --window",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if isMoveMouse && !parsed.hasCenter && !parsed.hasWindow &&
		!parsed.useSelection &&
		((parsed.hasX && !parsed.hasY) || (!parsed.hasX && parsed.hasY)) {
		return &ipc.Response{
			Success: false,
			Message: "move_mouse requires both --x and --y when using absolute coordinates",
			Code:    ipc.CodeInvalidInput,
		}
	}

	return nil
}

func hasUnsupportedFlags(parsed parsedActionArgs) bool {
	return parsed.hasX || parsed.hasY || parsed.hasDX || parsed.hasDY ||
		parsed.hasCenter || parsed.hasMonitorName || parsed.modifierStr != "" ||
		parsed.useSelection || parsed.useBare || parsed.usePrevious
}
