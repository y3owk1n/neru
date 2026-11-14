package hints

import (
	"image"
	"testing"
)

func TestHint_TypedPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		label   string
		matched string
		want    string
	}{
		{name: "nothing typed", label: "SD", matched: "", want: ""},
		{name: "first of two", label: "SD", matched: "S", want: "S"},
		{name: "the whole label", label: "SD", matched: "SD", want: "SD"},
		{name: "typed in lower case", label: "SD", matched: "s", want: "S"},
		{name: "spelled as the label spells it", label: "sd", matched: "S", want: "s"},
		{name: "a prefix that does not match", label: "SD", matched: "D", want: ""},
		{name: "longer than the label", label: "A", matched: "AB", want: "A"},
		{name: "past a multi-byte character", label: "ÉA", matched: "é", want: "É"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			hint := NewHint(test.label, image.Point{}, image.Point{}, test.matched)
			if got := hint.TypedPrefix(); got != test.want {
				t.Errorf("TypedPrefix() of %q typed %q = %q, want %q",
					test.label, test.matched, got, test.want)
			}
		})
	}
}
