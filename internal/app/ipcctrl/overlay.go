package ipcctrl

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/parity"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// OverlayHandler handles the overlay commands, which hide the overlay from
// screen sharing.
type OverlayHandler struct {
	appState *state.AppState
	logger   *zap.Logger
	// platform names the platform the handler answers for.
	platform func() (parity.Platform, bool)
}

// NewOverlayHandler creates a new overlay command handler.
func NewOverlayHandler(appState *state.AppState, logger *zap.Logger) *OverlayHandler {
	return &OverlayHandler{
		appState: appState,
		logger:   logger,
		platform: parity.Current,
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
	refusal := h.screenShareRefusal()
	if refusal != nil {
		return *refusal
	}

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
		Data:    ipc.ScreenShareData{Hidden: newState},
	}
}

// screenShareRefusal refuses toggle-screen-share where the option it switches
// does nothing, rather than reporting a hidden overlay that nothing hid.
func (h *OverlayHandler) screenShareRefusal() *ipc.Response {
	platform, known := h.platform()
	if known && config.HidesFromScreenShare(platform) {
		return nil
	}

	where := string(platform)
	if !known {
		where = "this platform"
	}

	return &ipc.Response{
		Success: false,
		Message: domain.CommandToggleScreenShare + " does nothing on " + where + ": " +
			config.ScreenShareHideNote,
		Code: ipc.CodeNotSupported,
	}
}
