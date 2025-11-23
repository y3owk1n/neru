package hint

import (
	"context"
	"errors"
	"image"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// Hint represents a labeled UI element for keyboard-driven navigation.
// Hints are immutable after creation.
type Hint struct {
	label         string
	elem          *element.Element
	position      image.Point
	matchedPrefix string
}

// NewHint creates a new hint with validation.
func NewHint(label string, elem *element.Element, position image.Point) (*Hint, error) {
	if label == "" {
		return nil, errors.New("hint label cannot be empty")
	}

	if elem == nil {
		return nil, errors.New("hint element cannot be nil")
	}

	return &Hint{
		label:    label,
		elem:     elem,
		position: position,
	}, nil
}

// Label returns the hint label.
func (h *Hint) Label() string {
	return h.label
}

// Element returns the associated element.
func (h *Hint) Element() *element.Element {
	return h.elem
}

// Position returns the hint display position.
func (h *Hint) Position() image.Point {
	return h.position
}

// MatchedPrefix returns the currently matched prefix.
func (h *Hint) MatchedPrefix() string {
	return h.matchedPrefix
}

// WithMatchedPrefix returns a new hint with the matched prefix set.
func (h *Hint) WithMatchedPrefix(prefix string) *Hint {
	return &Hint{
		label:         h.label,
		elem:          h.elem,
		position:      h.position,
		matchedPrefix: prefix,
	}
}

// Bounds returns the bounding rectangle for the hint.
func (h *Hint) Bounds() image.Rectangle {
	return h.elem.Bounds()
}

// IsVisible checks if the hint is visible within the given screen bounds.
func (h *Hint) IsVisible(screenBounds image.Rectangle) bool {
	return h.elem.IsVisible(screenBounds)
}

// MatchesLabel checks if the hint label matches the given input.
func (h *Hint) MatchesLabel(input string) bool {
	return h.label == input
}

// HasPrefix checks if the hint label starts with the given prefix.
func (h *Hint) HasPrefix(prefix string) bool {
	if len(prefix) > len(h.label) {
		return false
	}
	return h.label[:len(prefix)] == prefix
}

// Generator generates hint labels for UI elements.
type Generator interface {
	// Generate creates hints for the given elements.
	Generate(ctx context.Context, elements []*element.Element) ([]*Hint, error)

	// MaxHints returns the maximum number of hints this generator can create.
	MaxHints() int

	// Characters returns the character set used for hint generation.
	Characters() string
}

// Collection manages a collection of hints with efficient lookup.
type Collection struct {
	hints   []*Hint
	byLabel map[string]*Hint
	prefix1 map[byte][]*Hint
	prefix2 map[string][]*Hint
}

// NewCollection creates a new hint collection with indexed lookups.
func NewCollection(hints []*Hint) *Collection {
	c := &Collection{
		hints:   hints,
		byLabel: make(map[string]*Hint, len(hints)),
		prefix1: make(map[byte][]*Hint),
		prefix2: make(map[string][]*Hint),
	}

	// Build indexes
	for _, h := range hints {
		label := h.Label()
		c.byLabel[label] = h

		if len(label) >= 1 {
			first := label[0]
			c.prefix1[first] = append(c.prefix1[first], h)
		}

		if len(label) >= 2 {
			prefix := label[:2]
			c.prefix2[prefix] = append(c.prefix2[prefix], h)
		}
	}

	return c
}

// All returns all hints in the collection.
func (c *Collection) All() []*Hint {
	return c.hints
}

// FindByLabel finds a hint by its exact label.
func (c *Collection) FindByLabel(label string) *Hint {
	return c.byLabel[label]
}

// FilterByPrefix returns all hints that start with the given prefix.
func (c *Collection) FilterByPrefix(prefix string) []*Hint {
	if prefix == "" {
		return c.hints
	}

	// Fast path for single character
	if len(prefix) == 1 {
		return c.prefix1[prefix[0]]
	}

	// Fast path for two characters
	if len(prefix) == 2 {
		return c.prefix2[prefix]
	}

	// Slow path for longer prefixes
	var result []*Hint
	for _, h := range c.hints {
		if h.HasPrefix(prefix) {
			result = append(result, h)
		}
	}
	return result
}

// Count returns the number of hints in the collection.
func (c *Collection) Count() int {
	return len(c.hints)
}

// Empty returns true if the collection has no hints.
func (c *Collection) Empty() bool {
	return len(c.hints) == 0
}
