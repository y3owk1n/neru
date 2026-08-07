package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// TestResolveSublayerKeys pins the answer to "which keys does a subgrid use
// when sublayer_keys is unset?". Four consumers answered it before this
// resolution existed — two overlay backends with a hardcoded alphabet of their
// own — so the overlay could draw one set while the mode layer accepted
// another. These cases are the rule itself.
//
// Unset settles to the characters the grid is *labeled* with, which is the
// resolved form rather than the configured one: the same answer
// ResolveGridLabels gives, floor and all.
func TestResolveSublayerKeys(t *testing.T) {
	testCases := []struct {
		name           string
		sublayerKeys   string
		gridCharacters string
		hintCharacters string
		want           string
	}{
		{
			name:           "configured keys are kept",
			sublayerKeys:   "uiop",
			gridCharacters: gridLabelGridChars,
			hintCharacters: gridLabelHintChars,
			want:           "uiop",
		},
		{
			name:           "unset keys are the characters the grid is labeled with",
			gridCharacters: gridLabelGridChars,
			hintCharacters: gridLabelHintChars,
			want:           gridLabelsFromGridChars,
		},
		{
			name:           "blank keys are not a key set",
			sublayerKeys:   gridLabelBlank,
			gridCharacters: gridLabelGridChars,
			hintCharacters: gridLabelHintChars,
			want:           gridLabelsFromGridChars,
		},
		{
			name:           "unset keys follow the grid through the hint-characters fallback",
			gridCharacters: "",
			hintCharacters: gridLabelHintChars,
			want:           gridLabelsFromHintChars,
		},
		{
			// Nothing left to infer from. The grid still labels itself a-z, so
			// resolving to the blank set here would draw a subgrid with no keys
			// on it under a grid that has them.
			name:           "no characters anywhere still leaves a subgrid to navigate",
			gridCharacters: "",
			hintCharacters: "",
			want:           gridLabelsFloor,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Grid.SublayerKeys = testCase.sublayerKeys
			cfg.Grid.Characters = testCase.gridCharacters
			cfg.Hints.HintCharacters = testCase.hintCharacters

			cfg.ResolveSublayerKeys()

			if cfg.Grid.SublayerKeys != testCase.want {
				t.Errorf(
					"Grid.SublayerKeys = %q, want %q",
					cfg.Grid.SublayerKeys, testCase.want,
				)
			}
		})
	}
}

// TestResolveSublayerKeys_SettlesToItself covers the second run every caller of
// the derivation chain makes: settled keys are indistinguishable from typed
// ones, so re-running has to leave them alone rather than resolve them again
// from characters that may have moved on.
func TestResolveSublayerKeys_SettlesToItself(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Grid.SublayerKeys = ""
	cfg.Grid.Characters = "asdfg"

	cfg.ResolveSublayerKeys()
	settled := cfg.Grid.SublayerKeys

	cfg.Grid.Characters = "qwert"
	cfg.ResolveSublayerKeys()

	if cfg.Grid.SublayerKeys != settled {
		t.Errorf("Grid.SublayerKeys = %q on a second run, want %q", cfg.Grid.SublayerKeys, settled)
	}
}
