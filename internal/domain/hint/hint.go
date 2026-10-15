package hint

import (
	"context"
	"image"
	"sort"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// Interface is a labeled UI element for keyboard-driven navigation.
// Hints are immutable after creation.
type Interface struct {
	label         string
	labelUpper    string
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
		label:      label,
		labelUpper: strings.ToUpper(label),
		element:    element,
		position:   position,
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
		labelUpper:    h.labelUpper,
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

	// LabelDirection returns the label enumeration direction the generator
	// is configured to use. Implementations expose this so the hint service
	// can multiplex generators by direction.
	LabelDirection() LabelDirection
}

// TrieNode is a node in the hint trie for efficient prefix matching.
type TrieNode struct {
	asciiChildren [26]*TrieNode // Optimized for A-Z
	otherChildren map[rune]*TrieNode
	hints         []*Interface
	isEnd         bool
}

// Trie implements a trie data structure for efficient hint prefix matching.
type Trie struct {
	root *TrieNode
}

// NewTrie creates a new empty trie.
func NewTrie() *Trie {
	return &Trie{
		root: &TrieNode{
			otherChildren: make(map[rune]*TrieNode),
			hints:         nil, // Use nil instead of empty slice
		},
	}
}

// Insert adds a hint to the trie.
func (t *Trie) Insert(hint *Interface) {
	label := hint.labelUpper
	node := t.root

	for _, char := range label {
		if char >= 'A' && char <= 'Z' {
			index := char - 'A'
			if node.asciiChildren[index] == nil {
				node.asciiChildren[index] = &TrieNode{
					otherChildren: make(map[rune]*TrieNode),
					hints:         nil,
				}
			}

			node = node.asciiChildren[index]
		} else {
			if node.otherChildren[char] == nil {
				node.otherChildren[char] = &TrieNode{
					otherChildren: make(map[rune]*TrieNode),
					hints:         nil,
				}
			}

			node = node.otherChildren[char]
		}
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
		if char >= 'A' && char <= 'Z' {
			index := char - 'A'
			if node.asciiChildren[index] == nil {
				return nil
			}

			node = node.asciiChildren[index]
		} else {
			if node.otherChildren[char] == nil {
				return nil
			}

			node = node.otherChildren[char]
		}
	}

	return t.collectAllHints(node)
}

// collectAllHints recursively collects all hints from end nodes in the subtree.
func (t *Trie) collectAllHints(node *TrieNode) []*Interface {
	var result []*Interface
	t.collectAllHintsInto(node, &result)

	// Sort for deterministic order
	sort.Slice(result, func(i, j int) bool {
		return result[i].Label() < result[j].Label()
	})

	return result
}

// collectAllHintsInto collects hints into the provided result slice to avoid intermediate allocations.
func (t *Trie) collectAllHintsInto(node *TrieNode, result *[]*Interface) {
	if node.isEnd {
		*result = append(*result, node.hints...)
	}

	for _, child := range node.asciiChildren {
		if child != nil {
			t.collectAllHintsInto(child, result)
		}
	}

	for _, child := range node.otherChildren {
		t.collectAllHintsInto(child, result)
	}
}

// Collection manages a collection of hints with efficient lookup.
type Collection struct {
	hints   []*Interface
	byLabel map[string]*Interface
	trie    *Trie

	// searchTexts holds each hint's element texts normalized for search. The
	// first search builds it and later ones reuse it. A search runs on every
	// keystroke, normalizing is most of its cost, and the hints do not change
	// while the user types.
	searchOnce  sync.Once
	searchTexts [][]string
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

	return c.trie.FindByPrefix(prefix)
}

// FilterBySearch returns the hints whose element text contains query. When
// query has a space, it also returns the hints whose text contains the words
// before the last space and whose label starts with the word after it, so
// "save a" keeps the "save" matches labeled A. A label match never hides a
// text match. Hints kept by label carry the typed label as their matched
// prefix.
func (c *Collection) FilterBySearch(query string) *Collection {
	if query == "" {
		return c
	}

	normalizedQuery := normalizeForSearch(query)

	space := strings.LastIndex(query, " ")
	readsLabel := space >= 0
	normalizedHead := ""
	labelPrefix := ""

	if readsLabel {
		normalizedHead = normalizeForSearch(strings.TrimRight(query[:space], " "))
		labelPrefix = strings.ToUpper(query[space+1:])
	}

	filtered := make([]*Interface, 0, len(c.hints))

	for i, texts := range c.normalizedSearchTexts() {
		hint := c.hints[i]

		if readsLabel && strings.HasPrefix(hint.labelUpper, labelPrefix) &&
			(normalizedHead == "" || containsAny(texts, normalizedHead)) {
			if labelPrefix != "" {
				hint = hint.WithMatchedPrefix(labelPrefix)
			}

			filtered = append(filtered, hint)

			continue
		}

		if containsAny(texts, normalizedQuery) {
			filtered = append(filtered, hint)
		}
	}

	return NewCollection(filtered)
}

// WithoutMatchedPrefixes returns the collection with every hint's matched
// prefix cleared, or the collection itself when no hint carries one.
func (c *Collection) WithoutMatchedPrefixes() *Collection {
	cleared := make([]*Interface, len(c.hints))
	changed := false

	for i, hint := range c.hints {
		cleared[i] = hint
		if hint.matchedPrefix != "" {
			cleared[i] = hint.WithMatchedPrefix("")
			changed = true
		}
	}

	if !changed {
		return c
	}

	return NewCollection(cleared)
}

func containsAny(texts []string, query string) bool {
	for _, text := range texts {
		if strings.Contains(text, query) {
			return true
		}
	}

	return false
}

func normalizeForSearch(text string) string {
	if text == "" {
		return ""
	}

	searchCaser := cases.Fold()
	folded := searchCaser.String(text)

	searchTransformer := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

	normalized, _, err := transform.String(searchTransformer, folded)
	if err != nil {
		return folded
	}

	return normalized
}

// Count returns the number of hints in the collection.
func (c *Collection) Count() int {
	return len(c.hints)
}

// Empty returns true if the collection has no hints.
func (c *Collection) Empty() bool {
	return len(c.hints) == 0
}

// normalizedSearchTexts returns the texts FilterBySearch matches against, one
// entry per hint, nil for a hint without an element.
func (c *Collection) normalizedSearchTexts() [][]string {
	c.searchOnce.Do(func() {
		c.searchTexts = make([][]string, len(c.hints))

		for index, hint := range c.hints {
			elem := hint.Element()
			if elem == nil {
				continue
			}

			c.searchTexts[index] = []string{
				normalizeForSearch(elem.Title()),
				normalizeForSearch(elem.Description()),
				normalizeForSearch(elem.Value()),
				normalizeForSearch(elem.SearchText()),
			}
		}
	})

	return c.searchTexts
}
