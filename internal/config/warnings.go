package config

import (
	"fmt"
	"slices"
)

// Warnings collects what a configuration says that will not happen.
//
// A warning is for a setting that is written well enough to load and will not
// do what it reads as — a binding naming a flag its mode has no use for, say.
// Refusing the load over one would be worse than the bug: a refused
// configuration is replaced by the defaults, so the user loses every binding,
// theme and setting they wrote over one line that mostly works (ADR 0002).
//
// The list travels out on a [LoadResult] so `neru config validate` can print
// it. A warning that only reached the log would be invisible to the command
// people run to check their configuration, which would leave the whole tier
// meaningless.
//
// A nil *Warnings collects nothing, so a caller that only acts on a refusal
// can validate without one.
type Warnings struct {
	messages []string
}

// Addf records one warning, worded as advice about the field it names.
func (w *Warnings) Addf(format string, args ...any) {
	if w == nil {
		return
	}

	w.messages = append(w.messages, fmt.Sprintf(format, args...))
}

// Messages returns what was collected, sorted.
//
// Sorted because most of a configuration lives in maps: the same file walked
// twice reports the same warnings in a different order, and a user comparing
// two runs of `neru config validate` should not have to notice that nothing
// changed. The order that results groups a field's warnings together.
func (w *Warnings) Messages() []string {
	if w == nil || len(w.messages) == 0 {
		return nil
	}

	sorted := slices.Clone(w.messages)
	slices.Sort(sorted)

	return sorted
}
