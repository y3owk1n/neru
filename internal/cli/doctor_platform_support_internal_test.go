package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

// hideCursorAction is the darwin-only action the fixtures below are built
// around, named once so the fixtures cannot disagree about its spelling.
const hideCursorAction = "hide_cursor"

// errRefusedFixture stands in for whatever refused a configuration; the row
// under test only asks whether one did.
var errRefusedFixture = derrors.New(derrors.CodeInvalidConfig, "bad file")

// TestPrintPlatformSupportCheck covers the `neru doctor` half of ADR 0013: the
// row that tells a person which of the words they wrote mean nothing on the
// platform they are on, and why.
//
// The findings are handed in rather than loaded, so the Linux row can be read
// from any machine — on macOS nothing in the schema is inert, and a test that
// loaded a file would assert the empty case three times.
func TestPrintPlatformSupportCheck(t *testing.T) {
	inert := parity.Declaration{
		{
			Kind:      parity.KindOption,
			Name:      "smooth_scroll.enabled",
			Platforms: parity.Platforms{parity.Darwin},
			Note:      "only the darwin scroll animator reads these",
		},
		{
			Kind:      parity.KindAction,
			Name:      hideCursorAction,
			Platforms: parity.Platforms{parity.Darwin},
			Note:      "a Wayland client may not hide another client's cursor",
		},
	}

	tests := []struct {
		name     string
		result   *config.LoadResult
		contains []string
		absent   []string
	}{
		{
			name:     "a configuration that works here reports healthy",
			result:   &config.LoadResult{},
			contains: []string{"✅", platformSupportRow},
			absent:   []string{"⚠️"},
		},
		{
			name:   "inert words are listed with the reason",
			result: &config.LoadResult{Inert: inert},
			contains: []string{
				"⚠️",
				platformSupportRow,
				"2 settings do nothing",
				"smooth_scroll.enabled",
				"only the darwin scroll animator reads these",
				hideCursorAction,
				"the daemon runs",
			},
		},
		{
			name:   "one finding is worded as one",
			result: &config.LoadResult{Inert: inert[:1]},
			contains: []string{
				"1 setting does nothing",
			},
		},
		{
			name:     "a refused configuration says so instead of guessing",
			result:   &config.LoadResult{ValidationError: errRefusedFixture},
			contains: []string{"neru config validate"},
			absent:   []string{"✅"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer

			cmd := &cobra.Command{}
			cmd.SetOut(&out)

			printPlatformSupportCheck(cmd, test.result)

			if _, known := parity.Current(); !known {
				t.Skipf("no platform column is declared for this build")
			}

			for _, want := range test.contains {
				if !strings.Contains(out.String(), want) {
					t.Errorf("output does not mention %q:\n%s", want, out.String())
				}
			}

			for _, unwanted := range test.absent {
				if strings.Contains(out.String(), unwanted) {
					t.Errorf("output mentions %q, which it should not:\n%s",
						unwanted, out.String())
				}
			}
		})
	}
}
