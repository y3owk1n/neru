package hint

import (
	"context"
	"sort"
	"strings"
	"unicode"

	"github.com/y3owk1n/neru/internal/domain/element"
	derrors "github.com/y3owk1n/neru/internal/errors"
)

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
	if len(characters) < 2 {
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
	context context.Context,
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
	case <-context.Done():
		return nil, derrors.Wrap(context.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	// Sort elements by position (top-to-bottom, left-to-right)
	sorted := make([]*element.Element, len(elements))
	copy(sorted, elements)
	sort.Slice(sorted, func(i, j int) bool {
		boundI, boundJ := sorted[i].Bounds(), sorted[j].Bounds()
		// Compare Y first (top to bottom)
		if boundI.Min.Y != boundJ.Min.Y {
			return boundI.Min.Y < boundJ.Min.Y
		}
		// Then X (left to right)
		return boundI.Min.X < boundJ.Min.X
	})

	// Generate labels
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
	if len(characters) < 2 {
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

	return nil
}

// generateLabels generates alphabet-based hint labels using a prefix-avoidance strategy.
// Returns uppercase labels.
// generateLabels generates alphabet-based hint labels using a prefix-avoidance strategy.
// Returns uppercase labels sorted by length then alphabetically.
func (g *AlphabetGenerator) generateLabels(count int) []string {
	if count == 0 {
		return nil
	}

	chars := []rune(g.uppercaseChars)
	numChars := len(chars)
	labels := make([]string, 0, count)

	// Calculate how many labels of each length we need
	// counts[i] stores number of labels of length i+1
	// We assume max length won't exceed 10 for reasonable counts
	counts := make([]int, 0, 5)

	remainingTarget := count
	availableSlots := numChars // Slots available at current level

	// Determine counts for each level
	for remainingTarget > 0 {
		// Calculate max capacity if we expand everything to next level
		// We check if next level can hold the target to decide if we keep any at current level
		nextLevelCapacity := availableSlots * numChars

		var keep int

		switch {
		case availableSlots >= remainingTarget:
			// We can satisfy the rest of the target at this level
			keep = remainingTarget
		case nextLevelCapacity < remainingTarget:
			// Even expanding everything isn't enough for next level?
			// This implies we need to go deeper.
			// We keep 0 at this level to maximize expansion capacity.
			keep = 0
		default:
			// We can satisfy target at next level.
			// We want to keep as many as possible at this level.
			// Formula: available*N - k*(N-1) >= target
			// k <= (available*N - target) / (N-1)
			keep = (availableSlots*numChars - remainingTarget) / (numChars - 1)
		}

		counts = append(counts, keep)
		remainingTarget -= keep

		// Update available slots for next level
		// We used 'keep' slots. The remaining 'availableSlots - keep' are expanded.
		availableSlots = (availableSlots - keep) * numChars

		if availableSlots == 0 && remainingTarget > 0 {
			// Should not happen if maxHints check passed
			break
		}
	}

	var current []int

	for level, keep := range counts {
		length := level + 1

		// If this is the first level
		if length == 1 {
			for i := range keep {
				labels = append(labels, string(chars[i]))
			}
			// The start for next level is 'keep'
			current = []int{keep}
		} else {
			// We need to generate 'keep' labels of 'length'.
			// 'current' holds the prefix indices for this level.
			// e.g. if L1 kept 2 (A, B), current is [2] (C).
			// We expand current.

			// We need to generate 'keep' labels starting from 'current'.
			// We treat 'current' as a number in base-N.
			// We increment it 'keep' times.

			// Ensure current has correct length
			for len(current) < length {
				current = append(current, 0)
			}

			for range keep {
				// Build string from current indices
				var stringBuilder strings.Builder
				stringBuilder.Grow(length)

				for _, index := range current {
					stringBuilder.WriteRune(chars[index])
				}

				labels = append(labels, stringBuilder.String())

				// Increment current
				// Go from right to left
				for pos := len(current) - 1; pos >= 0; pos-- {
					current[pos]++
					if current[pos] < numChars {
						break
					}
					// Carry over
					current[pos] = 0
					// If we overflow the first digit, it means we are done with this block?
					// But we loop 'keep' times, so we shouldn't overflow invalidly.
				}
			}
		}
	}

	return labels
}
