// Package supportref renders the published platform-support table from the
// declarations that own the words.
//
// The table is the list in docs/CROSS_PLATFORM.md of every option, mode flag
// and action that does not do the same thing on all three platforms. Kept by
// hand it would be the fourth copy of a fact that already has three homes, and
// the one nobody reruns — which is how `smooth_scroll` came to be documented as
// a cross-platform block that only the darwin adapter reads
// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md).
//
// Only the narrow rows are published. A vocabulary of several hundred words
// that work everywhere is not a table anybody reads; what a person needs is the
// short list of promises that are not kept here, which is exactly what
// [parity.Declaration.Limited] answers.
//
// [Rewrite] replaces the generated region of a document, so `just gensupportref`
// writes it and the guardrail test in internal/architecture compares against it.
// A document that is out of date fails the build rather than misleading a
// reader.
package supportref
