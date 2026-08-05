# Config — the option chain

`~/.config/neru/config.toml`, hot-reloadable via `loader/` (load/watch/persist/set-field). Adding or changing an option touches every link; none are optional:

1. Struct field in `config.go` (`toml:"snake_case"`, nested in its feature's sub-struct).
2. Shared default in `newDefaultConfig()` (`config_defaults.go`) — every field gets an explicit default.
3. Platform overrides in `applyPlatformDefaults()` in `config_<os>.go`; both layers reached only via `DefaultConfig()` (`config_platform.go`).
4. Validation in the matching `validators_*.go` `Validate*` method — reject invalid values loudly, never silently clamp.
5. `configs/` examples (embedded and tested — stale examples break the build) and `docs/CONFIGURATION.md` (the single home for config reference facts).

The `add-config-option` skill walks this end to end, plus tests. Fast check: `just test-foundation`.

**Refusing costs the whole file.** A failed `Validate()` replaces the entire configuration with the defaults, not the offending line, so a check that would refuse a setting the user is living with belongs in the warnings channel instead: `ValidateWithWarnings` collects them, they ride out on `LoadResult`, and `neru config validate` prints them. The line between the two, and why it sits where it does, is ADR 0002 (`docs/adr/0002-severity-tiered-config-validation.md`). Everything else still refuses loudly — never silently clamp.
