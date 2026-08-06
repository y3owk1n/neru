# Mode-lifecycle dispatch runs under one lock hold

**Status**: accepted

Putting a mode's **Frame** back on screen after a monitor move was a switch over
all five modes in `internal/app/modes/monitor.go`, dispatched *unlocked*. Every
arm was a `*Handler` method that took `h.mu` for itself and then re-read the
active mode as a TOCTOU guard, because the mode could have been exited between
the switch reading it and the arm acting on it — the cursor warp in between is
animated, and a user can press escape during it. Five arms, five guards, five
lock acquisitions for one decision, and no way for a reader to tell which of
the guards were load-bearing.

We decided that monitor-move refresh becomes the fifth method on the core
`Mode` interface, and that the dispatching entry point takes `h.mu` once, reads
the active mode once, selects it and calls it. It is core rather than an
optional extension because all five modes participate: even scroll, which draws
nothing, has to switch the overlay back to the mode its indicators name.

The locking change is not a separate decision, which is why it is not a
separate ADR. Mode implementations are built on `*handlerState`, which has no
mutex and cannot reach the locking surface — that is the compiler-enforced
split the area guide describes, and this work does not touch it. So the moment
a refresher becomes a mode method, the lock has to move up to the caller. The
five re-checks then guard a window that no longer exists and are deleted rather
than kept.

That was verified against the one hazard rather than assumed. `moveCursorToMonitor`
is clear → warp → redraw: `clearFrameForMonitorMove` takes and releases `h.mu`,
`MoveCursorToPointAndWait` runs the animation and `WaitForCursorIdle` with no
handler lock held, and only then does the dispatch take `h.mu`. A single hold
across the dispatch therefore does not span the animation, and the rule against
holding `h.mu` across blocking calls survives. The macOS `dispatch_sync` that
hides the monitor picker's panels is not newly reachable either: the clear
before the warp already took them down and reset the adapter's drawn flag, so
the `ShowFrame` on the other side finds nothing to hide, and the picker's own
frame skips that branch by construction.

## Considered options

**Keeping the refreshers as `*Handler` methods and dispatching through a
registry that is not the `Mode` interface.** This is the option that preserves
today's locking exactly: each refresher keeps taking `h.mu` for itself, the
five re-checks stay, and nothing new comes under the lock. Rejected. It buys
the locality and pays for it with a second dispatch mechanism sitting beside
the mode map — the "two mechanisms for one thing" that deepening `Mode` exists
to remove — and nothing would check that every mode is in the registry, where
the `Mode` interface makes a missing implementation a compile error. Worse, it
makes the five guards permanent: each entry would still be re-reading the mode
it was just selected by, and a reader would have no way to tell a guard that
closes a real window from one that defends against a race the caller already
prevented. This ADR exists so that a future reader who finds five guards
deleted in the history knows they were removed because the window closed, not
because someone judged the race unlikely.

**Widening the lock to `MoveMonitor` itself**, one hold across clear, warp and
redraw. Rejected on the rule above: it spans the animated cursor move and the
idle wait after it, which is exactly the blocking call `h.mu` must not be held
across.

**Locking once around the existing switch and leaving the arms where they
are.** Rejected as a half-step. The arms are `*Handler` methods that lock, so
this is only sound once they have moved onto `*handlerState` — and once they
have, the switch is doing by hand what the mode map already does, at the cost
of keeping mode knowledge in a switch the modes work exists to delete.

## Consequences

- **The five TOCTOU re-checks are gone.** The mode cannot change between being
  chosen and being used, so an implementation must not re-check it; the `Mode`
  method's doc comment says so, because the guard reads reasonable in isolation
  and would be re-added otherwise.
- **A mode that changed during the warp is now refreshed, where before nothing
  was.** This is the one behaviour difference, and it follows from deleting the
  guards rather than from the lock. If the user leaves hints and opens grid
  while the cursor is still travelling, the old dispatch had already chosen
  hints' arm, which then found grid active and did nothing — leaving grid sized
  for the display the cursor left. The new dispatch reads the mode after the
  warp and refreshes grid against the display the cursor is now on, which is
  what the user is looking at. On the warp-failure path the same applies with
  the source display, which is equally where the cursor still is.
- **Hint regeneration on this path moves under `h.mu`.** It is the only work
  that is newly inside the hold, and it is not a new *kind* of work: the
  activation path and the screen-change path both walk the accessibility tree
  under the same lock. What it buys is the end of a real data race — the
  session's filter roles, strategy and label-direction overrides were read with
  no lock held while a concurrent mode exit reset them. That is pinned by
  `TestRefreshActiveModeOnNewScreen_ReadsTheModeSessionUnderTheHandlerLock`,
  which fails under `-race` against the dispatch this ADR replaces.
- **The hold is now as long as the slowest refresh**, so that refresh had to
  be given a bound. The hints walk gets the same `HintTimeout` budget the
  activation path gives it; the context a `move_monitor` step arrives with
  carries no deadline of its own, and an accessibility tree that never answers
  would otherwise pin `h.mu` — and with it every keystroke — indefinitely. The
  screen-change refresh has the same unbounded context today and is left alone
  here, because it is not this change's hold; it should be bounded when those
  refreshers move under one hold too. No keystroke runs this path — it is
  reached only from `move_monitor` — so the latency budget `AGENTS.md` protects
  is untouched.
- **Idle is answered by absence rather than by an arm.** `domain.ModeIdle` has
  no entry in the handler's mode map, so a monitor move with nothing open draws
  nothing — the same nothing the deleted `case domain.ModeIdle` said.
- **`exhaustive` no longer covers this dispatch site.** The compiler does
  instead: a mode that does not implement the method does not satisfy `Mode`
  and cannot be registered in `newModes`. That is a stronger guarantee than the
  linter gave, and it is why monitor move is core rather than optional — an
  optional extension would have let a mode fall silently into the
  not-implemented branch.
- **The screen-change and theme-change refreshes keep their guards**, and those
  guards are not vestigial. They are still `*Handler` methods called from the
  app's screen-change goroutine, which snapshots the active mode without
  holding `h.mu`, so the window they close is real. They become mode behaviour
  in a later change, and their guards go the same way this one's did — when the
  dispatch that selects them takes the lock.
