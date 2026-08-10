package ipcctrl

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// Flags shared by several actions, named so the table stays readable.
//
// Flag support is declarative: an action declares what it accepts and
// rejectUnsupportedFlags refuses the rest, once, before dispatch. The
// per-handler reject lists this replaced had drifted — backspace silently
// accepted --steps, --toggle and --backward, added to the parser after the list
// was written. Forgetting an entry here is now a rejection, not a dropped
// flag.
var (
	// clickFlags are accepted by the three click actions, which can be split
	// into their press and release halves.
	clickFlags = []string{flagModifier, flagSelection, flagBare, flagState, flagToggle}
	// heldButtonFlags are accepted by the press/release/toggle actions, which
	// already name a phase and so take no --state or --toggle.
	heldButtonFlags = []string{flagModifier, flagSelection, flagBare}
	// pagedScrollFlags are accepted by the scroll sub-actions whose amount is
	// fixed by configuration, so they take no --steps.
	pagedScrollFlags = []string{flagModifier, flagSelection, flagBare}
	// steppedScrollFlags add --steps for the directional sub-actions.
	steppedScrollFlags = []string{flagModifier, flagSteps, flagSelection, flagBare}
)

// actionFlagSupport maps an action name to the flags it accepts.
//
// feed and sleep are absent on purpose: both consume their raw arguments
// before flag parsing runs, so they never reach this check.
var actionFlagSupport = map[string][]string{
	string(action.NameLeftClick):   clickFlags,
	string(action.NameRightClick):  clickFlags,
	string(action.NameMiddleClick): clickFlags,

	string(action.NameLeftMouseDown):     heldButtonFlags,
	string(action.NameLeftMouseUp):       heldButtonFlags,
	string(action.NameRightMouseDown):    heldButtonFlags,
	string(action.NameRightMouseUp):      heldButtonFlags,
	string(action.NameMiddleMouseDown):   heldButtonFlags,
	string(action.NameMiddleMouseUp):     heldButtonFlags,
	string(action.NameLeftMouseToggle):   heldButtonFlags,
	string(action.NameRightMouseToggle):  heldButtonFlags,
	string(action.NameMiddleMouseToggle): heldButtonFlags,
	// The deprecated left-button spellings are still accepted from configs
	// and IPC, so they need flag declarations too.
	string(action.NameMouseDown): heldButtonFlags, //nolint:staticcheck // still accepted
	string(action.NameMouseUp):   heldButtonFlags, //nolint:staticcheck // still accepted

	string(action.NameMoveMouse): {
		flagX, flagY, flagCenter, flagWindow, flagSelection, flagBare,
	},
	string(action.NameMoveMouseRelative): {flagDX, flagDY},
	string(action.NameMoveMonitor):       {flagPrevious, flagName},
	string(action.NameMoveCell):          {flagDirection, flagCount},

	// The bare scroll name is only reachable from a hotkey string or raw IPC
	// and takes no flags; the directional sub-actions below are the usable form.
	string(action.NameScroll):      {},
	string(action.NameScrollUp):    steppedScrollFlags,
	string(action.NameScrollDown):  steppedScrollFlags,
	string(action.NameScrollLeft):  steppedScrollFlags,
	string(action.NameScrollRight): steppedScrollFlags,
	string(action.NameGoTop):       pagedScrollFlags,
	string(action.NameGoBottom):    pagedScrollFlags,
	string(action.NamePageUp):      pagedScrollFlags,
	string(action.NamePageDown):    pagedScrollFlags,

	string(action.NameCycleHint):       {flagBackward},
	string(action.NameWaitForModeExit): {flagBail},

	string(action.NameReset):            {},
	string(action.NameBackspace):        {},
	string(action.NameSearchHints):      {},
	string(action.NameSaveCursorPos):    {flagSlot},
	string(action.NameRestoreCursorPos): {flagSlot},
	string(action.NameHideCursor):       {},
	string(action.NameShowCursor):       {},
}

// flagRejectionMessage overrides the generated message for flags whose
// audience reads better as prose than as a list of every action name.
var flagRejectionMessage = map[string]string{
	flagState:  msgStateOnlyOnClicks,
	flagToggle: msgStateOnlyOnClicks,
	flagModifier: "--modifier is only supported with click, mouse button, " +
		"and scroll actions",
	flagSelection: "--selection is only supported with move_mouse, scroll, and mouse button actions",
	flagBare:      "--bare is only supported with move_mouse, scroll, and mouse button actions",
}

// rejectUnsupportedFlags refuses any flag the named action does not accept.
//
// Unknown action names are left alone so the dispatcher can report the name
// itself, which is the more useful error.
func rejectUnsupportedFlags(actionName string, parsed parsedActionArgs) *ipc.Response {
	allowed, known := allowedFlagsFor(actionName)
	if !known {
		return nil
	}

	for _, flag := range parsed.presentFlags() {
		if slices.Contains(allowed, flag) {
			continue
		}

		return &ipc.Response{
			Success: false,
			Message: unsupportedFlagMessage(actionName, flag),
			Code:    ipc.CodeInvalidInput,
		}
	}

	return nil
}

// allowedFlagsFor resolves the flags an action name accepts. For a
// comma-separated chain it is the intersection over the chained actions, since
// every one of them runs with the same flags. The second result is false when
// any name is unknown.
func allowedFlagsFor(actionName string) ([]string, bool) {
	if !strings.Contains(actionName, ",") {
		allowed, known := actionFlagSupport[actionName]

		return allowed, known
	}

	var allowed []string

	for index, name := range strings.Split(actionName, ",") {
		flags, known := actionFlagSupport[strings.TrimSpace(name)]
		if !known {
			return nil, false
		}

		// Clone before narrowing: the table's slices are shared between
		// actions, so DeleteFunc must not run on one of them in place.
		if index == 0 {
			allowed = slices.Clone(flags)

			continue
		}

		allowed = slices.DeleteFunc(allowed, func(flag string) bool {
			return !slices.Contains(flags, flag)
		})
	}

	return allowed, true
}

// unsupportedFlagMessage explains why flag was refused, naming the actions
// that do accept it so the user has somewhere to go.
func unsupportedFlagMessage(actionName, flag string) string {
	if message, ok := flagRejectionMessage[flag]; ok {
		return message
	}

	audience := actionsAcceptingFlag(flag)
	if len(audience) == 0 {
		return fmt.Sprintf("%s is not supported by %s", flag, actionName)
	}

	return fmt.Sprintf(
		"%s is only supported with %s",
		flag,
		strings.Join(audience, ", "),
	)
}

// actionsAcceptingFlag lists, in stable order, the actions that accept flag.
func actionsAcceptingFlag(flag string) []string {
	var names []string

	for name, flags := range actionFlagSupport {
		if slices.Contains(flags, flag) {
			names = append(names, name)
		}
	}

	slices.Sort(names)

	return names
}

// presentFlags returns the flags the caller supplied, in stable order.
func (p *parsedActionArgs) presentFlags() []string {
	return slices.Sorted(maps.Keys(p.present))
}

// markPresent records that flag appeared in the argument list.
func (p *parsedActionArgs) markPresent(flag string) {
	if p.present == nil {
		p.present = make(map[string]bool, 1)
	}

	p.present[flag] = true
}
