# The focused app is published, not polled

**Status**: accepted

Resolving a per-mode hotkey asks the operating system which application is
focused, on the keystroke that needs it, with `h.mu` held
(`app/modes/key_dispatch.go:141-144`, `handler.go:429-442`). On macOS that is
`AXUIElementCopyAttributeValue` against the system-wide element
(`accessibility_element_darwin.m:25-48`) — a message to another process, in a
repo that never calls `AXUIElementSetMessagingTimeout`, through a client that
discards the context it is given (`accessibility/native/client.go:78`). We
decided that the app watcher **publishes** the focused app into a cell the mode
handler owns, and that the [[Keymap]] — the bindings in force for the current
mode with that app's overrides applied — is **settled** when the mode, the
focused app or the configuration changes. A keystroke consults a keymap; it
never builds one, and it never asks the operating system anything.

The part worth recording is the line this draws, because the obvious reading —
"no accessibility work under `h.mu`" — is false here and ADR 0004 makes it
false on purpose: the activation path and the screen-change path both walk the
accessibility tree under that lock. The line is **a one-shot the user is
waiting on may block under the lock; a keystroke may not block at all.** A
person who pressed the hints hotkey is waiting for hints and a walk that stalls
is visible as the thing they asked for being slow. A person typing a hint label
is not waiting on anything, and the same stall is the daemon freezing — mode
exit included, since it takes the same lock.

## Considered options

- **Bound the query with a context deadline.** The first answer, and it does
  not work: the context is discarded before the C call, so a deadline has
  nothing to check between itself and the thing that blocks. ADR 0004's
  `HintTimeout` is real only because `darwin/tree.go` checks `ctx.Done()`
  between nodes — it bounds a walk, never one message. A genuine bound needs
  `AXUIElementSetMessagingTimeout` in the darwin bridge, which is a change to
  every accessibility call in the process and is filed on its own.
- **Memoize inside the accessibility adapter.** Smallest diff, and it makes
  every caller faster without any of them asking. Rejected: `hints.go:157` and
  the scroll and hint services need a fresh answer at activation, and a cache
  behind a port method changes what they get without changing what they say.
  The staleness belongs to the keymap, which can state it, not to the port.
- **Settle synchronously from the watcher callback.** Rejected on the locking
  contract. `Watcher.dispatchAppEvent` calls its callbacks inline under
  `w.mu.RLock()`, and on macOS the notification arrives on the main queue.
  `internal/app/modes/AGENTS.md` records that the `dispatch_sync` in
  `clearFrameForMonitorMove` is safe only while nothing on the main queue takes
  `h.mu`. So the callback publishes into an atomic cell and takes no lock; the
  handler settles lazily inside a locked entry point it was going to enter
  anyway.
- **Defer the update to mode exit, as the global hotkey path already does.**
  `handleAppActivation` defers hotkey *registration* during a mode
  (`app/lifecycle.go:301-309`), and copying that would have been free.
  Rejected: the stated reason for that deferral is re-entry during OS hotkey
  registration, which settling an in-memory keymap does not do, and the case it
  would break — passthrough out of a mode into another app, then keep
  navigating — is the case per-app hotkeys exist for.
- **Normalize hotkey tables at config load, as a separate earlier change.**
  Rejected once the pipeline was read: `loader/load.go` has no derivation step,
  so this invents one, and it has to run on every path that builds a `Config`
  — the loader, `DefaultConfig()`, `config set`, every test literal — or key
  matching differs between a loaded config and a default one. The normalized
  index has one hot consumer, and the keymap already exists to hold it.

## Consequences

- **Staleness is bounded by watcher latency, and that is a real bound only
  where a watcher exists.** darwin and Linux have one; Windows and `other` get
  an empty `platformStartWatcher()`, so there the keymap settles by asking.
  (Windows has since gained a watcher, #1572; the reasoning below still holds
  for `other` and for the pre-watcher ask.)
  That is affordable precisely there: `FocusedApplication` on Windows is
  `GetForegroundWindow` plus `GetWindowThreadProcessId`
  (`platform/windows/win32.go:351-366`) — local, and unable to block on another
  process. The hazard this ADR removes is specific to cross-process
  accessibility, which is also specific to the two platforms that can be told
  about focus instead of asking. One `resolveFocusedApp` covers both: read the
  cell when it is fed, ask when it is not. `docs/CROSS_PLATFORM.md` gains the
  row.
- **Publishing only works while something is listening, so the order the daemon
  starts in became load-bearing.** Polling was self-healing: whoever asked got
  an answer, however late they asked. A publication has one delivery and no
  retry, so a watcher started before `App.Run` registers its activation callback
  drops what it reports, and on Linux — where the watcher samples once as it
  starts and reports only changes after that — the lost sample is not
  re-reported until the user switches application again. Until then the keymap
  settles by asking, which is what this ADR set out to avoid, and per-app
  overrides bind to whatever was focused when the mode opened. #1348 is that
  bug; `app/lifecycle.go` registers before it starts, and the simulation journey
  pins the order.
- **`handlerState.focusedBundleID` is deleted.** Leaving it would leave the
  regression writable; deleting it means the only way to learn the focused app
  inside the handler is the published cell. Callers that legitimately need a
  fresh answer reach the service directly with their own context, as
  `hints.go:157` and `hints_debug.go:33` already do.
- **The cell will be reached for by things that cannot tolerate stale.** This
  is the trap, and it is why this is written down rather than left to the diff:
  once one consumer reads "the focused app" from a cell, the next one will too,
  including ones that must not. The cell is named and documented for the keymap;
  a consumer that needs the truth asks the port.
- **The keymap is what makes this a deepening rather than a cache.**
  `syncModifierPassthrough` merges the same bindings twice more
  (`passthrough.go:45,106`) on the same triggers; those reads come from the
  settled keymap. Delete the keymap and you lose the single answer to "what is
  bound right now", not merely some speed.
- **The claim is reliability, not measured latency.** The cost removed is a
  call with no upper bound, on a path that holds the lock every other path
  needs. How many microseconds it also saves is unmeasured, and the change ships
  with a benchmark and a harness assertion that a keystroke inside a mode makes
  zero focused-app calls, rather than with a number in the subject line.
