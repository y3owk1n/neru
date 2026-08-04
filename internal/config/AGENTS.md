# Config — the option chain

`~/.config/neru/config.toml`, hot-reloadable via `loader/` (load/watch/persist/set-field). Adding or changing an option touches every link; none are optional:

1. Struct field in `config.go` (`toml:"snake_case"`, nested in its feature's sub-struct).
2. Shared default in `newDefaultConfig()` (`config_defaults.go`) — every field gets an explicit default.
3. Platform overrides in `applyPlatformDefaults()` in `config_<os>.go`; both layers reached only via `DefaultConfig()` (`config_platform.go`).
4. Validation in the matching `validators_*.go` `Validate*` method — reject invalid values loudly, never silently clamp.
5. `configs/` examples (embedded and tested — stale examples break the build) and `docs/CONFIGURATION.md` (the single home for config reference facts).

The `add-config-option` skill walks this end to end, plus tests. Fast check: `just test-foundation`.
