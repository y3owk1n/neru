package modecmd_test

import (
	"reflect"
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// TestParse_TargetsReadsAQueryAnswer pins --targets to the shape `neru query
// hints --json` prints: the label it reports, and any key a later query
// gains, are ignored rather than refused.
func TestParse_TargetsReadsAQueryAnswer(t *testing.T) {
	t.Parallel()

	answer := `{"hints":[{"label":"A","role":"ax:AXButton","title":"Save","description":"","value":"",` +
		`"x":100,"y":38,"width":44,"height":52,"added_later":true}]}`

	activation, err := modecmd.Parse(domain.ModeHints, []string{"--targets", answer})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	want := []modecmd.Target{
		{X: 100, Y: 38, Width: 44, Height: 52, Role: "ax:AXButton", Title: "Save"},
	}
	if !reflect.DeepEqual(activation.Targets, want) {
		t.Errorf("Targets = %+v, want %+v", activation.Targets, want)
	}
}
