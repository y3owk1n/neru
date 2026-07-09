package hint

import (
	"image"
	"reflect"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/element"
)

func mkHint(t *testing.T, label, stableID string) *Interface {
	t.Helper()

	el, err := element.NewElement(
		element.ID(stableID+"#ptr"),
		image.Rect(0, 0, 10, 10),
		element.Role("AXButton"),
		element.WithStableID(stableID),
	)
	if err != nil {
		t.Fatalf("NewElement: %v", err)
	}

	h, err := NewHint(label, el, el.Center())
	if err != nil {
		t.Fatalf("NewHint: %v", err)
	}

	return h
}

func labelMultiset(hints []*Interface) map[string]int {
	out := map[string]int{}
	for _, h := range hints {
		out[h.Label()]++
	}

	return out
}

func labelForStableID(hints []*Interface, sid string) string {
	for _, h := range hints {
		if h.Element().StableID() == sid {
			return h.Label()
		}
	}

	return ""
}

// The core guarantee: the output uses exactly the same label set as the input,
// so the generator's prefix-free property is preserved (labels are only permuted).
func TestReuseLabelsPreservesLabelSetAndCarriesPersistent(t *testing.T) {
	newHints := []*Interface{
		mkHint(t, "a", "x1"),
		mkHint(t, "s", "x2"),
		mkHint(t, "d", "x3"),
	}
	prev := map[string]string{"x2": "a", "x3": "s"}

	out := ReuseLabels(newHints, prev)

	if !reflect.DeepEqual(labelMultiset(out), labelMultiset(newHints)) {
		t.Fatalf("label set changed: got %v want %v", labelMultiset(out), labelMultiset(newHints))
	}

	if got := labelForStableID(out, "x2"); got != "a" {
		t.Fatalf("x2 should keep its previous label a, got %q", got)
	}

	if got := labelForStableID(out, "x3"); got != "s" {
		t.Fatalf("x3 should keep its previous label s, got %q", got)
	}

	if got := labelForStableID(out, "x1"); got != "d" {
		t.Fatalf("x1 should get the remaining label d, got %q", got)
	}
}

// When the set crosses a label-length boundary (old label no longer in the new
// set), the element is relabeled but the set stays prefix-free.
func TestReuseLabelsGrowthDropsUnusableOldLabel(t *testing.T) {
	newHints := []*Interface{
		mkHint(t, "aa", "x1"),
		mkHint(t, "ab", "x2"),
		mkHint(t, "ac", "x3"),
	}
	prev := map[string]string{"x1": "s"} // 1-char label not present in the new 2-char set

	out := ReuseLabels(newHints, prev)

	if !reflect.DeepEqual(labelMultiset(out), labelMultiset(newHints)) {
		t.Fatalf("label set changed: got %v want %v", labelMultiset(out), labelMultiset(newHints))
	}

	if got := labelForStableID(out, "x1"); got == "s" || got == "" {
		t.Fatalf("x1 should be relabeled from the new set, got %q", got)
	}
}

func TestReuseLabelsNoPreviousIsNoOp(t *testing.T) {
	newHints := []*Interface{mkHint(t, "a", "x1"), mkHint(t, "s", "x2")}

	out := ReuseLabels(newHints, nil)

	if len(out) != len(newHints) {
		t.Fatalf("length changed: %d vs %d", len(out), len(newHints))
	}

	for i := range newHints {
		if out[i] != newHints[i] {
			t.Fatalf("hint %d changed with no previous labels", i)
		}
	}
}

// A stable-identity collision (two elements sharing an id) must not break the
// bijection: exactly one claims the carried label, the other is relabeled.
func TestReuseLabelsHandlesIdentityCollision(t *testing.T) {
	newHints := []*Interface{
		mkHint(t, "a", "dup"),
		mkHint(t, "s", "dup"),
		mkHint(t, "d", "x3"),
	}
	prev := map[string]string{"dup": "a"}

	out := ReuseLabels(newHints, prev)

	if !reflect.DeepEqual(labelMultiset(out), labelMultiset(newHints)) {
		t.Fatalf("label set changed on collision: got %v want %v",
			labelMultiset(out), labelMultiset(newHints))
	}

	if len(out) != 3 {
		t.Fatalf("expected 3 hints, got %d", len(out))
	}
}

func TestLabelsByStableIDSkipsEmptyAndDedupes(t *testing.T) {
	hints := []*Interface{
		mkHint(t, "a", "x1"),
		mkHint(t, "s", ""),    // no stable identity: skipped
		mkHint(t, "d", "x1"),  // collision: first writer wins
	}

	got := LabelsByStableID(hints)

	if len(got) != 1 {
		t.Fatalf("expected 1 mapping, got %d: %v", len(got), got)
	}

	if got["x1"] != "a" {
		t.Fatalf("first writer should win: got %q want a", got["x1"])
	}
}
