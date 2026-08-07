package grid

import "strings"

// SubgridKeys is the key set a subgrid of cells is drawn with, which is
// the same set it is navigated by: the configured keys, in the case they are
// drawn in, capped at the number of cells there are to name.
//
// It exists because those two sets have to be one set. Every overlay backend
// decided the drawn set for itself and the manager decided the accepted one,
// each trimming, upper-casing and capping on its own, so a subgrid could draw a
// label that did nothing or accept a key it had never shown. Both sides call
// this now, and the config resolves the keys they pass it
// (config.ResolveSublayerKeys), so there is one answer rather than five.
//
// The cap is the caller's cell count rather than a constant here, because the
// caller is the one that knows how big the subgrid it is about to draw is —
// taking it as a parameter is what lets each side drop the second cap it used
// to apply on its own.
//
// Keys past the last cell are dropped rather than refused: a longer set is what
// a user gets by pointing sublayer_keys at their whole alphabet, and refusing
// the configuration over it would cost them the grid. The keys that name a cell
// are the keys, and the rest were never drawn.
func SubgridKeys(keys string, cells int) []rune {
	runes := []rune(strings.ToUpper(strings.TrimSpace(keys)))

	return runes[:max(0, min(len(runes), cells))]
}
