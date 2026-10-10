package cli

import (
	"slices"
	"testing"

	"github.com/spf13/pflag"
)

// TestQueryHints_OffersOnlyTheCollectionFlags pins that `neru query hints`
// takes the hints flags that decide which elements are collected and no
// others. A query enters nothing, so a flag describing an activation, such as
// --action, is refused as unknown rather than dropped.
func TestQueryHints_OffersOnlyTheCollectionFlags(t *testing.T) {
	t.Parallel()

	var offered []string

	queryHintsCmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		offered = append(offered, flag.Name)
	})

	want := []string{"capture-scope", "json", "role", "split-word", "strategy", "text"}

	slices.Sort(offered)

	if !slices.Equal(offered, want) {
		t.Errorf("query hints offers %v, want %v", offered, want)
	}
}
