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
// Running it again is harmless — settled labels settle to themselves — so a
// runtime field change can re-run it to normalize what was typed. What a
// second run cannot do is *re-infer*: a settled label is indistinguishable
// from one the user wrote, so once the labels are filled in, a later change to
// the characters no longer carries them along. That is why the inferring run
// has to be the one that sees the whole configuration.
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
