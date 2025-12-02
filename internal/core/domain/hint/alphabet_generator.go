package hint

import (
	"context"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/y3owk1n/neru/internal/core/domain/element"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

const (
	// MinCharactersLength is the minimum length for characters.
	MinCharactersLength = 2

	// CountsCapacity is the capacity for counts.
	CountsCapacity = 5

	// MaxLabelLength is the maximum length for a label.
	MaxLabelLength = 4
)

// labelCache caches generated labels by count for instant reuse.
var labelCache sync.Map // map[string][]string (key: "chars:count")

// singleCharCache caches single-character strings to avoid allocations.
var singleCharCache sync.Map // map[rune]string

// stringBuilderPool is a pool of string builders for label generation.
var stringBuilderPool = sync.Pool{
	New: func() any {
		return &strings.Builder{}
	},
}

// AlphabetGenerator generates hint labels using an alphabet-based strategy.
// It uses a prefix-avoidance algorithm to ensure no single-character label
// conflicts with the start of a multi-character label.
type AlphabetGenerator struct {
	characters       string
	uppercaseChars   string
	maxHints         int
	uppercaseRuneMap map[rune]rune
}

// NewAlphabetGenerator creates a new alphabet-based hint generator.
func NewAlphabetGenerator(characters string) (*AlphabetGenerator, error) {
	if len(characters) < MinCharactersLength {
		return nil, derrors.Newf(
			derrors.CodeInvalidInput,
			"characters must have at least 2 characters, got %d",
			len(characters),
		)
	}

	// Build uppercase mapping
	uppercaseRuneMap := make(map[rune]rune)

	var uppercaseBuilder strings.Builder

	for _, rune := range characters {
		upper := unicode.ToUpper(rune)
		uppercaseRuneMap[rune] = upper
		uppercaseBuilder.WriteRune(upper)
	}

	uppercaseChars := uppercaseBuilder.String()
	charCount := len(characters)

	// Calculate max hints: up to 3 chars
	// Max capacity for length 3 prefix-free code is N^3
	n := charCount
	maxHints := n * n * n

	return &AlphabetGenerator{
		characters:       characters,
		uppercaseChars:   uppercaseChars,
		maxHints:         maxHints,
		uppercaseRuneMap: uppercaseRuneMap,
	}, nil
}

// Generate creates hints for the given elements.
func (g *AlphabetGenerator) Generate(
	ctx context.Context,
	elements []*element.Element,
) ([]*Interface, error) {
	if len(elements) == 0 {
		return nil, nil
	}

	if len(elements) > g.maxHints {
		return nil, derrors.Newf(
			derrors.CodeHintGenerationFailed,
			"too many elements: %d exceeds maximum %d",
			len(elements),
			g.maxHints,
		)
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	// Sort elements by position (top-to-bottom, left-to-right)
	// Sort in-place if we can modify the input, otherwise copy
	sorted := elements
	if len(elements) > 0 {
		// Create a copy to avoid modifying the input slice
		sorted = make([]*element.Element, len(elements))
		copy(sorted, elements)
	}

	sort.Slice(sorted, func(i, j int) bool {
		boundI, boundJ := sorted[i].Bounds(), sorted[j].Bounds()
		// Compare Y first (top to bottom)
		if boundI.Min.Y != boundJ.Min.Y {
			return boundI.Min.Y < boundJ.Min.Y
		}
		// Then X (left to right)
		return boundI.Min.X < boundJ.Min.X
	})

	// Generate labels (with caching)
	labels := g.generateLabels(len(sorted))

	// Create hints
	hints := make([]*Interface, len(sorted))
	for index, element := range sorted {
		// Use element center as hint position
		position := element.Center()

		hint, err := NewHint(labels[index], element, position)
		if err != nil {
			return nil, derrors.Wrapf(
				err,
				derrors.CodeHintGenerationFailed,
				"failed to create hint %d: %v",
				index,
				err,
			)
		}

		hints[index] = hint
	}

	return hints, nil
}

// MaxHints returns the maximum number of hints this generator can create.
func (g *AlphabetGenerator) MaxHints() int {
	return g.maxHints
}

// Characters returns the character set used for hint generation.
func (g *AlphabetGenerator) Characters() string {
	return g.characters
}

// UpdateCharacters updates the character set and recalculates max hints.
func (g *AlphabetGenerator) UpdateCharacters(characters string) error {
	if len(characters) < MinCharactersLength {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"characters must have at least 2 characters, got %d",
			len(characters),
		)
	}

	// Build uppercase mapping
	uppercaseRuneMap := make(map[rune]rune)

	var uppercaseBuilder strings.Builder

	for _, rune := range characters {
		upper := unicode.ToUpper(rune)
		uppercaseRuneMap[rune] = upper
		uppercaseBuilder.WriteRune(upper)
	}

	uppercaseChars := uppercaseBuilder.String()
	charCount := len(characters)
	// Max capacity for length 3 prefix-free code is N^3
	n := charCount
	maxHints := n * n * n

	g.characters = characters
	g.uppercaseChars = uppercaseChars
	g.maxHints = maxHints
	g.uppercaseRuneMap = uppercaseRuneMap

	// Pre-cache single character strings
	for _, r := range characters {
		if _, ok := singleCharCache.Load(r); !ok {
			singleCharCache.Store(r, string(r))
		}
	}

	return nil
}

// generateLabels generates alphabet-based hint labels using a prefix-avoidance strategy.
// It ensures no label is a prefix of another to prevent ambiguity during input.
// Returns uppercase labels sorted by length then alphabetically.
// Uses a level-based approach where each level represents labels of increasing length.
// Results are cached for instant reuse on repeated counts.
func (g *AlphabetGenerator) generateLabels(count int) []string {
	if count == 0 {
		return nil
	}

	// Check cache first (key: "chars:count")
	cacheKey := g.uppercaseChars + ":" + string(rune(count))
	if cached, ok := labelCache.Load(cacheKey); ok {
		if labels, ok := cached.([]string); ok {
			return labels
		}
	}

	// Generate labels if not cached
	labels := g.computeLabels(count)

	// Store in cache for future use
	labelCache.Store(cacheKey, labels)

	return labels
}

// computeLabels performs the actual label generation (extracted for caching).
func (g *AlphabetGenerator) computeLabels(count int) []string {
	chars := []rune(g.uppercaseChars)
	numChars := len(chars)
	labels := make([]string, 0, count)

	// Calculate distribution of labels across different lengths (levels)
	// counts[i] stores number of labels of length i+1
	// Uses greedy algorithm to minimize average label length while ensuring prefix-free property
	counts := make([]int, 0, CountsCapacity)

	remainingTarget := count
	availableSlots := numChars // Slots available at current level (length 1)

	// Determine how many labels to keep at each level
	for remainingTarget > 0 {
		// Calculate capacity if all current slots are expanded to next level
		nextLevelCapacity := availableSlots * numChars

		var keep int

		switch {
		case availableSlots >= remainingTarget:
			// Can satisfy remaining target at current level
			keep = remainingTarget
		case nextLevelCapacity < remainingTarget:
			// Need to expand everything to reach target, keep none at current level
			keep = 0
		default:
			// Keep as many as possible at current level while ensuring next level can handle remainder
			// Formula derived from: availableSlots*N - keep*(N-1) >= remainingTarget
			keep = (availableSlots*numChars - remainingTarget) / (numChars - 1)
		}

		counts = append(counts, keep)
		remainingTarget -= keep

		// Update slots for next level: remaining slots expanded by branching factor
		availableSlots = (availableSlots - keep) * numChars

		if availableSlots == 0 && remainingTarget > 0 {
			// Should not happen if maxHints check passed
			break
		}
	}

	var current []int

	// Generate labels for each level using base-N arithmetic
	for level, keep := range counts {
		length := level + 1

		if length == 1 {
			// Generate single-character labels
			for i := range keep {
				char := chars[i]
				if cached, ok := singleCharCache.Load(char); ok {
					if str, ok := cached.(string); ok {
						labels = append(labels, str)

						continue
					}
				}

				labels = append(labels, string(char))
			}
			// Start next level from after the kept labels
			current = []int{keep}
		} else {
			// Generate multi-character labels by expanding prefixes
			// 'current' represents the starting point in base-N numbering

			// Ensure current array has correct length for this level
			for len(current) < length {
				current = append(current, 0)
			}

			// Generate 'keep' labels starting from current position
			for range keep {
				// Build label string from current indices using pooled builder
				stringBuilder, ok := stringBuilderPool.Get().(*strings.Builder)
				if !ok {
					stringBuilder = &strings.Builder{}
				}

				stringBuilder.Reset()
				stringBuilder.Grow(length)

				for _, index := range current {
					stringBuilder.WriteRune(chars[index])
				}

				labels = append(labels, stringBuilder.String())
				stringBuilderPool.Put(stringBuilder)

				// Increment current position (like adding 1 in base-N)
				for pos := len(current) - 1; pos >= 0; pos-- {
					current[pos]++
					if current[pos] < numChars {
						break
					}
					// Carry over to next position
					current[pos] = 0
				}
			}
		}
	}

	return labels
}
