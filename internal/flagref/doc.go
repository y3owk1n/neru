// Package flagref renders the published mode-flag reference from the grammar's
// descriptor table.
//
// The reference is the table in docs/CLI.md that says which flags a mode
// command accepts. It was kept by hand, which made it a promise rather than a
// fact: a flag added to internal/domain/modecmd appeared in the binary and not
// in the document, and a reader had no way to tell which of the two was wrong.
//
// Rendering it from the same declaration removes that gap. The document keeps
// its prose — what a flag is for in context, where its default comes from, what
// else to read — and gives up only the list itself, which is the part that
// drifted.
//
// [Rewrite] replaces the generated region of a document, so the generator
// writes it and the guardrail test in internal/architecture compares against
// it. A document that is out of date fails the build rather than misleading a
// reader.
package flagref
