# The first hour must not lie

**Status**: accepted

An architecture review's last section audited a new contributor's first hour —
land, build, orient, change, verify, ship — and produced two lists. One was
things the repo says that are not true. The other was things large Go projects
have that this one does not: an `examples/` directory, golden-file tests, a
`just dev` hot-reload loop, a `nix develop` devShell, commitlint, a path-based
auto-labeler. The second list is longer, easier to act on, and worth nothing.

So we decided the criterion rather than the checklist: **the first hour must not
lie.** A newcomer's first hour is a sequence of promises — the README's, `just
--list`'s, the PR template's, Quick Start's — and every one of them is consumed
before the contributor has any evidence with which to weigh it. Absence is
discovered for free and routed around: nobody clones a repo, finds no
`examples/` directory, and loses an evening. A false statement is discovered
only by acting on it, and it spends trust that has not yet been earned. So a
promise the repo already makes and does not keep outranks any promise it has
not made.

The lies were real and none of them were features. `just --list` — the command
`DEVELOPMENT.md` names under *Verify* as the way to see what the project can do
— was wrong for twenty-six of fifty-two recipes, because `just` takes only the
last contiguous comment line before a recipe and this justfile writes rationale
paragraphs there. `build` advertised itself as "*X11/Wayland native backends).
Windows currently builds with CGO disabled.*"; `list-foundation-packages` as
"*check could see.*"; `test` as a fragment opening on an unmatched backtick.
Four documents called `just ci` "exactly what CI runs" when it runs the right
six recipes on one host out of the three CI gates on. Two documents linked
newcomers to a `good first issue` query returning nothing, against a label that
exists and eight open issues carrying none of it. `DEVELOPMENT.md` said Devbox
"provides every tool pre-configured" when on Linux it does not pull the `-dev`
outputs a CGO build needs. And a green `just test-unit` printed 9,782 lines to
say `ok`.

The criterion also disqualified five of the review's own twenty claims, which
is the other half of its value. `lint-cross` and `test-linux` were said to
appear in zero markdown files; they appear in three, and `docs/CROSS_PLATFORM.md`
now presents the hand-rolled `CGO_ENABLED=0 GOOS=linux golangci-lint run` with a
caveat paragraph and names `just lint-cross` as the real check. The Linux build
dependencies were said to live only in `ci.yml`; they have a `## Build
dependencies` section in `LINUX_SETUP.md` with apt, dnf and pacman lists. There
was said to be no human first-PR walkthrough; `CONTRIBUTING.md` has a
seven-step one that already carries the Accessibility-permission warning. The
two best onboarding documents were said to be reachable only through a section
headed *AI-Assisted Contributions*; step 3 of that walkthrough links `AGENTS.md`
directly. And `nix develop` was said to fail — it does, and no document in the
repo has ever suggested running it. `flake.nix` is a distribution flake:
`packages`, `overlays`, and the darwin, NixOS and home-manager modules. It is
for people installing Neru. A thing nobody was promised cannot have been
promised falsely.

## Considered options

- **Work the OSS-parity list.** Each item is a defensible afternoon and the
  aggregate is a repo that looks like other repos. It has no stopping condition
  and no argument for any individual entry — the review's own list included
  `examples/`, which this repo has under the name `configs/`, five files of it,
  linked from the README. A checklist assembled by comparison cannot tell you
  that.
- **Fix everything the audit found.** Attractive, and the reason this ADR
  exists: a quarter of the findings were already false five days after they were
  written, and the criterion is what tells you which quarter without re-auditing.
  Fixing all twenty would have re-broken two things #1438 had just repaired.
- **Define the criterion as "anything that costs a newcomer time."** Too wide to
  decide anything. A missing hot-reload loop costs a newcomer time on every
  iteration forever; it is still not a lie, and treating the two as one category
  is how the parity list gets back in.

## Consequences

- **Recipe documentation is declared, not inferred from layout.** The twenty-six
  wrong entries were not typos — every comment block was accurate prose about
  the recipe below it. What was wrong was the mechanism: which line `just`
  happens to read is a function of where someone put a blank line, so the listed
  text drifts whenever the rationale is edited, silently, in a command no test
  runs. `build` had already tried the layout fix — a blank line mid-block — and
  still rendered wrong, because its summary sat on the far side of the blank.
  Recipes therefore carry an explicit `[doc('…')]` attribute (`just 1.54.0`,
  pinned in `devbox.json`) and the prose comments stay exactly where they are.
  This is the *Option* entry in `CONTEXT.md` applied to the build system: one
  declaration, and the rendered list is a projection of it.
- **It earns a guardrail, by ADR 0011's test.** The justfile parses, every
  recipe runs, `just lint` is silent and every test passes with the list wrong —
  which is how it stayed wrong for months. The guardrail reads `justfile` as
  text and fails on a public recipe with no `[doc('…')]`, in the manner ADR 0011
  prescribes for `internal/architecture`: no shelling out to `just`, so it holds
  on every leg regardless of what is installed.
- **`-v` comes off the unit recipes and stays on the integration ones.** The
  intuitive split — quiet locally, verbose in CI — is not available: `test-ci`
  is a composition of `test-foundation test-unit test-race-unit
  test-integration-ci`, not its own `go test` invocation, so there is no seam
  between the two audiences. Given that, the honest question is which suites
  gain anything from `-v`, and the answer is not the unit ones: `go test` prints
  a failing test's name and full output without it, so on a green run `-v` is
  100% of the 9,782 lines and on a red one it is noise around the part that
  matters. The integration recipes keep it — they run `-p 1` serialized, skip
  heavily on headless runners, and have hung; there the per-test line is
  progress, and it is hundreds of lines rather than thousands. This is a
  deliberate asymmetry, not an oversight.
- **`just ci` gains `check-cross` and keeps its promise modest.** A CGO-off
  type-check of the Linux and Windows builds costs seconds, needs no Docker, and
  catches the most common cross-platform failure — a build break in a tagged
  file. `lint-cross` and `test-linux` are deliberately *not* added: they require
  a running Docker daemon, and making the documented pre-push gate fail on
  missing infrastructure is precisely the first-hour failure this ADR is about.
  The prose in `AGENTS.md`, `CONTRIBUTING.md`, the PR template and the recipe
  comment already says "on your host only, where CI runs them on three"; that
  wording is the contract, and `check-cross` narrows the gap it admits to
  rather than closing it.
- **A link to an empty query is deleted, not relabelled.** Two documents pointed
  at a `good first issue` filter with nothing behind it. Curating starter issues
  is real editorial work with no deadline; deleting a link that lies is two
  lines. The tempting middle — labelling existing issues to make the link true —
  is worse than either, because most of what remains in the backlog is
  architectural, and a hard issue wearing that label is a lie the newcomer only
  discovers after committing the evening. `CONTRIBUTING.md`'s *Good First
  Contributions* section keeps its other half, which names real starter areas and
  points at `CROSS_PLATFORM.md`'s *Contributing safely*.
- **Conventional commits stay unenforced, and the PR template stops pointing at
  the wrong artifact.** Release Please ships the subject verbatim, so a
  malformed subject is user-visible — but the maintainer squash-merges, and the
  squash *title* is what Release Please reads, not the commit messages on the
  branch. The human who types that title is the gate. A PR-title action would
  re-check what a person checks anyway, at the cost of a CI job and a
  third-party dependency; commitlint would additionally fire on WIP commits that
  get squashed away. The template's "Commit messages follow conventional
  commits" is restated to name the title.
- **Declined, so they stop recurring:** `examples/` (exists as `configs/`),
  golden-file tests (no current test wants one; a pattern adopted for parity is
  a pattern nobody follows), a `just dev` hot-reload loop (`DEVELOPMENT.md`
  already explains why the daemon+CLI shape makes it awkward, and offers the
  `entr` snippet), a `nix develop` devShell (`devbox shell` is the documented
  path and `flake.nix` is for distribution — a second dev environment is a
  second thing to keep true), commitlint, the path-based auto-labeler (41 labels,
  applied by one maintainer), and a README table of contents (GitHub renders an
  outline for every markdown file; a hand-kept copy is a second home for the
  heading list).
- **No new word in `CONTEXT.md`.** Every prior grill in this series produced one,
  and this one deliberately does not. The glossary names Neru's domain — hotkeys,
  frames, bridges, vocabularies — plus two words about the repo's own practice,
  *Contract* and *Guardrail*, which earn their place because guardrail failure
  messages cite them. "The first hour" is a policy about documents. Putting it in
  the glossary would make the glossary a place where policies live, which is the
  drift the file's own header warns against.
