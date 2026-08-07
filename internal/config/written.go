package config

// WrittenConfig is the configuration a user wrote, as validation is allowed to
// see it: not a second Config to read values out of, but the one question a
// warning about a derived value has to ask — is this field in the user's file,
// or did the derivation settle it?
//
// The question needs asking because settling is one-way (see
// [Config.ResolveDerived]). Validation reads the configuration the daemon will
// run on, which is the right one to judge — a resolved label is what the grid
// is drawn with — but by then a value the derivation wrote reads exactly like
// one somebody typed. A warning that cannot tell the two apart cannot name the
// line to go and fix, because it cannot tell whether there is one.
//
// Only the load keeps the answer, on [LoadResult.Written], and only the load
// can hand it here: config sits below loader, so it arrives as an argument
// rather than being fetched.
//
// What it holds is every layer under the derivation and nothing above it — the
// declared defaults, the platform layer, the config file and the override file
// — so "the user wrote this field" is answered as "this field is not blank".
// That reading is exact only while a derived option's declared default is the
// blank that means "infer", which is the rule for derived options anyway
// (internal/config/AGENTS.md) and is pinned by
// TestConfigDefaults_LeaveTheDerivedGridLabelsBlank. Give one a default and the
// warning would name a line nobody wrote — the failure this exists to end.
//
// The zero value knows nothing, and is what [Config.Validate] and every caller
// that never loaded a file passes. Nothing is refused over it — the warnings
// that consult it fall back to comparing the resolved value against what an
// inferred one would be, which is what they did before there was anything
// better to ask.
type WrittenConfig struct {
	cfg *Config
}

// AsWritten hands a configuration to validation as the one the user wrote. A
// nil configuration gives the zero value, so a caller that may not have one
// does not have to branch.
func AsWritten(cfg *Config) WrittenConfig {
	return WrittenConfig{cfg: cfg}
}

// wroteDerived reports whether the field read selects is one the user wrote.
//
// It is for a derived field whose blank written value means "infer from
// somewhere else" — the grid labels, the sublayer keys — so a value at all is a
// value the user typed. resolved and inferred are the fallback for a caller
// with nothing written to consult: what the field settled to, and what it would
// have settled to had it been left empty. Equal, and the field was probably
// inferred; that "probably" is the guess this exists to replace (#1281).
//
// The blankness test is `== ""` rather than a trim because that is the test the
// derivation itself makes (domain/grid.settleLabels): a label of one space is a
// label the user wrote, and it earns its own warning for being untypeable
// rather than being read as an empty one.
func (w WrittenConfig) wroteDerived(read func(*Config) string, resolved, inferred string) bool {
	if w.cfg == nil {
		return resolved != inferred
	}

	return read(w.cfg) != ""
}
