package hints

import "image"

// Hint represents a hint to be displayed on the overlay.
type Hint struct {
	Label         string
	Position      image.Point
	Size          image.Point
	MatchedPrefix string
}

// GetLabel returns the hint label.
func (h *Hint) GetLabel() string {
	return h.Label
}

// GetPosition returns the hint position.
func (h *Hint) GetPosition() image.Point {
	return h.Position
}

// GetSize returns the hint size.
func (h *Hint) GetSize() image.Point {
	return h.Size
}

// GetMatchedPrefix returns the matched prefix.
func (h *Hint) GetMatchedPrefix() string {
	return h.MatchedPrefix
}
