# A contract earns a guardrail when its breach is silent

**Status**: accepted

The seven `AGENTS.md` files carry 166 normative statements between them. Twenty-six
have a test in `internal/architecture` that fails when they are broken; seventy-nine
more could have one and do not; thirty-seven are judgement calls no test can reach.
An architecture review proposed turning each stated contract into a guardrail test.
Counted properly that is seventy-nine tests, in a suite where the assertion is the
cheap part — the house pattern is an assertion, a comment naming the real bug it
prevents, a companion test that fails when its exemption list stops being real, and
a floor assertion so the walk cannot silently match nothing. Adding all of them
would roughly double a package that is already 11,626 lines, and would say nothing
about which of the next contract's sentences deserve the same.

So we decided the criterion rather than the backlog: **a contract earns a guardrail
when breaking it is silent to CI** — the code compiles, `just lint` passes, every
test passes, and the breach ships anyway. And **where prose and code disagree, the
code is presumed right and the prose is restated, unless the breach is
user-visible** — in which case the contract was real and the code is the defect.

Both halves came out of grilling the review's own evidence. Five of the seven
contract violations it listed did not survive contact with the code. Cocoa's
coordinate flip is stated at two different scopes — `AGENTS.md` says "inside the
darwin adapter", which the code satisfies exactly, and
`internal/adapter/platform/AGENTS.md` says `accessibility_screen_darwin.m` "only",
which it has never satisfied — so the finding was a disagreement between two
sentences, not fifteen bad call sites. The permission stubs returning `true` are
returning what `internal/ports/system.go` specifies for a platform with no such
gate, and making them "loud" would have produced an unbounded re-prompt loop the
same file warns against. The Linux file slots the review found unused are in
constant use with the redundant OS token dropped, under an exemption printed four
lines below the table it quoted. The eight `.m` files with no header publish
sixty-three entry points and every one of them is declared in its subsystem's
header. In each case the cheap, correct change was to the sentence.

The exception is the one row that survived, and it is what the second half is for:
two independent compositor detectors disagree in a real session — a `sway` daemon
launched without `WAYLAND_DISPLAY` in its environment runs on X11 while `neru info`
reports `display_server: wayland`, and an ungated probe writes a KWin script to
disk on X11 sessions. That is user-visible, so the code moves, not the prose.

## Considered options

- **Give every stated contract a guardrail.** The review's proposal, and the reason
  this ADR exists. It has no stopping condition: the seven files are prose written
  to be read, and they mix binding rules with guidance in the same bullet — root
  `AGENTS.md` puts "Latency is the product" and "Prefer fixing the default" in a
  list a reader will otherwise take as mechanical. Testing all of it means either
  testing the untestable or making an unstated judgement call seventy-nine times.
- **Pin only what has already been broken.** Attractive, and rejected. It forbids
  pinning a convention at the moment it is introduced, which is when the pin is
  cheapest and the meaning is freshest, and ADR 0007's language-boundary pins are
  the standing counterexample — most of them hold copies that have never diverged,
  and their value is that they cannot.
- **Define "silent" as silent to the user.** The intuitive reading, and wrong here.
  A lock-order inversion in `internal/app/modes` hangs the application: deafening
  to whoever is using it, and completely invisible to every check that runs before
  it ships. Under this reading the four highest-consequence contracts in the repo
  would not qualify, which is the answer inverted.

## Consequences

- **The criterion disqualifies things, which is the point.** A `Neru*` entry point
  defined in a `.m` and declared in no header is not silent — clang has errored on
  a call to an undeclared function by default since version 16, and the repo passes
  no `-W` flags of its own, so the naive breach fails the build and names the
  symbol. What *is* silent is the same definition with its prototype written into
  the calling `.m`, into a cgo preamble (for which there is already precedent in
  `internal/adapter/platform/darwin/keymap.go` and `bridge.go`), or into some other
  subsystem's header, where `overlay.h`'s existing thirty-one entry points hide one
  more. The guardrail ADR 0009 promised is therefore justified, but its target is
  narrower than that ADR's wording: a non-`static` `Neru*` definition whose
  declaration lives anywhere but its own subsystem's header. A pin that only asked
  "is it declared somewhere" would be a no-op. Six such definitions exist today and
  all six are `static`, so the pin lands green. A seventh, `_NeruEnableCursorInBackground`
  in `internal/adapter/platform/darwin/overlay_darwin.m`, is `static` too but is
  outside the pin's reach for a different reason — its leading underscore puts it
  outside the `Neru*` PascalCase naming contract rather than making it file-local,
  which is why `internal/architecture/darwin_entry_point_headers_test.go` counts
  six.
- **Some contracts are half-covered, and the guardrail takes the other half.**
  `exhaustive` is enabled and fires on an *incomplete* switch over `domain.Mode`. A
  complete one lints clean — and a complete switch is what a careful contributor
  writes. So the guardrail for `internal/app/modes/AGENTS.md:18` targets the case
  the linter cannot see, and says so.
- **Restoring the compiler beats writing a guardrail.** The strongest instance
  found is `internal/app/modes/AGENTS.md:8`, four `ports.OverlayPort` methods the
  handler must never call. The file records that the compiler used to enforce this
  and that #1213 ended that arrangement; nothing replaced it. The handler now holds
  the full port, all four methods are on it, and the mocks are counter no-ops that
  can neither deadlock nor fail, so no existing test would catch the call even at
  runtime. Where a narrower consumer-side interface can restore the compile error,
  that is better than any test: it deletes the contract instead of pinning it, and
  a deleted contract cannot go stale. A guardrail is what to write when the type
  system cannot be made to carry the rule.

  Here it can. The port is twenty-seven methods — ADR 0003's eleven is stale, the
  mode surfaces landed on it in #1210–#1213 — and `internal/app/modes` calls
  thirteen, a set disjoint from the forbidden four and coherent on its own terms:
  frames, grid updates, hint search, keyboard capture, the flush and the active
  screen, with the indicator primitives left to the services that own them and
  lifecycle and theme left to the composition root. Nothing in the package asserts
  or switches on the field, so the narrower type costs the composition root
  nothing, and `MockOverlayPort` satisfies it structurally. The interface is
  declared at the consumer, which is already this repo's documented idiom —
  `internal/app/keybinding/hotkey.go` says so in as many words, and
  `internal/app/modes/monitor.go` does it inline. It is not named `*Port`: that
  suffix belongs to `internal/ports`.
- **A sentence may claim a guardrail only by naming it.** The failure of this kind
  the grill actually found was not an unenforced rule but a lying one:
  `AGENTS.md:78` stated that every platform stub gets a contract test when three
  existed against thirty-eight candidates, and `internal/config/AGENTS.md:5` said
  three of the four config links are guarded when it is two links by three tests. A
  sentence that claims enforcement it does not have is worse than one that claims
  nothing, because it stops the reader checking. So an `AGENTS.md` may assert that
  a test exists only by naming it, and a guardrail asserts that every such name
  resolves to a real test function — which several sentences already do. Neither of
  the two above could survive that rule: the test each implies has no name to give,
  because it does not exist. Both were restated to say what is actually checked
  (#1432, #1428), which is the outcome the rule is for.
- **The link points from the test to the prose, never back.** This was already the
  house style and is recorded here so it stays one: the failure message names the
  offending file, states the rule in the imperative, and cites the document that
  states it, as `layering_test.go` and `ports_test.go` do. No test parses `AGENTS.md`
  prose, and no `AGENTS.md` sentence is annotated with the name of its test except
  under the rule above. The index of what is executable is
  `internal/architecture/doc.go`, which `doc_inventory_test.go` already holds to the
  directory in both directions.
- **Lint where the rule is one node; test where it is a relationship.** Both are
  available and they are not interchangeable. `gocritic`'s `ruleguard` is enabled in
  `.golangci.yml` with no rules file, so it loads nothing and fails open; standing
  it up costs a module dependency that `govulncheck` can never produce a finding on
  and dependabot will bump weekly. Its DSL matches one syntactic node with a type
  filter, which fits "no `switch` whose tag is `domain.Mode`" and cannot express
  "every `Unlock` is deferred by the method that took the lock" — no cross-match
  state, no ancestry above file scope, no call graph, no CFG. Against that,
  `nolintlint` is enabled but unconfigured, so `require-explanation` is off and
  fifty-three of ninety-six suppressions in `internal/` carry no reason with CI
  green; a lint-enforced contract is silenced by one bare comment, where silencing a
  test is a reviewable diff. What is *not* an argument either way is
  `max-issues-per-linter`: truncation limits the report and never the exit code, so
  a branch violating a lint rule sixty times still fails CI. Nor is local coverage,
  quite — `just lint` sees only the host's build tags, but CI runs it natively on all
  three operating systems. The residual holes are the tags no runner sets, `race`
  and the non-host `GOARCH`, which lint never reaches and which
  `internal/architecture` covers on any leg because it reads the tree as text.
- **The One Rule's double enforcement gets its reason written down.** It is
  advertised in four places that `depguard` and
  `internal/architecture/dependency_boundary_test.go` both enforce it, and nowhere
  that this is deliberate or why. The reason is inferable from both comment blocks
  and worth stating: `depguard` matches directories, so its soundness rests on
  `TestPlatformPackagesTagEveryFile` — a Go test with no lint equivalent. The test
  is primary; the lint rule is the fast feedback.
