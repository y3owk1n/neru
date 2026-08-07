package config

// ResolveDerived settles every value the daemon runs on that the user did not
// write: the derivation, in the order it has to happen.
//
// It exists so that there is one list rather than one per caller. Each step
// writes a derived value over the written one, which destroys the difference
// between the two — run a step twice and the second run reads its own output,
// not the user's input — so a caller that ran some of these and not others
// would leave a configuration nobody can re-derive. Adding a derivation that
// writes to the Config means adding it here;
// TestEveryConfigDerivationRunsInTheChain fails when it is not.
//
// The loader has one more step of its own, which is why it wraps this rather
// than replacing it: see loader.ResolveDerived. This package cannot reach that
// one — config is below loader, and has to stay there.
//
// It writes to the Config, so every caller runs it on a Config no other
// goroutine can see yet; the mode handler reads these fields under its own lock.
func (c *Config) ResolveDerived() {
	c.ResolveThemeDefaults()
	c.ResolveGridLabels()
	c.ResolveSublayerKeys()
}
