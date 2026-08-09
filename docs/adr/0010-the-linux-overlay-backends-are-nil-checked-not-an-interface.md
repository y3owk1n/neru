# The Linux overlay backends are nil-checked, not an interface

**Status**: proposed

`internal/adapter/overlay/linux/manager.go` is 1,251 lines carrying 52
`m.x11 != nil` / `m.wlroots != nil` checks — 62 counting the `== nil` forms and
the two lazily-built indicator overlays — and almost every exported method is
the same three lines: guard one pointer, else guard the other. The architecture
review read this as two types with identical exported method sets that no Go
interface declares, and proposed declaring one.

Neither half holds. The sets are not identical: `x11Overlay` has `Scale()`
(`x11_cgo.go:65`), consumed at `manager.go:1015-1021` because Wayland renders in
logical units and returns a hardcoded `1`; `wlrootsOverlay` has `startPoller`,
`setKeyboardCaptureEnabled`, `keyboardPoller` and `selectAvailableBuffer`, none
of which X11 has an analogue for. An interface would carry four methods dead on
one side. The decisive half is worse: both constructors return a raw `nil` when
the native call fails (`x11_cgo.go:48-50`, `wayland_cgo.go:56-58`), so assigning
one into an interface field yields a non-nil interface around a nil pointer and
**all 52 guards pass at once**. That is not a hypothetical — it is the failure
`overlay/CLAUDE.md` names for the process-wide accessor, and the reason
`backend_linux.go:21-29` unwraps a nil rather than returning it.

So we decided: **the two Linux overlay backends stay concrete pointers
dispatched by nil-check, and the seam between them is `overlaySurface`.** The
consequence worth recording is the one that reads as a mistake: the nil-check
density in `manager.go` is the price of a correctness property, not an interface
nobody got round to writing.

The seam already exists and already says so. `overlaySurface`
(`overlay_shared_cgo.go:29-76`) declares fifteen unexported primitives, both
types satisfy it (`x11_cgo.go:58`, `wayland_cgo.go:68`), and its doc comment
states the design directly: everything above the primitives is identical between
X11 and Wayland and lives once on `sharedOverlay`, which is 1,003 lines of
already-deduplicated Go. The seam is drawn at the layer where the two backends
genuinely differ — buffers, scale, flush — and deliberately not at the exported
surface. This ADR extends that seam rather than adding a second one beside it.

## Considered options

- **Declare an interface over the exported set.** Rejected above on both counts.
  Worth stating why the typed-nil objection is not answerable by discipline:
  the guards are spread over 52 sites in one file, the failure is silent, and it
  degrades to a nil dereference inside cgo rather than a Go panic with a useful
  frame. A convention that every constructor must return a non-nil value or the
  caller must convert to an untyped nil is a convention that will be broken once
  and not noticed.
- **Force the interface with no-op methods** — `Scale()` returning `1` on
  Wayland, empty `startPoller` on X11. Rejected on the same typed-nil ground,
  and because it inverts a real asymmetry into a lie: Wayland has no scale to
  report and X11 has no poller to start, and a no-op says they do.
- **Leave the exported delegates as they are.** The status quo, and the option
  the previous bullet's rejection does not automatically rule out. Rejected
  because 13 of the 14 duplicated exported methods contain no cgo call at all —
  they are a nil-guard prologue plus one delegate into `sharedOverlay`
  (`x11_cgo.go:79-245` against `wayland_cgo.go:83-258` is 88.6% identical once
  the receiver type name and C symbol prefix are normalised). The only thing
  keeping them per-backend is that `raw` is `*C.NeruX11Overlay` on one side and
  `*C.NeruWaylandOverlay` on the other, which one accessor method resolves.

## Consequences

- `overlaySurface` gains an `alive() bool`, and the fourteen exported delegates
  — `Hide`, `Clear`, `ClearRect`, `UpdateGridMatches`, `ShowSubgrid`,
  `SetHideUnmatched`, `DrawGrid`, `DrawRecursiveGridWithSubKeyPreview`,
  `DrawBadge`, `Flush`, `DrawMonitorSelect`, `DrawHints`,
  `DrawMouseActionIndicator`, `setOriginOffset` — move onto `sharedOverlay` and
  are promoted into both types. `Show`, `Resize` and `Destroy` stay per-backend:
  Wayland re-runs buffer setup before showing, layer shells auto-resize so
  `Resize` is empty, and Wayland's `Destroy` waits on the keyboard poller.
- `setRenderMu` and `setDisplayMu` are one operation under two names, both
  forwarding to `setRenderMuShared` (`x11_cgo.go:238`, `wayland_cgo.go:240`).
  They unify to one name. This costs nothing and is the precondition for the
  point above.
- The manager's 52 guards stay, and this ADR is what stops the next reader
  removing them.
- Roughly twenty exported `Manager` methods no-op silently when no backend is
  attached, which reads against `AGENTS.md`'s rule that unsupported behaviour
  returns `CodeNotSupported` rather than a silent no-op. It is not a breach of
  that rule: every method that *has* an error to return does return it — five of
  five draws, pinned by `stub_contract_test.go:35-129`. The rest are declared
  errorless by `manager.Interface`, and `ports.OverlayPort` declares `Flush()`
  errorless too. Widening those signatures is the architecture review's
  candidate 2 and ADR 0003's frame port, not this decision.
- `NewOverlayManager` returns a nil `*Manager` for `linuxOverlayBackendUnknown`
  (`manager.go:129-131`) and `Init` passes it straight out as a `manager.Interface`
  without the unwrapping `Get` does (`backend_linux.go:13` against `:21-29`).
  Only 4 of 30 exported methods guard a nil receiver. This is unreachable today:
  the unknown backend covers GNOME, wayland-other and no-display
  (`manager.go:716-718`), and `platform.NewSystemPort` already refuses all three
  at the first step of daemon startup, so the daemon exits before an overlay
  exists (`docs/CROSS_PLATFORM.md:104-110`). It becomes reachable the moment a
  compositor moves out of that bucket — COSMIC (#898) is `wayland-other` today
  and `CROSS_PLATFORM.md:1023` already records that layer-shell works there.
  Whoever lands COSMIC support fixes `Init` in the same change.
- Out of scope, and a real defect: `Hide` and `Clear` read `m.x11`/`m.wlroots`
  outside `renderMu` — deliberately, to avoid deadlocking the animation
  goroutine (`manager.go:167-175`, `:207-215`) — while `Destroy` nils both under
  it (`manager.go:283-289`). That is an unsynchronised read racing a
  synchronised write, and `sharedOverlay.cancelAnimation`
  (`overlay_shared_cgo.go:524`) is the one backend method with no nil-receiver
  guard, so losing the race dereferences nil. It is shutdown-only and
  race-detector-visible. It is a locking bug, not a dispatch-shape bug, and
  moving the delegates does not fix it.
- No new word enters `CONTEXT.md`. *Overlay* already names the thing, and the
  two backends are an implementation detail below the port — naming them in the
  glossary would be the first entry that describes how something is built rather
  than what it means.
