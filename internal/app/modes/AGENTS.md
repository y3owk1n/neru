# Mode handler — locking contract (deadlock hazard)

`modes.Handler` is split for compiler-enforced locking: the outer `Handler` owns `mu` and the exported entry points (`HandleKeyPress`, `ActivateMode`, `ExitMode`, …), each of which locks and delegates into the embedded `handlerState`, which carries all fields and lock-held methods and has no mutex. Modes are built on `*handlerState`, so calling a locking entry point from locked context is a compile error.

- Deferred callbacks (timers, goroutines) reach the lock via `handlerState.outer` — **never call it synchronously**; from a method running under `h.mu` it self-deadlocks.
- Lock order: `moveMonitorMu` → `h.mu` → `StyleResolver.mu`, never the reverse. Reading a Style under `h.mu` is safe because `overlayStyle()` takes and releases that lock itself and the resolver never calls back into the handler; handler context must never reach the resolver's `applyMu`, which is held across pushes into the render components. Any new mutex needs a stated position in that order.
- Don't hold `h.mu` across blocking calls (IPC, exec, channel sends, overlay draws, modal dialogs) or adapter calls that can synchronously call back into the handler. Overlay draws may block by contract (`internal/adapter/overlay/AGENTS.md`); the hint update callback (`hintdraw.go`) is the one remaining draw under the lock, pending #1203.
- **No method releases a lock it did not take** — every lock is released, via `defer`, by the method that took it; never unlock mid-method to make a blocking call safe. Instead compute a plan under the lock and have the lock-taking method execute it after release (`planIndicatorTick`/`drawIndicators` in `indicator_polling.go`), or hand the blocking call to a goroutine that re-enters through the outer locked surface guarded by the mode-session token (`requestScreenCapturePermissionAndResume` in `hints.go`).

The `Mode` interface is `Activate(modecmd.Activation)`, `HandleKey(string)`, `Exit()`, `ModeType()`; embed `baseMode` (`base.go`) for defaults and register in the handler's mode map. `Handler.ActivateMode` is the only activation entry point (`handler.go`).

Concurrency-sensitive changes need a test that fails under `-race`: `go test -race ./internal/app/modes/`. Run the `deadlock-reviewer` agent on the diff before opening a PR.
