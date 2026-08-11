---
name: add-config-option
description: "Add or change a Neru config.toml option end to end: struct field, shared and platform defaults, validation, examples, docs, and hot-reload behavior. Use whenever a change adds, renames, or removes a key in the TOML config schema. Not for theme palette entries or CLI flags."
---

# Adding a config option to Neru

Config changes fail in review when one link of the chain is missing — a default
that exists on macOS but not Linux, a validator that never runs, or an example
file that still shows the old key. `internal/config/AGENTS.md` states the
contract and which test fails per link; this is the order to do it in.

Five steps are universal. Then check the conditional four — the ones that turn
a one-line option into a ten-file one, and that no guide used to name.

## The universal five

1. **Schema.** Add the struct field in `internal/config/config.go` with a
   `toml:"snake_case_name"` tag matching the surrounding section. Follow the
   existing nesting — options live in their feature's sub-struct, not at root.
2. **Shared default.** Set it in `newDefaultConfig()` in
   `internal/config/config_defaults.go`. Assign the field by name even when the
   value you assign is the zero value, and say in a comment what the zero
   means: a zero nobody wrote cannot be told from a forgotten one.
   `TestEverySchemaFieldHasAnExplicitDefault` fails on an omission.
3. **Example.** Add the line to `configs/default-config.toml`, which is the file
   a user copies. Comment it out if the option has no default — an uncommented
   empty value asserts a default that does not exist.
   `TestConfigOptionsAppearInTheDefaultExample` fails when the line is missing.
   Update the mode-specific examples (`grid-only-config.toml`,
   `hints-only-config.toml`, `recursive-grid-only-config.toml`) only if they
   show the affected section; they are deliberately partial, and only their
   *keys* are checked, by `TestShippedExamplesWriteOnlySchemaKeys`. Those four
   are the whole shipped set — `configs/embed.go` lists them, and anything else
   in `configs/` is a working file rather than a project artifact. Only
   `default-config.toml` is embedded; the rest are read from disk by the tests
   that check them.
4. **Docs.** Add the row to `docs/CONFIGURATION.md`. That file is the single
   home for config reference facts; do not also describe the option in
   ARCHITECTURE or README. **No test catches a missing row** — this is the one
   universal link a reviewer has to check by eye.
5. **Platform column.** Add the option's path to `PlatformSupport()` in
   `internal/config/platform_support.go`, saying which of macOS, Linux and
   Windows writing it actually does something on. Say so even when the answer
   is all three: a column nobody wrote cannot be told from a forgotten one, and
   `TestEveryConfigOptionDeclaresItsPlatformSupport` fails on an option that is
   neither. A narrower column needs a note saying why, in the words the user
   reads — it reaches the load-time warning, the `neru doctor`
   `platform_support` row and the published table, which you regenerate with
   `just gensupportref`. A `config.Color` is declared at the field, not at its
   `light` and `dark` leaves
   (`docs/adr/0013-parity-is-measured-in-words-not-subsystems.md`).

Steps 2 and 3 exempt three shapes, and an option with one of them passes both
tests whether or not you do the work: the `light`/`dark` leaves under a `Color`,
a collection that ships empty, and any field of a repeated table nobody ships
entries for (`app_configs`, `layers`). Do the work anyway — see
`internal/config/AGENTS.md`.

## The conditional four

Each is skippable, and each is a whole file when it is not.

- **Platform override.** If a platform needs a different default, set it in
  `applyPlatformDefaults()` in the matching `config_<os>.go`
  (`config_darwin.go`, `config_linux.go`, `config_windows.go`,
  `config_other.go`). Both layers are reached via `DefaultConfig()` in
  `config_platform.go` — never call `newDefaultConfig()` directly from feature
  code. Nothing checks this; pin the difference in a test.
- **Validator.** Add checks to the relevant `Validate*` method on `*Config` in
  `internal/config/validators_*.go` (hints, grid, style, hotkeys, macros,
  pointer, appconfig, …). Invalid values must fail validation with a clear
  message, not get silently clamped; new enum-ish strings get an allowlist. A
  brand-new `Validate*` method must be called from `ValidateWithWarnings` in
  `validate.go`, or `TestEveryConfigValidatorRunsInTheLadder` fails.
- **Derivation helper.** If the value the daemon runs on is not the value the
  user writes, resolve it once after the configuration is assembled rather than
  in each consumer — `internal/config/grid_labels.go` and `held_repeat.go` are
  the worked examples. The declared default stays what a user would type.
- **Cross-field rule.** If the option only makes sense alongside another, and a
  user could act on the mismatch, collect it into `Warnings` so it rides out on
  `LoadResult` and reaches `neru config validate`. `logger.Warn` reaches nobody
  (ADR 0002, `docs/adr/0002-severity-tiered-config-validation.md`).

## Hot reload

Config is hot-reloadable via `internal/config/loader` (load/watch/persist/
set-field). If the option should be settable at runtime with `neru config set`,
confirm the field path resolves through the loader's set-field lookup; if it
intentionally requires a restart, say so in the docs.

## Tests

- Table-driven validator tests next to the validator you touched
  (`validators_*_test.go`), covering the default, a valid custom value, and at
  least one rejected value.
- If the default differs per platform, pin it in `config_defaults_test.go` or
  the platform-tagged config test.

## Verify

```bash
just test-foundation    # the fast cross-platform-safe slice
```

That one recipe now runs every package this skill's guardrails live in — the
config package, its loader, and `internal/architecture`.

Then the standard pre-commit gate from AGENTS.md.
