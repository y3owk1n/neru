package hint_test

import (
	"context"
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
)

// TestNewAlphabetGenerator_CharacterCountBoundary pins the accept/reject edge of
// the character-set length check. Exactly MinCharactersLength characters is the
// smallest usable alphabet — a two-character set still produces a prefix-free
// labeling — so it must be accepted, and one fewer must be rejected. An
// off-by-one here either locks users out of a valid config or admits a
// single-character set that cannot generate distinct labels at all.
func TestNewAlphabetGenerator_CharacterCountBoundary(t *testing.T) {
	tests := []struct {
		name       string
		characters string
		wantErr    bool
	}{
		{"one below the minimum is rejected", "a", true},
		{"empty is rejected", "", true},
		{"exactly the minimum is accepted", "ab", false},
		{"one above the minimum is accepted", "abc", false},
	}

	if hint.MinCharactersLength != 2 {
		t.Fatalf(
			"this test's fixtures assume MinCharactersLength == 2, got %d",
			hint.MinCharactersLength,
		)
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			generator, err := hint.NewAlphabetGenerator(
				testCase.characters,
				hint.LabelDirectionReverse,
			)

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("NewAlphabetGenerator(%q) error = nil, want an error",
						testCase.characters)
				}

				return
			}

			if err != nil {
				t.Fatalf("NewAlphabetGenerator(%q) error = %v, want nil",
					testCase.characters, err)
			}

			// An accepted generator must actually be able to label elements.
			if labels := generator.LabelsForTesting(4); len(labels) != 4 {
				t.Errorf("LabelsForTesting(4) returned %d labels, want 4", len(labels))
			}
		})
	}
}

// TestAlphabetGenerator_UpdateCharacterCountBoundary applies the same boundary
// to the reload path. Update runs on config reload with user-supplied input, so
// it has to reject a too-small set rather than leaving the generator in a state
// that cannot produce distinct labels.
func TestAlphabetGenerator_UpdateCharacterCountBoundary(t *testing.T) {
	generator, err := hint.NewAlphabetGenerator("abcdef", hint.LabelDirectionReverse)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator: %v", err)
	}

	err = generator.UpdateCharacters("a")
	if err == nil {
		t.Error(`UpdateCharacters("a") error = nil, want an error for a one-character set`)
	}

	// A rejected update must not have taken effect.
	if got := generator.Characters(); !strings.EqualFold(got, "abcdef") {
		t.Errorf("Characters() = %q after a rejected update, want the original set", got)
	}

	err = generator.UpdateCharacters("xy")
	if err != nil {
		t.Errorf(`UpdateCharacters("xy") error = %v, want nil for a minimum-size set`, err)
	}

	if got := generator.Characters(); !strings.EqualFold(got, "xy") {
		t.Errorf("Characters() = %q after an accepted update, want %q", got, "xy")
	}
}

// TestAlphabetGenerator_Generate_AssignsLabelsInReadingOrder pins the element
// ordering. Labels are handed out in sorted order, so the sort comparator
// decides which element gets the shortest / earliest label. Users navigate by
// position, so top-to-bottom then left-to-right is the contract; a comparator
// that comparedeither axis alone, or compared X before Y, would scramble it.
func TestAlphabetGenerator_Generate_AssignsLabelsInReadingOrder(t *testing.T) {
	generator, err := hint.NewAlphabetGenerator("abcdefghij", hint.LabelDirectionReverse)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator: %v", err)
	}

	// Deliberately supplied out of order, and laid out so that sorting by X
	// alone, or by Y alone, produces a different sequence than reading order.
	type placed struct {
		id   string
		x, y int
	}

	input := []placed{
		{"row2-right", 300, 200},
		{"row1-right", 300, 100},
		{"row2-left", 100, 200},
		{"row1-left", 100, 100},
		{"row1-middle", 200, 100},
	}

	elements := make([]*element.Element, 0, len(input))

	for _, spec := range input {
		created, elemErr := element.NewElement(
			element.ID(spec.id),
			image.Rect(spec.x, spec.y, spec.x+50, spec.y+20),
			element.RoleButton,
		)
		if elemErr != nil {
			t.Fatalf("NewElement(%s): %v", spec.id, elemErr)
		}

		elements = append(elements, created)
	}

	hints, err := generator.Generate(context.Background(), elements)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(hints) != len(elements) {
		t.Fatalf("Generate returned %d hints, want %d", len(hints), len(elements))
	}

	wantOrder := []string{"row1-left", "row1-middle", "row1-right", "row2-left", "row2-right"}

	gotOrder := make([]string, 0, len(hints))
	for _, generated := range hints {
		gotOrder = append(gotOrder, string(generated.Element().ID()))
	}

	if !slices.Equal(gotOrder, wantOrder) {
		t.Errorf("elements labeled in order %v, want reading order %v", gotOrder, wantOrder)
	}
}

// TestAlphabetGenerator_Generate_LabelsArePrefixFreeAndUnique is the invariant
// the whole level-allocation algorithm exists to preserve: no label may be a
// prefix of another, or typing the shorter one could never be resolved without
// an extra keystroke, and no two elements may share a label.
//
// The counts are swept across the points where the algorithm rolls over to a
// new label length, which is exactly where an off-by-one in the slot arithmetic
// shows up.
func TestAlphabetGenerator_Generate_LabelsArePrefixFreeAndUnique(t *testing.T) {
	for _, characters := range []string{"ab", "abc", "asdfghjkl"} {
		numChars := len(characters)

		// Sweep around each level rollover: numChars, numChars^2, numChars^3.
		counts := map[int]bool{1: true}
		for capacity := numChars; capacity <= numChars*numChars*numChars; capacity *= numChars {
			for _, delta := range []int{-1, 0, 1, 2} {
				if capacity+delta > 0 {
					counts[capacity+delta] = true
				}
			}
		}

		ordered := make([]int, 0, len(counts))
		for count := range counts {
			ordered = append(ordered, count)
		}

		slices.Sort(ordered)

		for _, direction := range []hint.LabelDirection{
			hint.LabelDirectionReverse,
			hint.LabelDirectionNormal,
		} {
			generator, err := hint.NewAlphabetGenerator(characters, direction)
			if err != nil {
				t.Fatalf("NewAlphabetGenerator(%q): %v", characters, err)
			}

			maxHints := generator.MaxHints()

			for _, count := range ordered {
				if count > maxHints {
					continue
				}

				t.Run(labelCaseName(characters, direction, count), func(t *testing.T) {
					labels := generator.LabelsForTesting(count)

					if len(labels) != count {
						t.Fatalf("got %d labels, want %d", len(labels), count)
					}

					assertLabelsUniqueAndPrefixFree(t, labels)
					assertLabelsNoLongerThanNecessary(t, labels, numChars, count)
				})
			}
		}
	}
}

// assertLabelsUniqueAndPrefixFree checks the two properties the hint router
// depends on: labels are distinct, and none is a prefix of another.
func assertLabelsUniqueAndPrefixFree(t *testing.T, labels []string) {
	t.Helper()

	seen := make(map[string]int, len(labels))

	for idx, label := range labels {
		if label == "" {
			t.Errorf("label %d is empty", idx)

			continue
		}

		if prev, dup := seen[label]; dup {
			t.Errorf("labels %d and %d are both %q", prev, idx, label)
		}

		seen[label] = idx
	}

	sorted := slices.Clone(labels)
	slices.Sort(sorted)

	// After sorting, a label can only be a prefix of its immediate successors,
	// so a single adjacent scan is enough.
	for idx := 1; idx < len(sorted); idx++ {
		if strings.HasPrefix(sorted[idx], sorted[idx-1]) {
			t.Errorf("label %q is a prefix of %q; typing the former could never resolve",
				sorted[idx-1], sorted[idx])
		}
	}
}

// assertLabelsNoLongerThanNecessary pins the efficiency the level-allocation
// algorithm exists to deliver: label length is keystrokes, so no label may be
// longer than the shortest fixed length that could address `count` elements
// with `numChars` characters. A greedy step that keeps one slot too few (or too
// many) at some level still yields unique, prefix-free labels — the property
// checked above — but spills into an extra level and costs the user a keystroke
// on every hint. That regression is only visible here.
func assertLabelsNoLongerThanNecessary(t *testing.T, labels []string, numChars, count int) {
	t.Helper()

	// Smallest L with numChars^L >= count.
	minimalLength := 1
	for capacity := numChars; capacity < count; capacity *= numChars {
		minimalLength++
	}

	longest := 0
	longestLabel := ""

	for _, label := range labels {
		if length := len([]rune(label)); length > longest {
			longest = length
			longestLabel = label
		}
	}

	if longest > minimalLength {
		t.Errorf(
			"longest label %q is %d characters; %d elements over a %d-character alphabet need at most %d",
			longestLabel,
			longest,
			count,
			numChars,
			minimalLength,
		)
	}
}

func labelCaseName(characters string, direction hint.LabelDirection, count int) string {
	name := "normal"
	if direction == hint.LabelDirectionReverse {
		name = "reverse"
	}

	return fmt.Sprintf("chars=%s/dir=%s/count=%d", characters, name, count)
}
