package config

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// TestFiniteFloats_ReachesEveryShape pins that the walk sees a float behind
// a pointer, inside a slice element and inside a map entry, and reports the
// toml path of each, so the schema sweep's coverage of those shapes rests on
// the walker and not on which shapes the schema happens to use today.
func TestFiniteFloats_ReachesEveryShape(t *testing.T) {
	type leaf struct {
		Ratio float64 `toml:"ratio"`
	}

	type fixture struct {
		Direct  float64            `toml:"direct"`
		Ptr     *float64           `toml:"ptr"`
		List    []leaf             `toml:"list"`
		Table   map[string]leaf    `toml:"table"`
		Rates   map[string]float64 `toml:"rates"`
		Pair    [2]float64         `toml:"pair"`
		Skipped map[string]leaf    `toml:"-"`
		hidden  float64            //nolint:unused // proves unexported fields are skipped
		Nested  struct{ L leaf }   `toml:"nested"`
	}

	nan := math.NaN()

	tests := []struct {
		name string
		cfg  fixture
		want string
	}{
		{name: "direct", cfg: fixture{Direct: nan}, want: "direct"},
		{name: "pointer", cfg: fixture{Ptr: &nan}, want: "ptr"},
		{
			name: "slice element",
			cfg:  fixture{List: []leaf{{Ratio: math.Inf(1)}}},
			want: "list.ratio",
		},
		{
			name: "map entry",
			cfg:  fixture{Table: map[string]leaf{"k": {Ratio: math.Inf(-1)}}},
			want: "table.k.ratio",
		},
		{
			name: "untagged table is skipped",
			cfg:  fixture{Skipped: map[string]leaf{"k": {Ratio: nan}}},
			want: "",
		},
		{name: "finite everywhere", cfg: fixture{Ptr: new(float64), List: []leaf{{}}}, want: ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := finiteFloats(reflect.ValueOf(testCase.cfg), "")
			if testCase.want == "" {
				if err != nil {
					t.Fatalf("finite fixture rejected: %v", err)
				}

				return
			}

			if err == nil {
				t.Fatalf("non-finite %s accepted", testCase.want)
			}

			if got := err.Error(); !strings.Contains(got, testCase.want) {
				t.Errorf("error %q does not name %s", got, testCase.want)
			}
		})
	}
}
