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
- **The hybrid landed as three methods, not two.** Done in #1210: `ShowFrame`
  is the transition and owns `ResizeToActiveScreen` → `Show` → `SwitchTo` →
  draw; `RedrawFrame` takes the same Frame and skips the window sequence,
  because the hint update callback fires on activation *and* on every
  narrowing keystroke and a `Show` per keystroke is main-thread work this
  change is not allowed to add; `ClearFrame` is the leaving half. `Frame` is a
  sealed interface with one implementation so far, `HintsFrame`, and
  `internal/architecture/overlay_frame_test.go` fails the build if a Frame
  field is typed from anything but the standard library or `internal/domain/`.
  The dead `ShowHints` and `Hide` left the port with the two service methods
  that were their only callers.
- **Both grid surfaces followed hints through the port.** Done in #1211:
  `GridFrame` and `RecursiveGridFrame` carry what each surface should show, the
  app-layer renderer in `app/render` was deleted with its last caller,
  and the ten positional parameters of `DrawRecursiveGrid` became one frame
  built in one place. The hybrid held where it was predicted to matter: grid's
  narrowing stayed on `UpdateGridMatches` / `SetGridHideUnmatched` and gained a
  measurement to prove it (interleaved benchstat over the simulation harness:
  time unchanged at p=0.97, allocations identical), while recursive grid —
  which has no incremental path and repaints its whole surface per keystroke —
  now boxes one frame per key: one extra allocation of 128 B, with no
  measurable time cost. That is the price this ADR said a fully declarative
  port would charge on every keystroke, paid only where the surface was
  already being repainted anyway.
- **The update half widens; it does not become a frame.** #1492 is the first
  time the hybrid's imperative side hit the problem the declarative side solves
  by construction: grid mode's selection keystroke opens a subgrid *and* moves
  the pointer, and on Linux both are painted into one Cairo target, so two
  calls were two full repaints of it per key — held arrow keys inside a subgrid
  included. The frame path would have fixed it and is exactly what this ADR
  rules out here, so `ShowGridSubgrid` took the pointer as an argument instead,
  the way `RecursiveGridFrame` already carried it. Measured the way this ADR's
  previous entry was, interleaved over the simulation harness (n=8, Apple M3):
  the selection keystroke's grid-surface updates went 2 → 1 (p=0.000) with time
  and allocations unchanged (~4.74µs → ~4.79µs, p=0.65; 18 allocations either
  way), and grid narrowing — the keystroke this ADR exists to protect — stayed
  unchanged again (~5.25µs both ways, p=0.80). Widening a hot call is therefore
  the move when a keystroke changes two things on one surface, and the frame
  path is still not. The leaving half took the matching share: `ClearFrame`
  drops what the update calls left, because a mode resetting it by hand was a
  mode repainting a surface it was about to clear.
- **The last two surfaces converted, and the mode handler kept one overlay
  reference.** Done in #1212: `MonitorSelectFrame` carries the displays on
  offer, and `ScrollFrame` carries nothing at all — scroll is a mode the
  indicators name rather than a surface with content, but entering it is still
  a transition, so saying that with a Frame is what leaves one path from a mode
  to the screen. Drawing the picker stays an **optional capability reached by
  type assertion**: the assertion moved from the mode into `Adapter.draw`,
  where a backend without `manager.MonitorSelector` reports `CodeNotSupported`
  and the mode refuses to activate, so the port never grew a monitor-select
  method. `ClearFrame` takes the panels down as well as the shared surface —
  they are not drawn on it, and a caller that had to take half the picker down
  itself was a caller that could leave it behind. With `SetActiveScreen` and
  `Flush` on the port for the two calls that were neither frame nor draw,
  `handlerState.overlayManager` and `handlerState.overlayStyles` were deleted:
  the handler holds `overlayPort` and nothing else it draws through — the
  Linux keyboard-capture extension is still reached through the package
  singleton, and draws nothing — and `setMode` went with them, because
  switching the overlay to a mode is now something only realizing a frame does.
  The window sequence is the one thing a frame varies, and only the picker
  varies it: its panels are windows of their own. Scroll draws nothing yet
  still brings the shared window up, because on Linux the indicators that name
  the mode are painted on that surface, and deciding otherwise would put
  macOS's one-window-per-indicator model into shared code.
- **The hint search input went with hints.** Its geometry was computed in
  `app/modes` from configuration and handed over as a render model; it is now
  a `SearchInputLayout` resolved with the rest of the Style and placed in the
  adapter, reached through `DrawHintSearch` / `HideHintSearch`. The IME field
  asks where it landed through `HintSearchBounds` rather than deriving the
  same rectangle a second time.
- **The boundary became the layering test, and the app stopped naming the
  overlay at all.** Done in #1213: `internal/adapter/overlay` left
  `sharedInfraPackages` in `internal/architecture/layering_test.go`, and the
  existing tests pass with only that data deleted. Four app-to-overlay calls
  that had been reaching past the port moved onto it — `ApplyConfig` and
  `RefreshStyles` (the single notification a reload or a theme change owes the
  Style owner), `SetHiddenInScreenShare` and `Destroy` — so `App` holds one
  overlay reference where it held three, and `app.WithOverlayManager` became
  `app.WithOverlayPort`. Two things that were never overlay vocabulary moved
  up: the per-mode `Context` types, which sat in `render/{hints,grid,
  recursivegrid}` and knew no colour and no surface, are now
  `internal/app/components/{hints,grid,recursivegrid}`; and the render
  `Overlay` handles on the app's component structs, which existed only for
  three "was this built" guards in `app/lifecycle.go`, are gone with the
  guards. The screen-change path lost its open-coded `ResizeToActiveScreen` →
  refresh → `Show`: hints and recursive grid now hand over a *transition* the
  way grid already did, so the window sequence is realized where every other
  transition realizes it. The Linux keyboard grab left the package singleton
  for `SetKeyboardCaptureEnabled` on the port. It is a plain method, not an
  optional capability: the adapter would have implemented the extension
  unconditionally, so asserting it would have declared nothing. What confines
  the behaviour to Linux is the caller's existing gate on the event tap, and
  the real capability assertion stays where it means something, on
  `manager.KeyboardCaptureController` inside the adapter.
- **The no-op manager is gone from the shipped binary.** Its last production
  caller left with the wide interface's last app-side user, and the simulation
  harness — the reason it existed — now implements `ports.OverlayPort` outright
  and records Frames instead of overriding eight of thirty-nine manager methods.
  What that cost is one assertion: the journeys can no longer read the resolved
  hint colours, because no resolver runs behind a fake port. They assert instead
  that the change reached the overlay — the reloaded config it was handed, the
  re-resolution a theme change asked for — and the colours stay pinned in
  `TestStyleResolver_RefreshPicksUpTheNewTheme`, one seam down, where they are
  produced. `manager.Interface` is still thirty-nine methods on purpose: it is
  the adapter-to-backend contract, out of scope here, and the headless base its
  own tests build fakes on now lives in `headless_manager_test.go`.
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
