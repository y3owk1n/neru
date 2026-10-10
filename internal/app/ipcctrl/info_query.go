package ipcctrl

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/derrors"
)

// actionServiceMissing answers a query asked of a daemon wired without the
// action service.
var actionServiceMissing = ipc.Response{
	Success: false,
	Message: msgActionServiceNotAvailable,
	Code:    ipc.CodeActionFailed,
}

// handleQueryDisplays answers `neru query displays`.
func (h *InfoHandler) handleQueryDisplays(ctx context.Context, _ ipc.Command) ipc.Response {
	if h.actionService == nil {
		return actionServiceMissing
	}

	displays, err := h.actionService.Displays(ctx)
	if err != nil {
		return queryFailedResponse("displays", err)
	}

	data := ipc.DisplaysData{Displays: make([]ipc.DisplayData, len(displays))}
	for idx, display := range displays {
		data.Displays[idx] = ipc.DisplayData{
			Name:   display.Name,
			X:      display.Bounds.Min.X,
			Y:      display.Bounds.Min.Y,
			Width:  display.Bounds.Dx(),
			Height: display.Bounds.Dy(),
			Scale:  display.Scale,
		}
	}

	return ipc.Response{Success: true, Message: "displays retrieved", Data: data, Code: ipc.CodeOK}
}

// handleQueryCursor answers `neru query cursor`. It fails whole when the
// displays cannot be listed, since every backend today answers both or neither.
func (h *InfoHandler) handleQueryCursor(ctx context.Context, _ ipc.Command) ipc.Response {
	if h.actionService == nil {
		return actionServiceMissing
	}

	point, err := h.actionService.ObservedCursorPosition(ctx)
	if err != nil {
		return queryFailedResponse("cursor", err)
	}

	displays, err := h.actionService.Displays(ctx)
	if err != nil {
		return queryFailedResponse("cursor", err)
	}

	data := ipc.CursorData{X: point.X, Y: point.Y}

	index, found := displayAt(displays, point)
	if found {
		data.Display = &displays[index].Name
		data.DisplayIndex = &index
	}

	return ipc.Response{Success: true, Message: "cursor retrieved", Data: data, Code: ipc.CodeOK}
}

// handleQueryWindow answers `neru query window`. The payload is null when no
// window has focus, such as when the desktop does.
func (h *InfoHandler) handleQueryWindow(ctx context.Context, _ ipc.Command) ipc.Response {
	if h.actionService == nil {
		return actionServiceMissing
	}

	bounds, found, err := h.actionService.FocusedWindowBounds(ctx)
	if err != nil {
		return queryFailedResponse("window", err)
	}

	resp := ipc.Response{Success: true, Message: "window retrieved", Code: ipc.CodeOK}
	if found {
		resp.Data = ipc.WindowData{
			X:      bounds.Min.X,
			Y:      bounds.Min.Y,
			Width:  bounds.Dx(),
			Height: bounds.Dy(),
		}
	}

	return resp
}

// handleQueryApp answers `neru query app` with the identity per-app config
// matches, read from the same source.
func (h *InfoHandler) handleQueryApp(ctx context.Context, _ ipc.Command) ipc.Response {
	if h.actionService == nil {
		return actionServiceMissing
	}

	bundleID, err := h.actionService.FocusedAppBundleID(ctx)
	if err != nil {
		return queryFailedResponse("app", err)
	}

	return ipc.Response{
		Success: true,
		Message: "app retrieved",
		Data:    ipc.AppData{BundleID: bundleID},
		Code:    ipc.CodeOK,
	}
}

// displayAt returns the index of the first display, in enumeration order,
// whose bounds hold point.
func displayAt(displays []services.Display, point image.Point) (int, bool) {
	for idx, display := range displays {
		if point.In(display.Bounds) {
			return idx, true
		}
	}

	return 0, false
}

// queryFailedResponse reports a query the platform could not answer. A
// platform with no way to answer it says so with ERR_NOT_SUPPORTED, so a
// script can tell that from a query that failed this time.
func queryFailedResponse(query string, err error) ipc.Response {
	code := ipc.CodeActionFailed
	if derrors.IsNotSupported(err) {
		code = ipc.CodeNotSupported
	}

	return ipc.Response{
		Success: false,
		Message: "failed to query " + query + ": " + err.Error(),
		Code:    code,
	}
}
