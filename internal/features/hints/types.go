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

// GetLabel returns the hint label.
func (h *Hint) GetLabel() string {
	return h.label
}

// GetPosition returns the hint position.
func (h *Hint) GetPosition() image.Point {
	return h.position
}

// GetSize returns the hint size.
func (h *Hint) GetSize() image.Point {
	return h.size
}

// GetMatchedPrefix returns the matched prefix.
func (h *Hint) GetMatchedPrefix() string {
	return h.matchedPrefix
}
