package hint

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/y3owk1n/neru/internal/core/domain/element"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

const (
	// MinCharactersLength is the minimum length for characters.
	MinCharactersLength = 2

	// maxLabelCacheEntries caps the global label cache to prevent unbounded growth.
	maxLabelCacheEntries = 64
)

// labelCacheEntry holds a cached label slice with access tracking.
type labelCacheEntry struct {
	labels   []string
	lastUsed time.Time
}

// labelCache is a bounded LRU cache for generated labels.
var (
	labelCacheMu   sync.Mutex
	labelCache     = make(map[string]*labelCacheEntry)
	labelCacheKeys []string // insertion-order tracking for LRU eviction
)

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

	// Pre-cache single character strings
	for _, r := range uppercaseChars {
		if _, ok := singleCharCache.Load(r); !ok {
			singleCharCache.Store(r, string(r))
		}
	}

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

	if len(sorted) > g.maxHints {
		sorted = sorted[:g.maxHints]
	}

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
	for _, r := range g.uppercaseChars {
		if _, ok := singleCharCache.Load(r); !ok {
			singleCharCache.Store(r, string(r))
		}
	}

	return nil
}

// generateLabels generates fixed-length base-N alphabet labels.
// For counts up to the alphabet size, returns single characters.
// For larger counts, progressively generates 2-char, 3-char, etc. labels to satisfy the count.
// Results are cached in a bounded LRU cache for instant reuse on repeated counts.
func (g *AlphabetGenerator) generateLabels(count int) []string {
	if count == 0 {
		return nil
	}

	// Check bounded LRU cache first (key: "chars:count")
	cacheKey := g.uppercaseChars + ":" + strconv.Itoa(count)

	labelCacheMu.Lock()
	if entry, ok := labelCache[cacheKey]; ok {
		entry.lastUsed = time.Now()
		labels := entry.labels
		labelCacheMu.Unlock()

		return labels
	}
	labelCacheMu.Unlock()

	// Generate labels if not cached
	labels := g.computeLabels(count)

	// Store in bounded LRU cache for future use.
	// Double-check under lock: another goroutine may have computed and cached
	// the same key while we were computing (TOCTOU).
	labelCacheMu.Lock()
	if entry, ok := labelCache[cacheKey]; ok {
		// Another goroutine beat us — use its result, update access time.
		entry.lastUsed = time.Now()
		labelCacheMu.Unlock()

		return entry.labels
	}

	if len(labelCache) >= maxLabelCacheEntries {
		// Evict least recently used entry
		oldestIdx := 0

		oldestTime := labelCache[labelCacheKeys[0]].lastUsed
		for i := 1; i < len(labelCacheKeys); i++ {
			t := labelCache[labelCacheKeys[i]].lastUsed
			if t.Before(oldestTime) {
				oldestTime = t
				oldestIdx = i
			}
		}

		oldestKey := labelCacheKeys[oldestIdx]
		delete(labelCache, oldestKey)

		labelCacheKeys[oldestIdx] = labelCacheKeys[len(labelCacheKeys)-1]
		labelCacheKeys = labelCacheKeys[:len(labelCacheKeys)-1]
	}

	labelCache[cacheKey] = &labelCacheEntry{labels: labels, lastUsed: time.Now()}
	labelCacheKeys = append(labelCacheKeys, cacheKey)
	labelCacheMu.Unlock()

	return labels
}

// computeLabels performs the actual label generation (extracted for caching).
func (g *AlphabetGenerator) computeLabels(count int) []string {
	chars := []rune(g.uppercaseChars)
	numChars := len(chars)
	labels := make([]string, 0, count)

	if count <= numChars {
		for i := range count {
			char := chars[i]
			if cached, ok := singleCharCache.Load(char); ok {
				if str, ok := cached.(string); ok {
					labels = append(labels, str)

					continue
				}
			}

			labels = append(labels, string(char))
		}

		return labels
	}

	length := 2
	if count > numChars*numChars {
		length = 3
	}

	for index := range count {
		stringBuilder, ok := stringBuilderPool.Get().(*strings.Builder)
		if !ok {
			stringBuilder = &strings.Builder{}
		}

		stringBuilder.Reset()
		stringBuilder.Grow(length)

		v := index
		for range length {
			digit := v % numChars
			v /= numChars

			stringBuilder.WriteRune(chars[digit])
		}

		labels = append(labels, stringBuilder.String())
		stringBuilderPool.Put(stringBuilder)
	}

	return labels
}
