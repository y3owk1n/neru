// Package parity declares which platforms a word a person can write actually
// does something on.
//
// Parity is a promise about every name a person can write — every option, mode
// flag and action — rather than about whether a subsystem works
// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md). A capability
// matrix cannot see the difference between an option that works and one that is
// merely accepted: `smooth_scroll` reached the schema, the validators and the
// documentation while only the darwin adapter ever read it, and every check in
// the tree stayed green.
//
// This package holds the vocabulary that difference is stated in — a
// [Platform], the [Platforms] column a word carries, and the [Word] itself. It
// holds no table: a word is declared beside the declaration that already owns
// it, so the config options are declared in internal/config, the mode flags in
// internal/domain/modecmd and the actions in internal/domain/action. The
// load-time warning, the `neru doctor` row and the documentation table are
// projections of those, and the guardrails in internal/architecture fail the
// build when a word is neither supported everywhere nor declared.
//
// The column is a set of platforms rather than a darwin-only flag, and every
// platform's row is filled from the first declaration. Windows has known gaps
// of its own; retrofitting a column into a boolean later would mean touching
// every call site.
package parity
