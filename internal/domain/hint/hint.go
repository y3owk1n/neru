package hint

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain/element"
	derrors "github.com/y3owk1n/neru/internal/errors"
)

// Interface represents a labeled UI element for keyboard-driven navigation.
// Hints are immutable after creation.
type Interface struct {
	label         string
	element       *element.Element
	position      image.Point
	matchedPrefix string
}

// NewHint creates a new hint with validation.
func NewHint(label string, element *element.Element, position image.Point) (*Interface, error) {
	if label == "" {
		return nil, derrors.New(derrors.CodeInvalidInput, "hint label cannot be empty")
	}

	if element == nil {
		return nil, derrors.New(derrors.CodeInvalidInput, "hint element cannot be nil")
	}

	return &Interface{
		label:    label,
		element:  element,
		position: position,
	}, nil
}

// Label returns the hint label.
func (h *Interface) Label() string {
	return h.label
}

// Element returns the associated element.
func (h *Interface) Element() *element.Element {
	return h.element
}

// Position returns the hint display position.
func (h *Interface) Position() image.Point {
	return h.position
}

// MatchedPrefix returns the currently matched prefix.
func (h *Interface) MatchedPrefix() string {
	return h.matchedPrefix
}

// WithMatchedPrefix returns a new hint with the matched prefix set.
func (h *Interface) WithMatchedPrefix(prefix string) *Interface {
	return &Interface{
		label:         h.label,
		element:       h.element,
		position:      h.position,
		matchedPrefix: prefix,
	}
}

// Bounds returns the bounding rectangle for the hint.
func (h *Interface) Bounds() image.Rectangle {
	return h.element.Bounds()
}

// IsVisible checks if the hint is visible within the given screen bounds.
func (h *Interface) IsVisible(screenBounds image.Rectangle) bool {
	return h.element.IsVisible(screenBounds)
}

// MatchesLabel checks if the hint label matches the given input.
func (h *Interface) MatchesLabel(input string) bool {
	return h.label == input
}

// HasPrefix checks if the hint label starts with the given prefix.
func (h *Interface) HasPrefix(prefix string) bool {
	if len(prefix) > len(h.label) {
		return false
	}

	return h.label[:len(prefix)] == prefix
}

// Generator generates hint labels for UI elements.
type Generator interface {
	// Generate creates hints for the given elements.
	Generate(ctx context.Context, elements []*element.Element) ([]*Interface, error)

	// MaxHints returns the maximum number of hints this generator can create.
	MaxHints() int

	// Characters returns the character set used for hint generation.
	Characters() string
}

// Collection manages a collection of hints with efficient lookup.
type Collection struct {
	hints   []*Interface
	byLabel map[string]*Interface
	prefix1 map[byte][]*Interface
	prefix2 map[string][]*Interface
}

// NewCollection creates a new hint collection with indexed lookups.
func NewCollection(hints []*Interface) *Collection {
	collector := &Collection{
		hints:   hints,
		byLabel: make(map[string]*Interface, len(hints)),
		prefix1: make(map[byte][]*Interface),
		prefix2: make(map[string][]*Interface),
	}

	// Build indexes
	for _, hint := range hints {
		label := hint.Label()
		collector.byLabel[label] = hint

		if len(label) >= 1 {
			first := label[0]
			collector.prefix1[first] = append(collector.prefix1[first], hint)
		}

		if len(label) >= 2 {
			prefix := label[:2]
			collector.prefix2[prefix] = append(collector.prefix2[prefix], hint)
		}
	}

	return collector
}

// All returns all hints in the collection.
func (c *Collection) All() []*Interface {
	return c.hints
}

// FindByLabel finds a hint by its exact label.
func (c *Collection) FindByLabel(label string) *Interface {
	return c.byLabel[label]
}

// FilterByPrefix returns all hints that start with the given prefix.
func (c *Collection) FilterByPrefix(prefix string) []*Interface {
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
	var filteredHints []*Interface

	for _, hint := range c.hints {
		if hint.HasPrefix(prefix) {
			filteredHints = append(filteredHints, hint)
		}
	}

	return filteredHints
}

// Count returns the number of hints in the collection.
func (c *Collection) Count() int {
	return len(c.hints)
}

// Empty returns true if the collection has no hints.
func (c *Collection) Empty() bool {
	return len(c.hints) == 0
}
