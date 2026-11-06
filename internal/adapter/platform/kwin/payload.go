//go:build linux

package kwin

import (
	"image"
	"strconv"
	"strings"
)

// The wire format between the KWin script and this package. It is one
// comma-separated string because KWin JavaScript's number marshaling is not
// worth depending on. Parsing is kept off the D-Bus method so it can be tested
// without a bus.

const (
	// UpdateActiveWindow payload is
	// "x,y,w,h,resourceClass,resourceName,caption": up to 7 fields, with the
	// geometry minimum being the first 4. The caption is last because it is the
	// one field that can contain a comma, and the split keeps the remainder
	// there.
	payloadParts    = 7
	payloadMinParts = 4

	// The optional identity fields. Both application identifiers are carried
	// because KWin's foreign-toplevel app_id is built from either, depending
	// on the window.
	payloadClassField = 4
	payloadNameField  = 5
	payloadTitleField = 6
)

// parseWindowPayload turns a push from the script into a window, or reports
// it unusable: too few fields, a non-numeric coordinate, or a non-positive
// area. The caller then keeps what it had, so this can never clear a window.
func parseWindowPayload(payload string) (Window, bool) {
	parts := strings.SplitN(payload, ",", payloadParts)
	if len(parts) < payloadMinParts {
		return Window{}, false
	}

	originX, errX := strconv.Atoi(strings.TrimSpace(parts[0]))
	originY, errY := strconv.Atoi(strings.TrimSpace(parts[1]))
	width, errW := strconv.Atoi(strings.TrimSpace(parts[2]))
	height, errH := strconv.Atoi(strings.TrimSpace(parts[3]))

	if errX != nil || errY != nil || errW != nil || errH != nil {
		return Window{}, false
	}

	if width <= 0 || height <= 0 {
		return Window{}, false
	}

	return Window{
		Rect:  image.Rect(originX, originY, originX+width, originY+height),
		Class: payloadField(parts, payloadClassField),
		Name:  payloadField(parts, payloadNameField),
		Title: payloadField(parts, payloadTitleField),
	}, true
}

// payloadField reads an optional trailing field, so a KWin version reporting
// no identity still reports a rectangle.
func payloadField(parts []string, index int) string {
	if index >= len(parts) {
		return ""
	}

	return parts[index]
}
