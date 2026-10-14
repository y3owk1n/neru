// Package segment finds hint targets in a screen frame from its pixels alone:
// no accessibility tree, no text recognition, no model. A screen is flat color
// plus glyph ink rather than a photograph, so it skips the blur, gradient and
// dilation passes a natural-image detector needs. It marks every sharp color
// step, joins touching ink into blobs over row runs, and sorts the blobs:
// separators and rules are dropped, a shell around something (a button, a
// field, a round key) is one control, and the rest are glyphs, grouped into
// words by gaps measured in glyph heights and split by ink color, so every
// word, and every link inside a sentence, is a target of its own. A photo is
// read again with a higher threshold, so what is drawn on it (desktop icons, a
// hero's buttons, a player's controls) is found rather than swallowed.
//
// It is pure Go with no cgo, so it builds everywhere Neru does, CGO_ENABLED=0
// included. Each vision adapter supplies the frame from its own capture
// backend and maps the boxes back to global coordinates.
//
// The frame is screen content. Detect reads it and returns rectangles; nothing
// here logs, copies past the call, or retains the pixels.
package segment
