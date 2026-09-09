// Package axobserver watches a macOS accessibility (AX) tree for structural
// changes and reports when the watched application's UI changed.
//
// The observer is process-wide: at most one application is watched at a time,
// the pid last passed to Watch. The set of notifications the observer
// subscribes to is fixed and defined in the darwin backend. Changes are
// reported through the callback installed with Init.
//
// The package only observes; it does not decide what to do on a change. The
// caller wires the change callback to its own refresh logic. Until Watch is
// called with a valid pid, nothing is armed and no OS resources are held.
package axobserver
