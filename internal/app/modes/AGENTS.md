# Mode handler — locking contract (deadlock hazard)

`modes.Handler` is split for compiler-enforced locking: the outer `Handler` owns `mu` and the exported entry points (`HandleKeyPress`, `ActivateMode`, `ExitMode`, …), each of which locks and delegates into the embedded `handlerState`, which carries all fields and lock-held methods and has no mutex. Modes are built on `*handlerState`, so calling a locking entry point from locked context is a compile error.

- Deferred callbacks (timers, goroutines) reach the lock via `handlerState.outer` — **never call it synchronously**; from a method running under `h.mu` it self-deadlocks.
- Lock order: `moveMonitorMu` → `h.mu`, never the reverse. Any new mutex needs a stated position in that order.
- Don't hold `h.mu` across blocking calls (IPC, exec, channel sends) or adapter calls that can synchronously call back into the handler.

The `Mode` interface is `Activate(modecmd.Activation)`, `HandleKey(string)`, `Exit()`, `ModeType()`; embed `baseMode` (`base.go`) for defaults and register in the handler's mode map. `Handler.ActivateMode` is the only activation entry point (`handler.go`).

Concurrency-sensitive changes need a test that fails under `-race`: `go test -race ./internal/app/modes/`. Run the `deadlock-reviewer` agent on the diff before opening a PR.
