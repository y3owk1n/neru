// Package hints provides hint generation and management functionality for the Neru application.
package hints

import (
	"image"
	"sort"
	"strings"

	"github.com/y3owk1n/neru/internal/accessibility"
	"github.com/y3owk1n/neru/internal/logger"
	"go.uber.org/zap"
)

// Hint represents a hint label for a UI element.
type Hint struct {
	Label         string
	Element       *accessibility.TreeNode
	Position      image.Point
	Size          image.Point
	MatchedPrefix string // Characters that have been typed
}

// Generator generates hints for UI elements.
type Generator struct {
	characters string
	maxHints   int
}

// NewGenerator creates a new hint generator.
func NewGenerator(characters string) *Generator {
	// Ensure we have at least some characters
	if characters == "" {
		// Use home row characters by default
		characters = "asdfghjkl" // fallback to default
	}

	charCount := len(characters)
	maxHints := charCount * charCount * charCount

	logger.Debug(
		"Considered characters",
		zap.String("characters", characters),
		zap.Int("charCount", charCount),
	)
	logger.Debug("Setting maxHints", zap.Int("maxHints", maxHints))

	return &Generator{
		characters: characters,
		maxHints:   maxHints,
	}
}

// Generate generates hints for the given elements.
func (g *Generator) Generate(elements []*accessibility.TreeNode) ([]*Hint, error) {
	if len(elements) == 0 {
		return []*Hint{}, nil
	}

	// Sort elements by position (top-to-bottom, left-to-right)
	sortedElements := make([]*accessibility.TreeNode, len(elements))
	copy(sortedElements, elements)
	sort.Slice(sortedElements, func(i, j int) bool {
		posI := sortedElements[i].Info.Position
		posJ := sortedElements[j].Info.Position

		// Sort by Y first (top to bottom)
		if posI.Y != posJ.Y {
			return posI.Y < posJ.Y
		}
		// Then by X (left to right)
		return posI.X < posJ.X
	})

	// Limit to max hints
	if g.maxHints > 0 && len(sortedElements) > g.maxHints {
		sortedElements = sortedElements[:g.maxHints]
	}

	// Generate labels (alphabet-only)
	labels := g.generateAlphabetLabels(len(sortedElements))

	// Generate hints
	hints := make([]*Hint, len(sortedElements))
	for elementIndex, element := range sortedElements {
		// Position hint at the center of the element (like Vimac does)
		centerX := element.Info.Position.X + (element.Info.Size.X / 2)
		centerY := element.Info.Position.Y + (element.Info.Size.Y / 2)

		hints[elementIndex] = &Hint{
			Label:    strings.ToUpper(labels[elementIndex]), // Convert to uppercase
			Element:  element,
			Position: image.Point{X: centerX, Y: centerY},
			Size:     element.Info.Size,
		}
	}

	return hints, nil
}

// generateAlphabetLabels generates alphabet-based hint labels.
// Uses a strategy that avoids prefixes (no "a" if "aa" exists).
func (g *Generator) generateAlphabetLabels(count int) []string {
	if count <= 0 {
		return []string{}
	}

	labels := make([]string, 0, count)
	chars := []rune(g.characters)
	numChars := len(chars)

	var length int
	switch {
	case count <= numChars:
		length = 1
	case count <= numChars*numChars:
		length = 2
	default:
		length = 3
	}

	// Generate labels of the determined length
	switch length {
	case 1:
		// Single character labels
		for i := range chars[:count] {
			labels = append(labels, string(chars[i]))
		}
	case 2:
		// All 2-char combinations
		for i := 0; i < numChars && len(labels) < count; i++ {
			for j := 0; j < numChars && len(labels) < count; j++ {
				labels = append(labels, string(chars[i])+string(chars[j]))
			}
		}
	case 3:
		// All 3-char combinations
		for i := 0; i < numChars && len(labels) < count; i++ {
			for j := 0; j < numChars && len(labels) < count; j++ {
				for k := 0; k < numChars && len(labels) < count; k++ {
					labels = append(labels, string(chars[i])+string(chars[j])+string(chars[k]))
				}
			}
		}
	}

	return labels[:count]
}

// GetBounds returns the bounding rectangle for a hint.
func (h *Hint) GetBounds() image.Rectangle {
	return image.Rectangle{
		Min: h.Position,
		Max: image.Point{
			X: h.Position.X + h.Size.X,
			Y: h.Position.Y + h.Size.Y,
		},
	}
}

// IsVisible checks if the hint is visible on screen.
func (h *Hint) IsVisible(screenBounds image.Rectangle) bool {
	bounds := h.GetBounds()
	return bounds.Overlaps(screenBounds)
}

// HintCollection manages a collection of hints.
type HintCollection struct {
	hints   []*Hint
	active  bool
	byLabel map[string]*Hint
	prefix1 map[byte][]*Hint
	prefix2 map[string][]*Hint
}

// NewHintCollection creates a new hint collection.
func NewHintCollection(hints []*Hint) *HintCollection {
	hintCollection := &HintCollection{
		hints:   hints,
		active:  true,
		byLabel: make(map[string]*Hint, len(hints)),
		prefix1: make(map[byte][]*Hint),
		prefix2: make(map[string][]*Hint),
	}
	for _, hint := range hints {
		label := strings.ToUpper(hint.Label)
		hintCollection.byLabel[label] = hint
		if len(label) >= 1 {
			firstByte := label[0]
			hintCollection.prefix1[firstByte] = append(hintCollection.prefix1[firstByte], hint)
		}
		if len(label) >= 2 {
			prefix := label[:2]
			hintCollection.prefix2[prefix] = append(hintCollection.prefix2[prefix], hint)
		}
	}
	return hintCollection
}

// GetHints returns all hints.
func (hc *HintCollection) GetHints() []*Hint {
	return hc.hints
}

// FindByLabel finds a hint by label.
func (hc *HintCollection) FindByLabel(label string) *Hint {
	return hc.byLabel[strings.ToUpper(label)]
}

// FilterByPrefix filters hints by prefix.
func (hc *HintCollection) FilterByPrefix(prefix string) []*Hint {
	if prefix == "" {
		return hc.hints
	}
	upperPrefix := strings.ToUpper(prefix)
	if len(upperPrefix) == 1 {
		firstByte := upperPrefix[0]
		bucket := hc.prefix1[firstByte]
		if len(bucket) == 0 {
			return []*Hint{}
		}
		out := make([]*Hint, 0, len(bucket))
		out = append(out, bucket...)
		return out
	}
	if len(upperPrefix) >= 2 {
		if bucket, ok := hc.prefix2[upperPrefix[:2]]; ok {
			out := make([]*Hint, 0, len(bucket))
			for _, hint := range bucket {
				if strings.HasPrefix(hint.Label, upperPrefix) {
					out = append(out, hint)
				}
			}
			return out
		}
		return []*Hint{}
	}
	return []*Hint{}
}

// IsActive returns whether the collection is active.
func (hc *HintCollection) IsActive() bool {
	return hc.active
}

// Deactivate deactivates the collection.
func (hc *HintCollection) Deactivate() {
	hc.active = false
}

// Count returns the number of hints.
func (hc *HintCollection) Count() int {
	return len(hc.hints)
}
