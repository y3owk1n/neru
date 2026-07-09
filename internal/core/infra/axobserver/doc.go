// Package axobserver provides a push-based, resource-safe observer of macOS
// accessibility-tree changes, used to auto-refresh hints while hints mode is
// active.
//
// The Manager is an actor: every mutation (Reconcile, DisarmAll,
// HandleAppTerminated, Close) is a command processed on a single goroutine, so
// callers on different threads never touch the observer map directly and no
// cross-process accessibility call runs under a caller's lock. Observers are
// armed only for the processes a hint scan resolved (see ports.ObservationTarget)
// and only while hints mode is active; when the last observer is disarmed the
// dedicated run-loop thread is stopped and joined, so an idle process has zero
// observer threads and zero background cost.
//
// The platform-specific work lives behind the platform* shims: darwin wires the
// AXObserver bridge in internal/core/infra/platform/darwin; other platforms are
// no-ops until a native change-observer is implemented.
package axobserver
