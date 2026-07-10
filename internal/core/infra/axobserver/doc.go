// Package axobserver watches macOS accessibility (AX) trees for structural
// changes and reports the pid of any application whose UI changed.
//
// A Manager owns a Platform (the OS observer backend) and keeps a set of armed
// pids reconciled with the desired observation targets. Each target names a pid
// and a structural Mask (which notifications to watch). The Platform reports a
// change by invoking the callback passed to New, carrying the firing pid.
//
// The package only observes; it does not decide what to do on a change. The
// caller wires the change callback to its own refresh logic. Until Reconcile is
// called with a non-empty target set, nothing is armed and no OS resources are
// held.
package axobserver
