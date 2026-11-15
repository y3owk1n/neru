// Package hookref renders the two [hooks] tables in
// docs/reference/configuration.md, the hooks and the variables their exec
// steps see, from the event catalog in internal/domain/event.
//
// The catalog is also what sets those variables and what `neru describe`
// lists, so the page, the manifest and a running hook cannot disagree.
package hookref
