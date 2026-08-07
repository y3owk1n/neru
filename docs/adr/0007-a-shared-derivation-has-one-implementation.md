# A shared derivation has one implementation

**Status**: accepted

The same arithmetic is written once per platform backend in several places
here, and the copies are not kept honest by anything. Two live instances. The
subgrid breakpoints — where the nine cells of a 3×3 subgrid fall inside one
grid cell — exist four times: once in the domain, deciding where the cursor
lands (`internal/domain/grid/manager.go`), and once per overlay backend,
deciding where the cell is drawn (`internal/adapter/overlay/render/grid/overlay_darwin.go`,
`internal/adapter/overlay/linux/overlay_shared_cgo.go`,
`internal/adapter/overlay/windows/overlay.go`), with the 0.5 they round by
declared five times over. They agree today; nothing makes them, and
a disagreement is a cursor that lands somewhere other than the cell a person
selected. The font generic-alias parser existed three times and **did** diverge:
`sans_serif` resolved on macOS and Windows and fell through as a literal family
name on Linux, whitespace was trimmed on Linux only, and the Windows copy had a
`case "sans-serif"` its own normalisation had already made unreachable.

We decided the rule this repo had already been following twice without naming
it: **a value more than one layer computes from the same inputs has exactly
one implementation, and it lives at the lowest layer that has more than one
caller.** *Shared derivation* is the word for such a value; `CONTEXT.md`
carries it.

The part worth recording is the second half — *lowest layer with more than one
caller*, not "the domain". Nothing outside the three platform font resolvers
cares what "sans" means, so the parser this ADR ships with went to
`internal/adapter/platform/fontgeneric`, an untagged package among tagged
siblings, and each backend kept the one thing that is genuinely its own: which
concrete family its sans, serif and mono are. Both precedents say the same.
`internal/adapter/overlay/render/badge` is untagged, tested, and imported by the
Linux and Windows backends and both render style packages — badge sizing is not
a domain concept, it is a thing every renderer has to agree on.
`internal/domain/grid/subgrid_keys.go` is in the domain because the set a
subgrid is drawn with and the set it is navigated by have to be one set, and one
of those callers is the domain. Neither package was created by this ADR; they
are the precedent, not the exception.

## Considered options

- **Put every shared derivation in `internal/domain`.** The reading the
  hexagon suggests, and it is too strong. It would move fontconfig-flavoured
  alias vocabulary — "sans", "monospace", the fontconfig spelling of a font
  request — into the pure core to serve three adapters and nobody else, and the
  next contributor would reasonably read that as licence for the domain to
  learn about GDI. The domain earns a derivation when the domain is one of the
  callers, which is exactly why `SubgridKeys` is there and `fontgeneric` is not.
- **Leave the copies and pin them with a cross-package test.** Cheaper than any
  move, and it is the right answer in exactly one place (below). Rejected as the
  default: a test that asserts four implementations agree still ships four
  implementations to maintain, and it can only compare the cases somebody
  thought to write down. The font parser is the demonstration — the three
  copies were close enough to look deliberate and different enough to be a bug.
- **Extract pure-but-private logic for testability alone.** The Windows
  software rasterizer — ~310 lines of alpha blending and SDF rounded-box maths
  in `internal/adapter/platform/windows/overlay.go` — is pure Go, platform-free
  in everything but its build tag, and it stays where it is. It has one caller
  and no second implementation to disagree with, and `.github/workflows/ci.yml`
  already runs `test` on all three operating systems, so it is testable today;
  nobody has written the tests. Extracting it would buy a package boundary and a
  test suite the maintainer cannot run locally, which is how tests rot.
  Divergence is the problem this ADR addresses. Build tags are not.
- **Move the Objective-C copies into Go too, for completeness.** Rejected, and
  the exclusion is deliberate rather than pending — see below.

## Consequences

- **The rule stops at the language boundary.** Hint badge placement geometry is
  written in Objective-C (`hintRectForPlacement:` in
  `internal/adapter/platform/darwin/overlay_darwin.m`) and cannot call Go. Where
  the second implementation is in another language, Go cannot be the one
  implementation, and what this rule asks for there is a test pinning the copies
  together, not a deletion. Moving that geometry into Go would mean changing what
  crosses the cgo boundary — that is the overlay-port work, and doing it under
  this rule would be that redesign arriving without its design. No such pinning
  test exists yet: this ADR states what is owed, and the placement vocabulary
  written four times in Go against the enum in `internal/adapter/platform/darwin/overlay.h`
  is the first candidate.
- **"Lowest layer with more than one caller" is a judgement someone has to
  make, and it moves.** A derivation with one caller is not shared and should
  stay private; the second caller is what triggers the move, and the move is
  cheap precisely because it is a pure function. The failure mode this invites
  is a `common` package that collects everything a second caller ever touched —
  which is why `CONTEXT.md` lists *helper*, *util*, *common code* and *shared
  logic* as the words to avoid. A shared derivation is named for what it
  derives.
- **This ADR ships one instance and leaves two known ones standing.** The font
  parser is collapsed here (#1286). The subgrid breakpoints and the 3×3 written
  five times are filed separately (#1287), because they touch the drawing path on every
  backend and deserve their own diff. Naming the rule before finishing the work
  is the point: the next platform backend should not have to infer it from two
  packages that never explained themselves.
