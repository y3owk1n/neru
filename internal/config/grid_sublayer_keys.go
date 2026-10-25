package config

import (
	"strings"
)

// ResolveSublayerKeys settles grid.sublayer_keys to the keys a subgrid will
// actually be drawn with and navigated by, so that reading the option answers
// the question rather than starting one. An empty value means "use the
// characters the grid is built from". That meaning used to be implemented in
// four consumers walking independent chains that nothing forced to agree. They
// were the mode layer, the component factory and two overlay backends, and the
// last two had an alphabet hardcoded in them.
//
// That disagreement is why it is resolved here. The overlay
// decides which keys are *drawn* and the mode layer decides which keys are
// *accepted*, and they read the configuration separately. An overlay is handed
// GridConfig alone and cannot see hints.hint_characters, so it could not have
// reached the same answer even in principle.
//
// It is derived, not defaulted, for the same reason ResolveGridLabels is. The
// characters it falls back to can come from the config file or from the
// override file layered on top, so it runs once the whole configuration is
// assembled, and the declared default stays the value a user would have typed.
//
// Running it again is harmless and cannot re-infer, because settled keys read
// exactly like typed ones. That is why a runtime field change starts from the
// configuration the user wrote (loader.ApplyFieldChange), so that setting
// grid.characters carries the subgrid along with it.
//
// It writes to the Config, so every caller runs it on a Config no other
// goroutine can see yet. The mode handler reads these fields under its own lock.
func (c *Config) ResolveSublayerKeys() {
	if strings.TrimSpace(c.Grid.SublayerKeys) != "" {
		return
	}

	// The characters the grid is *labeled* with rather than the ones it was
	// configured with. That is the same question ResolveGridLabels asks, and
	// it has to get the same answer. Those two settle to grid.DefaultCharacters
	// when the configured set is blank or too short to label with, and a
	// subgrid resolved to the blank set instead would be drawn with nothing
	// and accept nothing, a keyless subgrid under a labeled grid. ValidateGrid
	// refuses a blank grid.characters, so this is a floor rather than a
	// configuration a user runs on. `neru config set --no-reload` is the path
	// that skips ValidateGrid.
	c.Grid.SublayerKeys = c.inferredGridKeys()
}
