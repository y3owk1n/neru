# A bridge's interface is its headers, not its Go API

**Status**: accepted

`internal/adapter/platform/darwin` compiles roughly 5,400 lines of Objective-C
that no `.go` file in the package includes: `overlay.h` and its 3,920-line
`overlay_darwin.m`, `monitor_select_overlay_darwin.m`, and the `vision`,
`textinput` and `keyfeed` pairs — plus a `systray.h` whose `.m` lives in
`internal/adapter/systray/darwin` and `#import`s back out. Fifteen packages
reach that C through eighteen relative `#include "../../platform/darwin/*.h"`
directives. The architecture review read this as a missing seam: a package
whose own contents nothing consumes.

It is the opposite. A cgo package publishes two interfaces, and for this one
the headers are the interface that matters — `overlay.h` alone declares 31
entry points. The Go API is a *link edge*: importing the package is what pulls
its compiled objects into the binary, whether or not any Go symbol is called.
Four packages state that edge outright — `app/modes/cursor_darwin.go:16`,
`adapter/textinput/textinput_darwin.go:29`, `adapter/vision/adapter_darwin.go:21`
and `adapter/keyfeed/keyfeed_darwin.go:15`, the third with the comment
`// ensure darwin CGo .m files are compiled`, and two more do it with a named
import (`accessibility/native/darwin`, `eventtap/darwin`). The remaining eight
include a header and reach the archive only transitively — and every one of
them is in the overlay render tree, all arriving through
`overlay/render/overlayutil/native`, a three-function forwarding shim they do
not name. Nothing records that this is the arrangement, and nothing checks it.

So we decided: **a bridge's interface is its headers; its Go import is what
links them, and every consumer states that import directly.** The consequence
that makes this worth recording is the one that reads as a mistake — a package
blank-importing something it never calls is deliberate, not dead code.

The failure this prevents is the expensive kind. Refactor `overlayutil` to stop
needing `native` — a plausible, well-intentioned change — and the whole overlay
render stack fails to link at once, on macOS only, with an undefined-symbol
error naming none of the eight packages that broke. That is the cost
`cgo_includes_test.go:61-68` already names for the
other half of this boundary: `go vet` does not see it, the cross-platform vet
does not see it, and it surfaces as a red CI job minutes later.

The rule generalises off macOS. `internal/adapter/platform/linux` has the same
shape, and *Bridge* in `CONTEXT.md` is written for both.

## Considered options

- **Export `cgoSlot[T]`.** `platform/darwin/AGENTS.md` mandates that every
  C→Go callback funnel through it, and the type is unexported, so the fifteen
  packages that include a bridge header cannot obey the contract they are held
  to. Rejected because the contract is what is wrong. `cgoSlot` tracks *one
  long-lived handler per subsystem*: the C side passes no context, the
  `//export`ed function finds the process-global slot, and the generation
  counter invalidates on `Set`. The out-of-package case is a different problem
  — `NeruResizeOverlayToActiveScreenWithCallback` (`overlay_darwin.m:2274`)
  hands its context through two nested `dispatch_async` hops, so each *in-flight
  operation* needs its own identity, allocated on the C heap because Go's GC
  cannot see it, plus a timeout because the callback may never arrive at all.
  `overlayutil.CallbackManager` is that, and `cgoSlot` structurally cannot be:
  it has no per-operation identity to hand C. Exporting it would offer eleven
  packages a tool that does not fit their case and invite exactly the wrong
  reuse. The contract is restated instead as two named idioms with stated
  domains. Note the fifteen figure is the packages that include a bridge
  header; six already import the bridge directly and could reach an exported
  `cgoSlot` today, which is the point — none of them wants it.
- **Give every `.m` a Go wrapper in its own package**, so the package consumes
  what it compiles and the review's observation stops being true. Rejected on
  the deletion test: it interposes a Go call layer between cgo and cgo, buying
  nothing but the appearance of a Go-shaped API, and it would put the overlay's
  31 entry points behind a second set of 31.
- **Rename or absorb `overlay/render/overlayutil/native`.** Considered on the
  reading that an import edge should not be filed as a utility package.
  Rejected on the facts: `overlayutil` carries no build tag, so under the One
  Rule it cannot import `platform/darwin` at all, and `native` is the
  build-tagged dispatch pair `internal/adapter/platform/AGENTS.md` prescribes
  for exactly that crossing. `native_other.go:7` already says so. It is the
  prescribed pattern correctly applied; it stays.
- **Rely on the linker.** The status quo. Rejected above: it is true only until
  the accident stops holding, and it reports the failure in the place least
  able to explain it.

## Consequences

- The eight transitive consumers gain a direct import of the bridge, and a
  guardrail in `internal/architecture/cgo_includes_test.go` pins it: a package
  that includes a bridge header and has no native source of its own must import
  that bridge package. It sits beside
  `TestCgoIncludes_NativeBridgePointsAtThePlatformPackages`, which pins where a
  header may be reached *from*; this pins that the objects behind it are
  actually linked.
- `internal/adapter/systray/darwin` is the exemption, and it is the case that
  shows the rule is about linkage rather than includes: its `.m` compiles in
  its own package, so it needs `systray.h` and not the archive, and it reaches
  `platform/darwin` not at all. The exemption is encoded, not special-cased.
- `platform/darwin/AGENTS.md` restates its callback contract as two idioms:
  callbacks registered inside the bridge funnel through `cgoSlot[T]`;
  callbacks that cross an async dispatch back into another package go through
  the callback registry. `cgoSlot` stays unexported. Left unpinned on purpose:
  what the old wording was reaching for — no raw package var holding a C
  callback target — is not statically checkable. Deciding which package vars
  are callback targets means following `//export` reachability, and every
  approximation of that either misses the case it exists for or fires on
  innocent vars. The contract carries this one, not a test.
- `callback_context.h` said its struct *"matches overlayutil.CallbackContext in
  Go"* — a layout pinned across the language boundary by a comment, with the two
  halves in packages that cannot see each other. It joins ADR 0007's inventory
  as a pin instead, reading the header through the `readNativeSource` entry
  point `native_constants_test.go` already publishes, and the comment now points
  at that pin rather than standing in for it. This is a missed case of a
  convention that already runs, not a new one. The one thing it settled that
  this decision had assumed: the Go half is *not* behind a build tag, so it is
  linked and reflected over rather than read as text, and only the header is a
  reading. ADR 0007's inventory carries the detail.
- *Bridge* moves into `CONTEXT.md` and carries this rule; the one-line
  definition under `AGENTS.md` Domain Concepts goes, because each fact has one
  home.
- Out of scope, deliberately. `overlayutil`'s callback registry is a
  per-component type over process-global state — `NewCallbackManager` is called
  twice in `factory_darwin.go` while the ID pool, the free list and the
  generation counter are package vars, and three packages call `SetComponent`
  on a shared manager. That is internal to `overlayutil`, does not touch this
  boundary, and belongs with the architecture review's candidate 8, which
  already carries the same shape for `adapter/systray/adapter.go:14`. It should
  not be "fixed" before someone establishes whether one global ID space is
  deliberate — a stale callback from any component being rejected by any
  manager is a defensible design.
- Also out of scope: the size of `overlay_darwin.m` and the eight `.m` files
  with no header. The first waits on ADR 0003's frame port, which decides what
  crosses `overlay.h` and would redo any split done now; the second is
  file-layout drift, and the answer to drift is a guardrail, not a decision.
