# The overlay port carries frames for transitions and calls for updates

**Status**: proposed

`ports.OverlayPort` declared 11 methods, but the mode handler does not use it:
`app/modes` and `app/render` talk to `overlay/manager.Interface` — 39 methods —
directly, and `app/component_factory.go` builds the adapter's own render
components and injects them back through six `Use*Overlay` setters matched by
six getters. We decided to make the port load-bearing: the adapter owns
component construction and style resolution, and modes reach it only through a
port that takes **frames** for mode transitions and plain **calls** for
incremental updates.

The hybrid is the part worth recording. A uniform declarative port is the
cleaner design, and transitions genuinely need it — `Show()` → `SwitchTo(mode)`
→ `Draw…` is an ordering ritual repeated at thirteen sites with nothing
checking that any of them gets it right, which is exactly the complexity a
Frame concentrates. But `UpdateGridMatches` and `SetHideUnmatched` fire on
every keystroke in grid mode, and `AGENTS.md` is unambiguous that key handling
getting slower is a regression. Routing the hot path through frame
construction and adapter-side diffing to buy uniformity is the trade this
project does not make. So transitions — rare, ordering-critical, duplicated —
are declarative; updates — hot, already narrow, already correct — are not.

## Considered options

- **Flat imperative port, one method per existing call.** Rejected on the
  deletion test: delete it, call the manager directly, and nothing is lost but
  a rename. It moves the interface without concentrating anything.
- **Fully declarative port.** Rejected for the keystroke cost above, not on
  design grounds. If the per-frame cost is ever measured and found negligible,
  this ADR is the thing to revisit.
- **Widening `ports.OverlayPort` to cover what modes already do.** Rejected:
  the port's width is the symptom. Two of its eleven methods were dead when
  this was decided — `Show` had no callers, and `ShowGrid` hardcoded an
  alphabet and was reachable only through `GridService.ShowGrid`, which
  nothing called. Both were deleted in #1204, leaving nine.

## Consequences

- **Overlay component construction moves out of `app` and into the adapter.**
  Done in #1209: `manager.Base` builds the render components from a
  configuration and a theme provider, on the surface each backend states for
  itself, and the six setters, six getters and `WindowPtr()` left the
  interface with them. No app package names a platform pointer type any more.
  The manager also owns handing those components a new configuration
  (`ConfigureComponents`), so the resolver notifies one owner instead of six.
  `config + theme → style` got its single owner earlier, in #1207.
- **Startup phases move.** Done in #1209: `internal/app/new.go` phase 4 asks
  the overlay to build its components and phase 5 no longer registers them, so
  the two overlay phases became one call. The phases themselves, their
  numbering and their individually-unwound cleanups are unchanged, and the one
  failure that could fail phase 4 before still fails it.
- **Headless detection has a new signal.** Done ahead of the move in #1205:
  `component_factory_headless.go` asked `WindowPtr() == nil`, and then asked
  the overlay through the optional `manager.HeadlessReporter` capability,
  pinned by tests on the no-op, macOS and Linux managers. With #1209 that file
  is gone: each backend answers its own headlessness where it builds, and
  removing `WindowPtr` was the deletion this predicted.
- **The port's threading contract stays "may block; never call under
  `h.mu`".** Draws are `dispatch_async` on macOS and hold `renderMu`
  synchronously on Linux; that asymmetry is left alone here. Modes compute
  under the lock and draw after releasing it, which is what lets the three
  naked `h.mu.Unlock()` sites be deleted rather than relocated. Making draws
  non-blocking everywhere would need a Linux dispatch queue and would change
  where draw errors surface — a separate decision.
- **Platform parity divergence is not addressed.** `Flush()` is a real flush on
  Linux and Windows and an empty no-op on darwin; `hints.ui.placement` is
  honoured on darwin and Linux but not Windows; `DrawRecursiveGrid` drops four
  parameters on Windows. A frame port makes these easier to express as declared
  capabilities. It does not declare them.
