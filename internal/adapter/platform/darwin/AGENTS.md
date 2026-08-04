# Darwin bridge — native-boundary contracts

Read `../AGENTS.md` first (slots, stubs, coordinates). This file covers the ObjC/cgo boundary; the authoritative memory & threading guide is `docs/go/OBJECTIVE_C.md`.

- **ARC is on** (`-fobjc-arc`): `retain`/`release` are compile errors. Transfer idiom: `__bridge_retained` to hand ownership out, `CFBridgingRelease` to take it back, plain `__bridge` to borrow.
- **AX ownership**: every `AXUIElementRef` handed to Go is +1 retained and Go-owned, balanced exactly once by `Element.Release()`; borrow-only functions must not touch the refcount (rule in `accessibility.h`). The leak gate lives in *another package*: `TestTreeWalk_ReleasesEveryElement` (`internal/adapter/accessibility`, tagged `integration && darwin`) — run it after touching `accessibility*.m`.
- **C→Go callbacks funnel through `cgoSlot[T]`** (`cgo_slot.go`, generation-counted so stale dispatches drop) — never a raw package var. Contexts that cross an async dispatch use `MallocCallbackContext` / `FreeCallbackContext`: C-heap, because Go's GC cannot see native retention.
- **Invalidate Mach ports before releasing them** — `CFRelease` alone leaks the kernel port (#1150, `eventtap_darwin.m`).
- **Go-created threads have no autorelease pool** — wrap traversal/drawing loops in `@autoreleasepool`.
- **Tests in this package need the main-loop harness**: `TestMain` calls `RunMainLoopForTesting(m.Run)` with a `runtime.LockOSThread()` `init`, or dispatched work deadlocks/times out (`runloop.go`).
- Naming: C entry points are `Neru*` PascalCase; one header + `_darwin.m` pair per subsystem. The untagged `doc.go` exists so `go vet` resolves the package off-macOS — leave it untagged.
