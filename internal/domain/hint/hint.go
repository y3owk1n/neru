package hint

import (
	"cmp"
	"context"
	"image"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/screentext"
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

// Generator generates hint labels for UI elements.
type Generator interface {
	// Generate creates hints for the given elements.
	Generate(ctx context.Context, elements []*element.Element) ([]*Interface, error)

	// MaxHints returns the maximum number of hints this generator can create.
	MaxHints() int

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
				}
			}

			node = node.asciiChildren[index]
		} else {
			if node.otherChildren[char] == nil {
				node.otherChildren[char] = &TrieNode{
					otherChildren: make(map[rune]*TrieNode),
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
	prefix = strings.ToUpper(prefix)
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

	// searchTexts holds each hint's element texts folded for search. The
	// first search builds it and later ones reuse it. A search runs on every
	// keystroke, folding is most of its cost, and the hints do not change
	// while the user types.
	searchOnce  sync.Once
	searchTexts [][]searchText
}

// searchText is an element text as search matches it: spelled in lowercase
// ASCII by screentext.Fold, with the character each byte came from and the
// bytes that start a word.
type searchText struct {
	text   string
	spans  [][2]int
	starts []bool
}

func foldForSearch(text string) searchText {
	folded, spans := screentext.Fold(text)

	return searchText{
		text:   strings.ToLower(folded),
		spans:  spans,
		starts: screentext.WordStarts(folded),
	}
}

// matchRank is how well a query matches a text, best first.
type matchRank int

const (
	// rankExact is the whole text.
	rankExact matchRank = iota
	// rankPrefix is the start of the text.
	rankPrefix
	// rankWordStart is the start of a word in it.
	rankWordStart
	// rankInitials is the initials of its words (screentext.Initials).
	rankInitials
	// rankInWord is inside a word, starting where a character does, so en
	// finds no 深 (Shen).
	rankInWord
	// rankNone is no match.
	rankNone
)

// rank is how well query matches t.
func (t searchText) rank(query string) matchRank {
	switch {
	case t.text == query:
		return rankExact
	case strings.HasPrefix(t.text, query):
		return rankPrefix
	}

	inWord := false

	for scanFrom := 0; ; {
		found := strings.Index(t.text[scanFrom:], query)
		if found < 0 {
			break
		}

		start := scanFrom + found
		if t.starts[start] {
			return rankWordStart
		}

		inWord = inWord || start == 0 || t.spans[start] != t.spans[start-1]
		scanFrom = start + 1
	}

	for at := range len(t.text) {
		if len(screentext.Initials(t.text, t.starts, query, at)) > 0 {
			return rankInitials
		}
	}

	if inWord {
		return rankInWord
	}

	return rankNone
}

// bestRank is how well query matches the best of texts.
func bestRank(texts []searchText, query string) matchRank {
	best := rankNone
	for _, text := range texts {
		best = min(best, text.rank(query))
	}

	return best
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

// FilterBySearch returns the hints whose element text contains query, or
// matches it by its words' initials, both as screentext.Fold spells them in
// lowercase, so cafe finds Café, nihao and nh find 你好, and sas finds Save
// As. The best matches come first: the whole text, then its start, the start
// of a word, the initials and last a match inside a word. When query has a
// space, it also returns the hints whose text matches the words before the
// last space and whose label starts with the word after it, so "save a" keeps
// the "save" matches labeled A. A label match never hides a text match. Hints
// kept by label carry the typed label as their matched prefix.
func (c *Collection) FilterBySearch(query string) *Collection {
	if query == "" {
		return c
	}

	normalizedQuery := foldForSearch(query).text

	space := strings.LastIndex(query, " ")
	readsLabel := space >= 0
	normalizedHead := ""
	labelPrefix := ""

	if readsLabel {
		normalizedHead = foldForSearch(strings.TrimRight(query[:space], " ")).text
		labelPrefix = strings.ToUpper(query[space+1:])
	}

	type rankedHint struct {
		hint *Interface
		rank matchRank
	}

	ranked := make([]rankedHint, 0, len(c.hints))

	for i, texts := range c.normalizedSearchTexts() {
		hint := c.hints[i]

		if readsLabel && strings.HasPrefix(hint.labelUpper, labelPrefix) {
			rank := rankExact
			if normalizedHead != "" {
				rank = bestRank(texts, normalizedHead)
			}

			if rank != rankNone {
				// The whole query can match better than the words before
				// the label, as save as matches Save As whole.
				rank = min(rank, bestRank(texts, normalizedQuery))

				if labelPrefix != "" {
					hint = hint.WithMatchedPrefix(labelPrefix)
				}

				ranked = append(ranked, rankedHint{hint: hint, rank: rank})

				continue
			}
		}

		if rank := bestRank(texts, normalizedQuery); rank != rankNone {
			ranked = append(ranked, rankedHint{hint: hint, rank: rank})
		}
	}

	slices.SortStableFunc(ranked, func(a, b rankedHint) int { return cmp.Compare(a.rank, b.rank) })

	filtered := make([]*Interface, len(ranked))
	for i, entry := range ranked {
		filtered[i] = entry.hint
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

// Count returns the number of hints in the collection.
func (c *Collection) Count() int {
	return len(c.hints)
}

// normalizedSearchTexts returns the texts FilterBySearch matches against, one
// entry per hint, nil for a hint without an element.
func (c *Collection) normalizedSearchTexts() [][]searchText {
	c.searchOnce.Do(func() {
		c.searchTexts = make([][]searchText, len(c.hints))

		for index, hint := range c.hints {
			elem := hint.Element()
			if elem == nil {
				continue
			}

			c.searchTexts[index] = []searchText{
				foldForSearch(elem.Title()),
				foldForSearch(elem.Description()),
				foldForSearch(elem.Value()),
				foldForSearch(elem.SearchText()),
			}
		}
	})

	return c.searchTexts
}
