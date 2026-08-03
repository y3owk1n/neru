package accessibility

import (
	"testing"

	"github.com/y3owk1n/neru/internal/ports"
)

// Every text comparison the filter drives is done against a lowercased element
// string, so a filter term that reaches matching with its original casing never
// matches anything the user typed in mixed case.

func TestLowerFilter_LowercasesEveryTextComparison(t *testing.T) {
	lowered := lowerFilter(ports.ElementFilter{
		TitleContains:       "Save As",
		DescriptionContains: "Close Button",
		ValueContains:       "README.md",
		TextContainsList:    []string{"OK", "Cancel"},
	})

	if lowered.TitleContains != "save as" {
		t.Errorf("TitleContains = %q, want it lowercased", lowered.TitleContains)
	}

	if lowered.DescriptionContains != "close button" {
		t.Errorf("DescriptionContains = %q, want it lowercased", lowered.DescriptionContains)
	}

	if lowered.ValueContains != "readme.md" {
		t.Errorf("ValueContains = %q, want it lowercased", lowered.ValueContains)
	}

	want := []string{"ok", "cancel"}
	for index, text := range lowered.TextContainsList {
		if text != want[index] {
			t.Errorf("TextContainsList[%d] = %q, want %q", index, text, want[index])
		}
	}
}

// TestLowerFilter_DoesNotWriteThroughToTheCaller pins that the caller's list is
// left alone. The filter travels by value but its list does not, so lowercasing
// in place would rewrite a slice the caller may still hold.
func TestLowerFilter_DoesNotWriteThroughToTheCaller(t *testing.T) {
	original := []string{"OK"}

	lowerFilter(ports.ElementFilter{TextContainsList: original})

	if original[0] != "OK" {
		t.Errorf("caller's list = %v, want it untouched", original)
	}
}

// TestLowerFilter_KeepsTheNonTextFilters guards against the copy dropping the
// fields it does not touch.
func TestLowerFilter_KeepsTheNonTextFilters(t *testing.T) {
	lowered := lowerFilter(ports.ElementFilter{
		IncludeDock:        true,
		IncludeMenubar:     true,
		SkipWindowElements: true,
	})

	if !lowered.IncludeDock || !lowered.IncludeMenubar || !lowered.SkipWindowElements {
		t.Errorf("filter = %+v, want the non-text fields carried through", lowered)
	}
}
