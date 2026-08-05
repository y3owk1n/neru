# Overlay adapters — contracts

These rules fail silently at runtime, not at compile time; read before editing.

- **Threading is platform-asymmetric.** macOS serializes through the Obj-C bridge (`dispatch_async` to the main thread); Linux must serialize itself — Cairo/X11/Wayland calls are not thread-safe (`linux/manager.go`).
- **Lock topology is deliberate.** The manager owns `renderMu`, held across synchronous draws; animation goroutines lock it via `sharedOverlay`. The mouse-action indicator owns an independent X11/Wayland connection and must **not** share `renderMu` — it has its own `indicatorMu` / `indicatorRenderMu`.
- **Surface primitives split** (#1177): layout, animation, offsets, and label logic live once on `sharedOverlay`; only buffer management, HiDPI scale, and window lifecycle go behind `overlaySurface`. Shared code never touches cgo — primitives take Go types and own their C marshaling, including CString lifetimes.
- **Backend exported methods are thin nil-guarded delegates** — a promoted method through a nil backend pointer panics before any receiver guard runs. `Get()` must never return a typed nil, or every `!= nil` guard downstream silently passes (`backend_linux.go`).
- **`originOffset` applies selectively**: grid, recursive-grid, and hints arrive screen-local and get the offset — including the virtual pointer, which rides the recursive-grid frame. Absolute draws (the cursor-tracking mode and sticky-modifiers indicators, monitor_select, click indicator) must not.
- **Layering exception**: app code may import this package for vocabulary and render models only (`layering_test.go` `sharedInfraPackages`); behavior still goes through `ports.OverlayPort`. `render/*` owns style and drawing only — state and math live in `internal/domain/*` or `app/services/*`. Optional backend capabilities are reached by type assertion, not interface widening.
