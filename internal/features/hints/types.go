package hints

import "image"

// Hint represents a hint to be displayed on the overlay.
type Hint struct {
	label         string
	position      image.Point
	size          image.Point
	matchedPrefix string
}

// NewHint creates a new Hint with the specified values.
func NewHint(label string, position, size image.Point, matchedPrefix string) *Hint {
	return &Hint{
		label:         label,
		position:      position,
		size:          size,
		matchedPrefix: matchedPrefix,
	}
}

// Label returns the hint label.
func (h *Hint) Label() string {
	return h.label
}

// Position returns the hint position.
func (h *Hint) Position() image.Point {
	return h.position
}

// Size returns the hint size.
func (h *Hint) Size() image.Point {
	return h.size
}

// MatchedPrefix returns the matched prefix.
func (h *Hint) MatchedPrefix() string {
	return h.matchedPrefix
}
