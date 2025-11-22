package hint

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/y3owk1n/neru/internal/domain/element"
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
		return nil, fmt.Errorf(
			"characters must have at least 2 characters, got %d",
			len(characters),
		)
	}

	// Build uppercase mapping
	uppercaseRuneMap := make(map[rune]rune)
	var uppercaseBuilder strings.Builder

	for _, r := range characters {
		upper := unicode.ToUpper(r)
		uppercaseRuneMap[r] = upper
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
) ([]*Hint, error) {
	if len(elements) == 0 {
		return nil, nil
	}

	if len(elements) > g.maxHints {
		return nil, fmt.Errorf(
			"too many elements: %d exceeds maximum %d",
			len(elements),
			g.maxHints,
		)
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Sort elements by position (top-to-bottom, left-to-right)
	sorted := make([]*element.Element, len(elements))
	copy(sorted, elements)
	sort.Slice(sorted, func(i, j int) bool {
		bi, bj := sorted[i].Bounds(), sorted[j].Bounds()
		// Compare Y first (top to bottom)
		if bi.Min.Y != bj.Min.Y {
			return bi.Min.Y < bj.Min.Y
		}
		// Then X (left to right)
		return bi.Min.X < bj.Min.X
	})

	// Generate labels
	labels := g.generateLabels(len(sorted))

	// Create hints
	hints := make([]*Hint, len(sorted))
	for i, elem := range sorted {
		// Use element center as hint position
		position := elem.Center()

		hint, err := NewHint(labels[i], elem, position)
		if err != nil {
			return nil, fmt.Errorf("failed to create hint %d: %w", i, err)
		}

		hints[i] = hint
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

// generateLabels generates alphabet-based hint labels using a prefix-avoidance strategy.
// Returns uppercase labels.
func (g *AlphabetGenerator) generateLabels(count int) []string {
	if count == 0 {
		return nil
	}

	chars := []rune(g.uppercaseChars)

	// Start with single characters
	pool := make([]string, len(chars))
	for i, c := range chars {
		pool[i] = string(c)
	}

	// Expand until we have enough labels
	for len(pool) < count {
		// Find the last element with minimal length to expand
		// This ensures we exhaust shorter labels before longer ones,
		// and by picking the last one, we preserve the "best" keys (start of alphabet)
		// for as long as possible.
		victimIdx := -1
		minLen := 1000 // Arbitrary large number

		for i := len(pool) - 1; i >= 0; i-- {
			if len(pool[i]) < minLen {
				minLen = len(pool[i])
				victimIdx = i
			}
		}

		if victimIdx == -1 {
			break // Should not happen
		}

		// Expand the victim
		victim := pool[victimIdx]

		// Remove victim
		pool = append(pool[:victimIdx], pool[victimIdx+1:]...)

		// Add expansions
		for _, c := range chars {
			pool = append(pool, victim+string(c))
		}
	}

	// Sort pool by length then alphabetically to ensure deterministic assignment
	// and that shortest hints are assigned to first elements
	sort.Slice(pool, func(i, j int) bool {
		if len(pool[i]) != len(pool[j]) {
			return len(pool[i]) < len(pool[j])
		}
		return pool[i] < pool[j]
	})

	return pool[:count]
}

// UpdateCharacters updates the character set and recalculates max hints.
func (g *AlphabetGenerator) UpdateCharacters(characters string) error {
	if len(characters) < 2 {
		return fmt.Errorf("characters must have at least 2 characters, got %d", len(characters))
	}

	// Build uppercase mapping
	uppercaseRuneMap := make(map[rune]rune)
	var uppercaseBuilder strings.Builder

	for _, r := range characters {
		upper := unicode.ToUpper(r)
		uppercaseRuneMap[r] = upper
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
