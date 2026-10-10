package ipcctrl

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/derrors"
)

// handleQueryDisplays answers `neru query displays`.
func (h *InfoHandler) handleQueryDisplays(ctx context.Context, _ ipc.Command) ipc.Response {
	if h.actionService == nil {
		return ipc.Response{
			Success: false,
			Message: msgActionServiceNotAvailable,
			Code:    ipc.CodeActionFailed,
		}
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
		return ipc.Response{
			Success: false,
			Message: msgActionServiceNotAvailable,
			Code:    ipc.CodeActionFailed,
		}
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
