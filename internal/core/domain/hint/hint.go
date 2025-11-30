package hint

import (
	"context"
	"image"
	"strings"

	"github.com/y3owk1n/neru/internal/core/domain/element"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

const (
	// PrefixLengthCheck is the check for prefix length.
	PrefixLengthCheck = 2
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

// TrieNode represents a node in the hint trie for efficient prefix matching.
type TrieNode struct {
	children map[rune]*TrieNode
	hints    []*Interface
	isEnd    bool
}

// Trie implements a trie data structure for efficient hint prefix matching.
type Trie struct {
	root *TrieNode
}

// NewTrie creates a new empty trie.
func NewTrie() *Trie {
	return &Trie{
		root: &TrieNode{
			children: make(map[rune]*TrieNode),
			hints:    make([]*Interface, 0),
		},
	}
}

// Insert adds a hint to the trie.
func (t *Trie) Insert(hint *Interface) {
	label := strings.ToUpper(hint.Label()) // Pre-normalize to uppercase
	node := t.root

	for _, char := range label {
		if node.children[char] == nil {
			node.children[char] = &TrieNode{
				children: make(map[rune]*TrieNode),
				hints:    make([]*Interface, 0),
			}
		}

		node = node.children[char]
	}

	// Store hint only at the end node
	node.hints = append(node.hints, hint)
	node.isEnd = true
}

// FindByPrefix returns all hints that start with the given prefix.
func (t *Trie) FindByPrefix(prefix string) []*Interface {
	prefix = strings.ToUpper(prefix) // Normalize prefix to uppercase
	node := t.root

	// Traverse to the node corresponding to the prefix
	for _, char := range prefix {
		if node.children[char] == nil {
			return []*Interface{} // No matches
		}

		node = node.children[char]
	}

	// Collect all hints from this node and its descendants
	return t.collectAllHints(node)
}

// collectAllHints recursively collects all hints from end nodes in the subtree.
func (t *Trie) collectAllHints(node *TrieNode) []*Interface {
	var result []*Interface

	// Add hints from current node if it's an end node
	if node.isEnd {
		result = append(result, node.hints...)
	}

	// Recursively collect from children
	for _, child := range node.children {
		result = append(result, t.collectAllHints(child)...)
	}

	return result
}

// Collection manages a collection of hints with efficient lookup.
type Collection struct {
	hints   []*Interface
	byLabel map[string]*Interface
	trie    *Trie
}

// NewCollection creates a new hint collection with indexed lookups.
func NewCollection(hints []*Interface) *Collection {
	collector := &Collection{
		hints:   hints,
		byLabel: make(map[string]*Interface, len(hints)),
		trie:    NewTrie(),
	}

	// Build indexes
	for _, hint := range hints {
		label := hint.Label()
		collector.byLabel[label] = hint
		collector.trie.Insert(hint)
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

	// Use trie for efficient prefix matching
	return c.trie.FindByPrefix(prefix)
}

// Count returns the number of hints in the collection.
func (c *Collection) Count() int {
	return len(c.hints)
}

// Empty returns true if the collection has no hints.
func (c *Collection) Empty() bool {
	return len(c.hints) == 0
}
