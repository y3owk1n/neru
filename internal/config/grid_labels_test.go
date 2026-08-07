package config_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// What a grid is built from: gridLabelHintChars is the hint alphabet it falls
// back to when grid.characters is blank, gridLabelGridChars the characters it
// is built from when it is not, and gridLabelBlank a value that was written and
// says nothing.
const (
	gridLabelHintChars = "qwerty"
	gridLabelGridChars = "asdfghjkl"
	gridLabelBlank     = "   "
)

// What each of those settles to: the labels a grid built from it carries, and
// the floor for a set too short to label with, read from the constant rather
// than spelled out so a copy here cannot pass while the two disagree.
const (
	gridLabelsFromGridChars = "ASDFGHJKL"
	gridLabelsFromHintChars = "QWERTY"
)

var gridLabelsFloor = strings.ToUpper(domainGrid.DefaultCharacters)

// TestResolveGridLabels pins the answer to "what is grid.row_labels when
// unset?". Before this resolution existed the answer lived in a consumer, so
// every other reader had to reimplement it; these cases are the rule itself.
func TestResolveGridLabels(t *testing.T) {
	testCases := []struct {
		name           string
		gridCharacters string
		hintCharacters string
		rowLabels      string
		colLabels      string
		wantRowLabels  string
		wantColLabels  string
	}{
		{
			name:           "unset labels are inferred from the grid characters",
			gridCharacters: gridLabelGridChars,
			hintCharacters: gridLabelHintChars,
			wantRowLabels:  gridLabelsFromGridChars,
			wantColLabels:  gridLabelsFromGridChars,
		},
		{
			name:           "configured labels are kept, uppercased",
			gridCharacters: gridLabelGridChars,
			hintCharacters: gridLabelHintChars,
			rowLabels:      "abc",
			colLabels:      "def",
			wantRowLabels:  "ABC",
			wantColLabels:  "DEF",
		},
		{
			name:           "one configured label leaves the other inferred",
			gridCharacters: "asdf",
			hintCharacters: gridLabelHintChars,
			rowLabels:      "xy",
			wantRowLabels:  "XY",
			wantColLabels:  "ASDF",
		},
		{
			name:           "blank grid characters infer from the hint characters",
			gridCharacters: gridLabelBlank,
			hintCharacters: gridLabelHintChars,
			wantRowLabels:  gridLabelsFromHintChars,
			wantColLabels:  gridLabelsFromHintChars,
		},
		{
			name:           "a character set too short to label anything infers the default alphabet",
			gridCharacters: "a",
			hintCharacters: gridLabelHintChars,
			wantRowLabels:  gridLabelsFloor,
			wantColLabels:  gridLabelsFloor,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Grid.Characters = testCase.gridCharacters
			cfg.Hints.HintCharacters = testCase.hintCharacters
			cfg.Grid.RowLabels = testCase.rowLabels
			cfg.Grid.ColLabels = testCase.colLabels

			cfg.ResolveGridLabels()

			if cfg.Grid.RowLabels != testCase.wantRowLabels {
				t.Errorf(
					"Grid.RowLabels = %q, want %q",
					cfg.Grid.RowLabels, testCase.wantRowLabels,
				)
			}

			if cfg.Grid.ColLabels != testCase.wantColLabels {
				t.Errorf(
					"Grid.ColLabels = %q, want %q",
					cfg.Grid.ColLabels, testCase.wantColLabels,
				)
			}
		})
	}
}
