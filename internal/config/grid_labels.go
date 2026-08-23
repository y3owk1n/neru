package config

import (
	"strings"

	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// GridCharacters is the coordinate character set a grid is actually built
// from: grid.characters, falling back to hints.hint_characters when it is
// blank. ValidateGrid refuses a blank grid.characters while grid is enabled,
// so the fallback is a floor rather than a configuration a user runs on — but
// it is the floor every grid construction already stood on, and naming it once
// here is what lets the resolved labels below agree with the grid at every
// call site rather than at most of them.
func (c *Config) GridCharacters() string {
	if strings.TrimSpace(c.Grid.Characters) == "" {
		return c.Hints.HintCharacters
	}

	return c.Grid.Characters
}

// GridOptions returns the resolved inputs used to construct a grid.
func (c *Config) GridOptions() domainGrid.Options {
	return domainGrid.Options{
		Characters:     c.GridCharacters(),
		RowLabels:      c.Grid.RowLabels,
		ColLabels:      c.Grid.ColLabels,
		MaxLabelLength: c.Grid.MaxLabelLength,
	}
}

// inferredGridKeys is what an empty grid.row_labels, grid.col_labels or
// grid.sublayer_keys settles to: the characters the grid is built from, in the
// case they are drawn in, or a-z when that set is too small to label with.
//
// It is one function because two callers have to agree on it. The derivation
// fills the blank fields with it, and the validator falls back to it when it
// has no written configuration to consult: a value equal to this one was
// probably inferred rather than written, and a fault in it probably belongs to
// grid.characters, under the name that is in the user's file. Probably is as
// far as a comparison reaches — a label somebody typed can equal the inference
// exactly, which is what [WrittenConfig] answers and #1281 is about. Two copies
// of the rule could drift into reporting a fault twice, or under a field the
// user never wrote.
func (c *Config) inferredGridKeys() string {
	keys, _ := domainGrid.ResolveLabels(c.GridCharacters(), "", "")

	return keys
}

// ResolveGridLabels settles grid.row_labels and grid.col_labels to the labels
// the grid will actually be drawn with, so that reading the option answers the
// question rather than starting one. An empty value means "infer from
// characters", and that meaning used to be implemented in a consumer
// (internal/app/components) where no other reader could see it.
//
// It is derived, not defaulted: it has to run after the whole configuration is
// assembled, because the characters it reads can come from the config file or
// from the override file layered on top. That is the same reason
// ResolveThemeDefaults runs where it does, and it is why the declared default
// in newDefaultConfig() stays the empty string — the empty string is what the
// user writes, not what the daemon runs on.
//
// Running it again is harmless — settled labels settle to themselves — but a
// second run cannot *re-infer*: a settled label is indistinguishable from one
// the user wrote, so once the labels are filled in, a later change to the
// characters no longer carries them along. That is why every caller runs it on
// a configuration nobody has run it on yet, and why a runtime field change
// starts from the one the user wrote rather than the one in use
// (loader.ApplyFieldChange).
//
// It writes to the Config, so it belongs beside the code that assembled it,
// on a Config no other goroutine can see yet — the mode handler reads these
// fields under its own lock.
func (c *Config) ResolveGridLabels() {
	c.Grid.RowLabels, c.Grid.ColLabels = domainGrid.ResolveLabels(
		c.GridCharacters(),
		c.Grid.RowLabels,
		c.Grid.ColLabels,
	)
}
