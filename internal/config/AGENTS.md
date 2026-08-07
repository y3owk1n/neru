# Config — the option chain

`~/.config/neru/config.toml`, hot-reloadable via `loader/` (load/watch/persist/set-field).

Adding or changing an option touches **four links every time, and up to four more when the option needs them**. The split is the part worth knowing. The universal four are projections of one declaration, and three of them have a guardrail test in `internal/architecture` that fails when you skip them. The conditional four are real code that has to be written either way — which is why generating the chain would buy back less than the file count suggests (`docs/adr/0006-config-options-get-guardrails-not-generation.md`).

## Universal — every option

1. **Struct field** in `config.go` (`toml:"snake_case"`, nested in its feature's sub-struct). The source of truth the rest of the chain is checked against.
2. **Shared default** in `newDefaultConfig()` (`config_defaults.go`). Every field is assigned by name, even when the value assigned is the zero value — a zero nobody wrote cannot be told from a forgotten one. Miss it and `TestEverySchemaFieldHasAnExplicitDefault` fails.
3. **Example line** in `configs/default-config.toml`, commented out when the option has no default. Miss it and `TestConfigOptionsAppearInTheDefaultExample` fails. Misspell a key in any of the four shipped examples and `TestShippedExamplesWriteOnlySchemaKeys` fails — the decoder drops an unknown key in silence, so a typo there is a dead line that looks like it works.
4. **Reference row** in `docs/CONFIGURATION.md`, the single home for config reference facts. **No test enforces this one** — the other three are checked, this one is not: matching a Go field to a prose row is the fuzziest match of the four, and ADR 0006 declined it. It is on you and the reviewer.

## Conditional — only when the option needs it

- **Platform override** in `applyPlatformDefaults()` in `config_<os>.go`; both layers are reached only via `DefaultConfig()` (`config_platform.go`), never `newDefaultConfig()` directly. No guardrail sees a per-platform difference — pin it in a test by hand.
- **Validator**, in the matching `validators_*.go` `Validate*` method on `*Config` — reject invalid values loudly, never silently clamp. Nothing forces an option to have one, but a validator that exists and is not called by `ValidateWithWarnings` fails `TestEveryConfigValidatorRunsInTheLadder`, because a validator nobody calls never runs.
- **Derivation helper**, when the value the daemon runs on is not the value the user writes (`grid_labels.go`, `held_repeat.go`, `ResolveThemeDefaults()` in `theme_palette.go`). It runs once the whole configuration is assembled — file, override file and platform layer all in — so the declared default stays the value a user would have typed, not the resolved one.
- **Cross-field rule**, when two settings only make sense together. If a user could act on it, it rides out on `LoadResult.Warnings` and reaches `neru config validate`; `logger.Warn` reaches nobody (ADR 0002).

`held_repeat.accel_*` needed all four conditional links, which is how one option came to touch nine files.

**What the guardrails do not reach.** Links 2 and 3 exempt three shapes, and an option with one of them can skip both with nothing failing: the `light`/`dark` leaves under a `Color` (`ResolveThemeDefaults()` fills those), a collection that ships empty, and every field of a repeated table nobody ships entries for (`app_configs`, `layers`). The exemptions are structural and recomputed from the live defaults each run, and the named allowlist beside each is empty — adding to it is a decision that wants a written reason (ADR 0006). If your option is one of those shapes, the reviewer is the only check.

The `add-config-option` skill walks this end to end, plus tests. Fast check: `just test-foundation`.

**Refusing costs the whole file.** A failed `Validate()` replaces the entire configuration with the defaults, not the offending line, so a check that would refuse a setting the user is living with belongs in the warnings channel instead: `ValidateWithWarnings` collects them, they ride out on `LoadResult`, and `neru config validate` prints them. The line between the two, and why it sits where it does, is ADR 0002 (`docs/adr/0002-severity-tiered-config-validation.md`). Everything else still refuses loudly — never silently clamp.
