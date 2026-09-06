package ipcctrl

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// ModesHandler activates and exits the navigation modes on request.
//
// It holds no mode state of its own: the mode handler owns that, and this
// hands it the activation the grammar parsed, unaltered. It holds no rules
// either — what a mode command may say is the grammar's business, in
// internal/domain/modecmd, which the CLI and the configuration validator read
// the same command with.
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
	for mode, activated := range activationMessages {
		handlers[domain.ModeString(mode)] = h.activationHandler(mode, activated)
	}

	handlers[domain.CommandHintsProbe] = h.handleHintsProbe
	handlers[domain.CommandToggleCursorFollowSelection] = h.handleToggleCursorFollowSelection
}

// activationMessages is what each mode answers with once it is entered. The
// wording is what a script reads back, so it is spelled out per mode rather
// than built from the mode's name.
var activationMessages = map[domain.Mode]string{
	domain.ModeHints:         "hints mode activated",
	domain.ModeGrid:          "grid mode activated",
	domain.ModeRecursiveGrid: "recursive-grid mode activated",
	domain.ModeScroll:        "scroll mode activated",
	domain.ModeMonitorSelect: "monitor_select mode activated",
	domain.ModeIdle:          "idle mode activated",
	domain.ModeCustom:        "custom mode activated",
}

// activationHandler answers a mode command by entering the mode it names.
//
// Every mode is served by this one handler: the flags a mode accepts, and the
// rules between them, are the grammar's to know, so there is nothing left here
// for a mode to be an exception to.
func (h *ModesHandler) activationHandler(
	mode domain.Mode,
	activated string,
) func(context.Context, ipc.Command) ipc.Response {
	return func(_ context.Context, cmd ipc.Command) ipc.Response {
		if h.modes == nil {
			return h.modesUnavailableResponse()
		}

		activation, err := modecmd.Parse(mode, cmd.Args)
		if err != nil {
			return refusalFor(err)
		}

		// The grammar reads the declared name; whether anything declares it
		// is the configuration's answer, and a caller is told here rather
		// than by an activation that cannot report back.
		if mode == domain.ModeCustom && !h.modes.DeclaresMode(activation.Name) {
			return refusalFor(modecmd.NotDeclared(activation.Name))
		}

		h.modes.ActivateMode(activation)

		return ipc.Response{Success: true, Message: activated, Code: ipc.CodeOK}
	}
}

// refusalFor answers a grammar error.
//
// The message a user sees is the grammar's own sentence, without the code the
// domain error carries for callers — the code travels in the response's own
// field instead, which is what a script branches on.
func refusalFor(err error) ipc.Response {
	message := derrors.Message(err)

	code := ipc.CodeActionFailed
	if derrors.IsCode(err, derrors.CodeInvalidInput) {
		code = ipc.CodeInvalidInput
	}

	return ipc.Response{Success: false, Message: message, Code: code}
}

// modesUnavailableResponse returns a standardized response when modes handler is not available.
func (h *ModesHandler) modesUnavailableResponse() ipc.Response {
	return ipc.Response{
		Success: false,
		Message: msgModesHandlerNotAvailable,
		Code:    ipc.CodeActionFailed,
	}
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
