---
name: add-config-option
description: "Add or change a Neru config.toml option end to end: struct field, shared and platform defaults, validation, examples, docs, and hot-reload behavior. Use whenever a change adds, renames, or removes a key in the TOML config schema. Not for theme palette entries or CLI flags."
---

# Adding a config option to Neru

Config changes fail in review when one link of the chain is missing — a default
that exists on macOS but not Linux, a validator that never runs, or an example
file that still shows the old key. Walk every step; none are optional.

## The chain

1. **Schema.** Add the struct field in `internal/config/config.go` with a
   `toml:"snake_case_name"` tag matching the surrounding section. Follow the
   existing nesting — options live in their feature's sub-struct, not at root.
2. **Shared default.** Set it in `newDefaultConfig()` in
   `internal/config/config_defaults.go`. Every field gets an explicit default;
   zero values by omission are how drift starts.
3. **Platform overrides.** If any platform needs a different default, set it in
   `applyPlatformDefaults()` in the matching `config_<os>.go`
   (`config_darwin.go`, `config_linux.go`, `config_windows.go`,
   `config_other.go`). Both layers are reached via `DefaultConfig()` in
   `config_platform.go` — never call `newDefaultConfig()` directly from
   feature code.
4. **Validation.** Add checks to the relevant `Validate*` method in
   `internal/config/validators_*.go` (hints, grid, style, hotkeys, macros,
   pointer, appconfig, …). Invalid values must fail validation with a clear
   message, not get silently clamped. New enum-ish strings get an allowlist.
5. **Hot reload.** Config is hot-reloadable via `internal/config/loader`
   (load/watch/persist/set-field). If the option should be settable at runtime
   with `neru config set`, confirm the field path resolves through the loader's
   set-field lookup; if it intentionally requires a restart, say so in the docs.
6. **Examples.** Update `configs/default-config.toml` and any other example in
   `configs/` that shows the affected section (`author-config.toml`,
   `hints-only-config.toml`, …). These are embedded and tested
   (`embedded_config_test.go`), so stale examples break the build — good.
7. **Docs.** Update `docs/CONFIGURATION.md`. That file is the single home for
   config reference facts; do not also describe the option in ARCHITECTURE or
   README.

## Tests

- Table-driven validator tests next to the validator you touched
  (`validators_*_test.go`), covering the default, a valid custom value, and at
  least one rejected value.
- If the default differs per platform, pin it in `config_defaults_test.go` or
  the platform-tagged config test.

## Verify

```bash
just test-foundation    # fast: config + action + ports slice
go test ./internal/config/...
```

Then the standard pre-commit gate from AGENTS.md.
