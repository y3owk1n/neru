package grid

import "strings"

// DistinctKeys is the key set a written one means: upper-cased, in the order it
// was written, with every character that appears again dropped.
//
// It exists because a grid resolves a typed key to a position by finding it, and
// finding it stops at the first one. A character written twice therefore named
// two places and reached one: nine subgrid labels over eight reachable points,
// or — through the coordinate alphabet, where a label is several characters —
// sixteen cells all called AAAA. The second one of each pair was drawn, was
// typed, and did nothing.
//
// Case-folded, because that is the comparison every reader of these sets makes
// (the manager upper-cases a key before matching it), so "aA" is one key in
// exactly the way "aa" is. It is the rule hints.hint_characters has always had —
// validateHintCharacters refuses a repeat outright — arrived at from the other
// side: the grid drops the repeat instead of refusing the configuration, and the
// config warns about it (config.ValidateGrid), because refusing a grid costs the
// whole file (ADR 0002).
//
// Exported so the config can ask what a written set comes to — how short the
// grid's alphabet ends up, which is what its floor and its warning are about —
// rather than working it out from a second copy of this rule.
func DistinctKeys(keys string) []rune {
	upper := strings.ToUpper(keys)
	distinct := make([]rune, 0, len(upper))

	seen := make(map[rune]struct{}, len(upper))

	for _, key := range upper {
		_, repeat := seen[key]
		if repeat {
			continue
		}

		seen[key] = struct{}{}
		distinct = append(distinct, key)
	}

	return distinct
}
