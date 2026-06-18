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

	// normalCountsCapacity is the initial capacity of the per-tier label
	// count slice used by the normal (prefix-avoidance) algorithm. The slice
	// is grown as we discover more tiers, so the cap is just an allocation
	// hint for the common 3-tier case.
	normalCountsCapacity = 5
)

// LabelDirection determines how multi-character hint labels are enumerated
// once the single-character pool is exhausted.
type LabelDirection uint8

const (
	// LabelDirectionReverse spreads labels across the alphabet by varying the
	// first character (e.g. "AAA", "BAA", "CAA", ...). Labels within the same
	// length tier are interleaved so prefix clusters do not collapse into a
	// single section of the screen.
	LabelDirectionReverse LabelDirection = iota

	// LabelDirectionNormal uses a prefix-avoidance greedy algorithm that
	// prefers shorter labels (e.g. "AAA", "AAB", "AAC", ...). Shorter labels
	// are kept at lower tiers when possible, but same-prefix labels cluster
	// near each other which can hide the label hint key when many elements
	// share a prefix. This is the default.
	LabelDirectionNormal
)

// String returns the canonical config representation of the label direction.
func (d LabelDirection) String() string {
	switch d {
	case LabelDirectionReverse:
		return "reverse"
	case LabelDirectionNormal:
		return "normal"
	default:
		return "normal"
	}
}

// LabelDirectionFromString parses a config-style label direction string.
// Unknown values resolve to LabelDirectionNormal so we never refuse to
// generate hints for a user with a typo in their config.
func LabelDirectionFromString(s string) LabelDirection {
	switch s {
	case "reverse":
		return LabelDirectionReverse
	default:
		return LabelDirectionNormal
	}
}

// Opposite returns the other label direction. It is safe to call on any
// direction value (including unknown ones), where it falls back to
// LabelDirectionReverse (the opposite of the user-facing default
// LabelDirectionNormal).
func (d LabelDirection) Opposite() LabelDirection {
	switch d {
	case LabelDirectionReverse:
		return LabelDirectionNormal
	case LabelDirectionNormal:
		return LabelDirectionReverse
	default:
		return LabelDirectionReverse
	}
}

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
//
// Two label directions are supported:
//
//   - LabelDirectionReverse emits fixed-length base-N labels so labels within
//     a tier are interleaved (e.g. "AAA", "BAA", "CAA").
//   - LabelDirectionNormal (default) uses a prefix-avoidance greedy
//     algorithm so shorter labels are preferred (e.g. "AAA", "AAB", "AAC").
type AlphabetGenerator struct {
	characters       string
	uppercaseChars   string
	maxHints         int
	uppercaseRuneMap map[rune]rune
	labelDirection   LabelDirection
}

// NewAlphabetGenerator creates a new alphabet-based hint generator.
//
// The label direction controls how multi-character labels are enumerated:
// see LabelDirection for the trade-offs.
func NewAlphabetGenerator(
	characters string,
	labelDirection LabelDirection,
) (*AlphabetGenerator, error) {
	if len(characters) < MinCharactersLength {
		return nil, derrors.Newf(
			derrors.CodeInvalidInput,
			"characters must have at least 2 characters, got %d",
			len(characters),
		)
	}

	// Build uppercase mapping and deduplicate characters
	uppercaseRuneMap := make(map[rune]rune)

	var (
		uppercaseBuilder strings.Builder
		seen             = make(map[rune]struct{}, len(characters))
	)

	for _, rune := range characters {
		upper := unicode.ToUpper(rune)
		uppercaseRuneMap[rune] = upper

		if _, ok := seen[upper]; ok {
			continue
		}

		seen[upper] = struct{}{}
		uppercaseBuilder.WriteRune(upper)
	}

	uppercaseChars := uppercaseBuilder.String()
	charCount := len(uppercaseChars)

	if charCount < MinCharactersLength {
		return nil, derrors.Newf(
			derrors.CodeInvalidInput,
			"characters must have at least %d unique characters after deduplication, got %d",
			MinCharactersLength,
			charCount,
		)
	}

	// Calculate max hints: up to 3 chars
	// Max capacity for fixed-length 3-char base-N encoding is N^3
	n := charCount
	maxHints := n * n * n

	// Pre-cache single character strings
	for _, r := range uppercaseChars {
		if _, ok := singleCharCache.Load(r); !ok {
			singleCharCache.Store(r, string(r))
		}
	}

	return &AlphabetGenerator{
		characters:       uppercaseChars,
		uppercaseChars:   uppercaseChars,
		maxHints:         maxHints,
		uppercaseRuneMap: uppercaseRuneMap,
		labelDirection:   labelDirection,
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

// LabelsForTesting exposes the internal label-generation algorithm so tests
// can assert on the exact ordering produced by each LabelDirection. It must
// not be called from production code — use Generate instead. The slice is
// freshly allocated by computeLabels and is safe to retain.
func (g *AlphabetGenerator) LabelsForTesting(count int) []string {
	return g.generateLabels(count)
}

// LabelDirection returns the label generation direction.
func (g *AlphabetGenerator) LabelDirection() LabelDirection {
	return g.labelDirection
}

// UpdateCharacters updates the character set and recalculates max hints.
// The label direction is preserved.
func (g *AlphabetGenerator) UpdateCharacters(characters string) error {
	return g.Update(characters, g.labelDirection)
}

// UpdateLabelDirection swaps the label generation direction. Character set
// and max hints are preserved. Cached labels are keyed by direction so a
// direction change simply misses the cache and recomputes lazily.
func (g *AlphabetGenerator) UpdateLabelDirection(direction LabelDirection) {
	g.labelDirection = direction
}

// Update replaces both the character set and label direction in a single
// call. Callers that only need to change one field should prefer
// UpdateCharacters or UpdateLabelDirection so the intent is explicit.
func (g *AlphabetGenerator) Update(characters string, direction LabelDirection) error {
	if len(characters) < MinCharactersLength {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"characters must have at least 2 characters, got %d",
			len(characters),
		)
	}

	// Build uppercase mapping and deduplicate characters
	uppercaseRuneMap := make(map[rune]rune)

	var (
		uppercaseBuilder strings.Builder
		seen             = make(map[rune]struct{}, len(characters))
	)

	for _, rune := range characters {
		upper := unicode.ToUpper(rune)
		uppercaseRuneMap[rune] = upper

		if _, ok := seen[upper]; ok {
			continue
		}

		seen[upper] = struct{}{}
		uppercaseBuilder.WriteRune(upper)
	}

	uppercaseChars := uppercaseBuilder.String()
	charCount := len(uppercaseChars)

	if charCount < MinCharactersLength {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"characters must have at least %d unique characters after deduplication, got %d",
			MinCharactersLength,
			charCount,
		)
	}

	// Max capacity for fixed-length 3-char base-N encoding is N^3
	n := charCount
	maxHints := n * n * n

	g.characters = uppercaseChars
	g.uppercaseChars = uppercaseChars
	g.maxHints = maxHints
	g.uppercaseRuneMap = uppercaseRuneMap
	g.labelDirection = direction

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
// The cache key includes the label direction so the two strategies never collide.
func (g *AlphabetGenerator) generateLabels(count int) []string {
	if count == 0 {
		return nil
	}

	// Check bounded LRU cache first (key: "chars:direction:count")
	cacheKey := g.uppercaseChars + ":" + g.labelDirection.String() + ":" + strconv.Itoa(count)

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
// It dispatches to the reverse or normal algorithm based on the configured
// label direction.
func (g *AlphabetGenerator) computeLabels(count int) []string {
	chars := []rune(g.uppercaseChars)
	numChars := len(chars)
	labels := make([]string, 0, count)

	if g.labelDirection == LabelDirectionReverse {
		return g.computeLabelsReverse(count, chars, numChars, labels)
	}

	return g.computeLabelsNormal(count, chars, numChars, labels)
}

// computeLabelsReverse emits fixed-length base-N labels so labels within a
// tier are interleaved. For counts up to the alphabet size it returns single
// characters; for larger counts it emits uniformly 2-char or 3-char labels
// depending on the bucket. Labels look like "AAA", "BAA", "CAA", ...
func (g *AlphabetGenerator) computeLabelsReverse(
	count int,
	chars []rune,
	numChars int,
	labels []string,
) []string {
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

// computeLabelsNormal uses a prefix-avoidance greedy algorithm. It keeps as
// many short labels as possible at the current level, only expanding prefixes
// when the remaining target would otherwise not fit. Labels look like "AAA",
// "AAB", "AAC", ... and same-prefix labels cluster near each other.
func (g *AlphabetGenerator) computeLabelsNormal(
	count int,
	chars []rune,
	numChars int,
	labels []string,
) []string {
	// Calculate distribution of labels across different lengths (levels).
	// counts[i] stores the number of labels of length i+1. We use a greedy
	// algorithm to minimize the average label length while ensuring the
	// prefix-free property.
	counts := make([]int, 0, normalCountsCapacity)

	remainingTarget := count
	availableSlots := numChars // slots available at current level (length 1)

	for remainingTarget > 0 {
		// Capacity if all current slots are expanded to the next level.
		nextLevelCapacity := availableSlots * numChars

		var keep int

		switch {
		case availableSlots >= remainingTarget:
			// Current level can satisfy the remaining target on its own.
			keep = remainingTarget
		case nextLevelCapacity < remainingTarget:
			// We must expand every slot to reach the target — keep nothing.
			keep = 0
		default:
			// Keep as many as possible at this level while ensuring the next
			// level can still cover the remainder.
			//
			// Formula derived from: availableSlots*N - keep*(N-1) >= remainingTarget
			keep = (availableSlots*numChars - remainingTarget) / (numChars - 1)
		}

		counts = append(counts, keep)
		remainingTarget -= keep

		// Remaining slots expanded by the branching factor feed the next level.
		availableSlots = (availableSlots - keep) * numChars

		if availableSlots == 0 && remainingTarget > 0 {
			// Should not happen if the MaxHints check passed at construction.
			break
		}
	}

	var current []int

	for level, keep := range counts {
		length := level + 1

		if length == 1 {
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

			// Next level starts immediately after the kept labels.
			current = []int{keep}
		} else {
			// Expand prefixes: `current` is the running base-N cursor.
			for len(current) < length {
				current = append(current, 0)
			}

			for range keep {
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

				// Increment cursor like adding 1 in base-N.
				for pos := len(current) - 1; pos >= 0; pos-- {
					current[pos]++
					if current[pos] < numChars {
						break
					}

					current[pos] = 0
				}
			}
		}
	}

	return labels
}
