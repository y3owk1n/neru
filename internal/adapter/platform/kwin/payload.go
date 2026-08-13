//go:build linux

package kwin

import (
	"image"
	"strconv"
	"strings"
)

// The wire format between the KWin script and this package.
//
// It is one comma-separated string rather than typed D-Bus arguments because
// the sender is JavaScript inside KWin, whose number marshaling is not worth
// depending on. Parsing it is kept apart from the D-Bus method that receives it
// so that what counts as a usable push can be decided — and tested — without
// standing up a bus.

const (
	// UpdateActiveWindow payload is
	// "x,y,w,h,resourceClass,resourceName,caption": up to 7 fields, with the
	// geometry minimum being the first 4. The caption is last because it is the
	// one field that can contain a comma, and the split keeps the remainder
	// there.
	payloadParts    = 7
	payloadMinParts = 4

	// The identity fields, indexed as the ones a payload may stop short of.
	// Both application identifiers are carried because neither is reliably the
	// one a reader will be comparing against: KWin's own foreign-toplevel app_id
	// is built from one of them, and which one differs by window (an XWayland
	// Firefox is resourceClass "firefox" and resourceName "navigator").
	payloadClassField = 4
	payloadNameField  = 5
	payloadTitleField = 6
)

// parseWindowPayload turns a push from the script into a window, or reports
// that there is nothing usable in it.
//
// The script is remote code as far as this process is concerned, so every way a
// payload can fail to describe a window on screen — too few fields, a
// non-numeric coordinate, a zero or negative area — is one answer: not usable.
// What the caller does with that is keep whatever it already had, which is why
// this says nothing about the previous window and cannot be told to clear one.
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

// payloadField reads an optional trailing payload field. The identity fields
// are the ones a payload may stop short of — they are a correlation key, and a
// KWin version that reports none of them should still report a rectangle.
func payloadField(parts []string, index int) string {
	if index >= len(parts) {
		return ""
	}

	return parts[index]
}
