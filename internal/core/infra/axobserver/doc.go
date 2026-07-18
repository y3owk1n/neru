// Package axobserver watches a macOS accessibility (AX) tree for structural
// changes and reports the pid of the application whose UI changed.
//
// A Manager owns a Platform (the OS observer backend) and watches at most one
// application: the pid the caller passes to Watch. The set of notifications an
// observer subscribes to is fixed and defined in the darwin backend. The
// Platform reports a change by invoking the callback passed to New, carrying
// the firing pid.
//
// The package only observes; it does not decide what to do on a change. The
// caller wires the change callback to its own refresh logic. Until Watch is
// called with a valid pid, nothing is armed and no OS resources are held.
package axobserver
