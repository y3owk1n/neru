package badge

import (
	"image"
	"strconv"
)

// searchPrompt is what the badge shows before anything has been typed. It is
// the macOS overlay's string (`drawSearchInputInRect`, overlay_darwin.m), kept
// identical so the same badge reads the same way on every backend.
const searchPrompt = "/ Search hints"

// SearchLabel returns the single line a hint-search badge shows: a prompt while
// the query is empty, and the query with the number of hints still matching it
// once it is not.
//
// It lives here rather than in a backend for the reason PlaceHint does: the
// badge is a display of state the app already holds, and where it is worded is
// not where it is painted. Linux reads it; macOS builds the same string in
// Objective-C (`drawSearchInputInRect`) and Windows composes its own inline,
// which is why the count reads `/ sav  3` on two platforms and `/ sav  3 /` on
// the third. Bringing that third one here is a change to what a Windows user
// sees, so it is not made in passing.
func SearchLabel(query string, resultCount int) string {
	if query == "" {
		return searchPrompt
	}

	return "/ " + query + "  " + strconv.Itoa(resultCount)
}

// SearchBounds returns the rectangle a hint-search badge occupies: the label
// sized by the shared font estimates, padded on both axes, anchored at position
// and never narrower than minWidth.
//
// minWidth is `hints.search_input_ui.width`, and it is a floor rather than the
// width because the label is the point of the badge: a query that outgrew the
// configured box widens it instead of being drawn past its own border. macOS
// takes the same maximum (`drawSearchInputWithAttrString`).
func SearchBounds(
	position image.Point,
	minWidth int,
	label string,
	fontSize float64,
	paddingX, paddingY int,
) image.Rectangle {
	bounds := Bounds(position.X, position.Y, 0, 0, label, fontSize, paddingX, paddingY)

	if bounds.Dx() < minWidth {
		bounds.Max.X = bounds.Min.X + minWidth
	}

	return bounds
}
