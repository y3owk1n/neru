# Config options get guardrails, not generation

**Status**: accepted

Adding one config option touches nine files before any consumer sees it, and
`held_repeat.accel_*` (`ef9e12ca`) is the worked example. That compares with two
guides that promise five links and seven steps respectively and disagree with
each other. The obvious reading is that the chain is too long and should be
generated, which means declaring the option once and emitting the default,
the example line and the documentation row. This repo has already done that
once, for mode
flags. `domain/modecmd` holds the descriptor table, `internal/flagref` renders
it, `just genflagref` writes it into a marked region of `docs/reference/cli.md`, and a
guardrail test fails when the page is stale.

We decided **not** to do it for config options, and to spend the same effort
making a forgotten link fail loudly instead. The four projections stay
hand-written, and four tests in `internal/architecture` make it impossible to
forget one quietly.

The part worth recording is the measurement, because it is the whole argument
and it is not what the file count suggests. Reflecting over the schema and
diffing it against `configs/default-config.toml` gives 350 leaf paths, 188
written and 165 absent, and **161 of those 165 absences are correct**. Of those,
100 are `config.Color` leaves that `ResolveThemeDefaults()` fills, 54 are
`app_configs[]` sub-fields of a repeated table nobody ships entries for, and 7
are collections that are empty by default. Four options are missing. The
validator ladder tells the same story: 23 `Validate*` methods, 21 wired into
`ValidateWithWarnings`, and the two absent are `Validate` and
`ValidateWithWarnings` themselves. Nothing is unwired.

So the chain is long but it is not, today, *drifting*. Generation would spend a
rewrite of `config.go` and `config_defaults.go`, 1,545 lines, to buy back
typing, and would buy it by taking on a code generator on the path that every
user's configuration flows through. Guardrails buy the property that
failed. **You cannot forget a link without CI saying so.** If the guardrails
turn out to fire often, that is the evidence for generation, and it is evidence
we do not have yet.

## Considered options

- **Generate the projections from one declaration**, `modecmd`/`flagref`-style.
  The strongest option, and the one this ADR exists to reject on evidence
  rather than on taste. Rejected for now on the 161-of-165 measurement. The
  drift it prevents is four cosmetic gaps in an example file, and the four
  links it can derive are only half the chain. The platform override, the
  validator, the derivation helper and the cross-field rule are real code that
  has to be written whether or not the schema is declarative. The honest
  ceiling is nine files to five, not nine to one, and the floor is a generator
  on the config path.
- **Drive defaults from struct tags** (`default:"120"`), the cheapest shape of
  generation. Rejected, because it only reaches scalars, and the defaults that hurt
  are the ones that are not: `ThemePalette`, hotkey maps, the launcher
  bindings. Every value would also become a string parsed at init, trading a compile
  error for a runtime one on the path where a bad config already costs the user
  their whole file (ADR 0002).
- **Restructure the validator ladder into an ordered table**, so plain
  reflection can check that every validator is wired. Rejected *for this work*,
  not on merit. With the ladder measured correct, it would be the only
  production change in an otherwise tests-only sequence, and an AST pass gets
  the same guarantee without touching the path. It stays available as a
  readability change, and it had one thing going for it that the test does not
  have: an ordering the ladder relies on, stated only in a comment. **Amended
  (#1270):** that comment named an ordering the ladder did not have. Checking
  macro definitions before macro calls is internal to `ValidateMacros`, not a
  claim on its position, and the position it did claim, last, stopped being
  true when `ValidateModeCommands` was appended below it in #1201. What the
  ladder does rely on is that its two whole-configuration walks close it, and
  `TestTheBindingWalksCloseTheLadder` now declares that, so the ordered table
  has nothing left over the AST pass.
- **Generate `docs/reference/configuration.md` rows too**, the third projection. Rejected
  on measurement again. The reference documents all four of the options missing
  from the example TOML, so its drift is currently zero, and matching a Go
  field path to a prose table row is the fuzziest of the three matches. If
  generation ever happens, the row becomes an emitted artifact and the matching
  problem disappears with it.
- **Do nothing and fix the guides.** Rejected because the guides were the
  problem. Both claimed "stale examples break the build" and no test compared
  `DefaultConfig()` to the TOML, so the sentence a contributor trusted was
  false. A guide that states a contract nothing enforces is worse than no
  guide, because it is believed.

## Consequences

- **Two of the four tests find nothing on the day they land, and that is the
  bargain.** The validator-wiring check finds zero, and the example checks find
  four cosmetic gaps. Only the explicit-default check has a confirmed defect
  behind it. `Grid.RowLabels` and `Grid.ColLabels` are declared in `config.go`
  and appear nowhere else in `internal/config`, with their real meaning
  implemented in a consumer (`app/components/types.go:51-57` reads `""` as
  "fall back to `characters`"). A guardrail whose value is preventive is the
  kind that gets deleted in a cleanup two years from now. This ADR is the
  answer to "what was this for".
- **The exemption rule is the design, and it will need upkeep.** Two structural
  rules exempt 161 of 165 absences: leaves under a `config.Color`, and
  empty-by-default collections. A future option shaped like a `Color`
  inherits the exemption without anyone deciding it should. The named allowlist
  ships empty on purpose so that the first entry is a decision someone writes a
  reason for, rather than a line appended to a list that was never empty.
- **`configs/` is a working area, and the tests respect that.** `author-config.toml`
  and `test.toml` are the maintainer's, not project artifacts.
  `embedded_config_test.go` already listed the four shipped examples rather than
  globbing, for that reason. The new tests import the same list rather than
  keeping a second copy, since a second copy is the failure mode they exist to
  prevent.
- **Only `default-config.toml` is embedded.** `configs/embed.go` reaches one
  file. The other three shipped examples are read from disk in tests. Any claim
  that the examples are "embedded and tested" has to say which, and
  `neru-add-config-option/SKILL.md` currently does not.
- **A cross-field rule now has one home.** ADR 0002 built the warnings tier so a
  setting that loads but will not do what it says reaches
  `neru config validate` instead of costing the user their file. The first
  cross-field rule added afterwards, `accel_enabled` without `enabled`
  (`loader/load.go:71-79`), went to `logger.Warn` and reaches nothing, which
  `warnings.go:16-19` names as the failure that would make the whole tier
  meaningless. The rule this ADR pins: a cross-field check a user could act on
  is reported through `LoadResult`.
- **Amended (#1453).** The chain is five links now, not four. An option also
  declares which platforms writing it does anything on, and
  `TestEveryConfigOptionDeclaresItsPlatformSupport` is a fifth guardrail on the
  same principle, that a column nobody wrote cannot be told from a forgotten one.
  The fifth link is also the first projection this ADR's reasoning does not
  cover. Its documentation rows *are* generated, into
  `docs/reference/platform-support.md`, because they were never a hand-written reference
  row per option but one table of the words that are not supported everywhere,
  where the match this ADR called fuzzy is exact
  (`docs/adr/0013-parity-is-measured-in-words-not-subsystems.md`). Nothing else
  here moves. The default, the example and the reference row stay hand-written.
- **Reversing this is cheap, which is why it is worth writing down.** Nothing
  here forecloses generation. The guardrails become the generator's acceptance
  tests if it is ever built. What would be expensive is the opposite order,
  generating first and discovering afterwards that the drift being prevented was
  four lines in an example file.
