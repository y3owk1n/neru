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
  this rule would be that redesign arriving without its design. The first such
  pin landed with the placement vocabulary (#1289): the three values are
  declared once in Go, and `internal/architecture/hint_placement_vocabulary_test.go`
  holds that declaration, the macros in `internal/adapter/platform/darwin/overlay.h`
  and the `HintPlacement` enum to the same numbering. It pins *which placement
  each value means*, not where the badge is then drawn — `hintRectForPlacement:`
  remains a single Objective-C implementation with no Go counterpart to
  disagree with. The second such pin landed with the recursive-grid
  label-autohide rule (#1298), where the copy is a rule rather than a constant:
  `internal/architecture/label_autohide_rule_test.go` reads the Objective-C
  guard in `drawGridLabel:` into something it can run, and holds its answer to
  `recursivegrid.Style.ShowLabelIn` over the cases that separate them — the
  multiplier that disables autohide, the threshold itself, and each cell
  dimension one pixel under it. The third is the sub-key-preview autohide rule
  (#1323), which is the same shape one level down — every sub-cell of the
  mini-grid previewing the next depth must reach the multiplier times the
  preview font size — and which this list had not named until it was pinned.
  `internal/architecture/sub_key_preview_autohide_rule_test.go` reads *both*
  copies out of their sources and runs them against each other, rather than
  running one and reading the other: the Go copy
  (`shouldShowSubKeyPreview` in `internal/adapter/overlay/linux/cgo_helpers.go`)
  is behind `//go:build linux && cgo`, so a test on the macOS host cannot link
  it. Giving it an untagged home belongs to #1297, which converges the Windows
  backend — whose predicate measures a different rectangle today, a deliberate
  difference recorded in `docs/CROSS_PLATFORM.md` rather than drift, and one the
  pin asserts nothing about.
- **The exception is the half of this rule deletion cannot enforce, so what is
  pinned is inventoried here.** Four language-boundary copies are pinned as of
  #1323: the three named above, plus the synthetic key-up and modifier-toggle
  wire prefixes, which `internal/adapter/platform/darwin/eventtap_darwin.m` and
  `internal/adapter/platform/linux/overlay_wayland.c` format with `printf`
  because neither can import `internal/domain/keyvocab`, held to that package by
  `internal/architecture/keyvocab_wire_test.go`. That one predates this ADR and
  is the precedent the placement-vocabulary pin followed.

  That list is the complete set of *pinned* copies, and it is deliberately not a
  claim that no others exist — sweeping for them while writing the sub-key
  preview pin turned up more, the nearest one ten lines below it in the same
  pair of methods: which sub-cell of the preview mini-grid is left blank, the
  center one when both next-level dimensions are odd, decided in
  `drawSubKeyPreviewInCellRect:` and again in `drawSubKeyMiniGrid` in
  `internal/adapter/overlay/linux/overlay_shared_cgo.go`. It is not pinned, and
  naming it unpinned is the honest form of this inventory: everywhere else this
  ADR is enforced by there being nothing left to diverge from, and here it is
  enforced only by someone having written the pin. A copy that gains one adds
  itself to this paragraph in the same change.
- **"Lowest layer with more than one caller" is a judgement someone has to
  make, and it moves.** A derivation with one caller is not shared and should
  stay private; the second caller is what triggers the move, and the move is
  cheap precisely because it is a pure function. The failure mode this invites
  is a `common` package that collects everything a second caller ever touched —
  which is why `CONTEXT.md` lists *helper*, *util*, *common code* and *shared
  logic* as the words to avoid. A shared derivation is named for what it
  derives.
- **This ADR shipped one instance and left two known ones standing.** The font
  parser is collapsed here (#1286). The subgrid breakpoints and the 3×3 written
  five times were filed separately (#1287), because they touch the drawing path on every
  backend and deserved their own diff; they were collapsed in #1292, where the
  mode layer and all three overlays came to ask `internal/domain/grid` for the
  cells rather than divide the rectangle themselves. Naming the rule before finishing the work
  is the point: the next platform backend should not have to infer it from two
  packages that never explained themselves.
- **A near-miss earns a second name, not one implementation.** #1299 finished
  the centering sweep #1288 started, and what it turned on is a pixel. Two
  forms of "put this rectangle on that point" were in use: `center ± half`, and
  the `center - half, + size` the shared `badge.CenteredIn` already spoke. They
  agree on the near edge always and on the far edge only when the dimension is
  even, so collapsing them would have moved the monitor-select panel's right
  edge and a hint badge's by a pixel. Both are named now — `badge.CenteredOn`
  beside `badge.CenteredIn`, each doc comment pointing at the other — and the
  sites that hang a static rectangle on a point (hint badges, the hint boundary
  highlight, the monitor-select panel) call one of them on both Linux and
  Windows. The animated indicators are left out on purpose, and each says so
  where it is written: the virtual pointer floors its half at one pixel, and
  the mouse-action indicator rounds a float diameter on Linux and places a
  native window on Windows, so neither is this derivation wearing a different
  spelling. This rule asks for one implementation per derivation; two
  derivations that merely look alike are the case where the honest answer is to
  say so out loud, because a near-miss is exactly what a later reader
  "simplifies".
