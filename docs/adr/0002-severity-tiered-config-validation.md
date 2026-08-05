# Mode commands in bindings are validated at two severities

**Status**: accepted

Once the config validator reads mode commands (see
[ADR 0001](0001-one-grammar-for-mode-commands.md)), it can reject bindings that
load cleanly today. That is not free: when `Validate()` fails, `refuse` in
`internal/config/loader/load.go` replaces the whole configuration with
`config.DefaultConfig()` — not just the offending binding. A single mistyped
flag would reset every binding, theme and setting the user had. We decided to
split the new checks by whether the binding already works: an unknown flag is a
load error, and a flag the named mode does not accept, or a flag whose
co-dependency is unmet, is a warning.

The line is drawn at what the user loses. A binding with an unknown flag is
already dead — the argument is taken as a positional action and refused at press
time, so the mode never activates — and `internal/config/validators_hotkeys.go`
already fails the whole load for an unknown *command* in a binding. Treating an
unknown flag the same way extends an existing rule rather than inventing a
hazard. The other two classes describe bindings that work today minus one flag;
demoting a working configuration to defaults over them would be a worse outcome
than the bug being reported.

## Consequences

- `LoadResult` gains a warnings channel, and `neru config validate` prints
  warnings instead of only ever saying "Configuration is valid". Without it a
  warning is a `zap` line the CLI never shows, which would make the tier
  meaningless for the command people run to check their config.
- Upgrading with an unknown flag in a binding resets the configuration to
  defaults. This is accepted as consistent with the existing unknown-command
  behaviour, not as desirable. Narrowing the blast radius so an invalid binding
  is dropped rather than failing the whole load would fix both, and is
  deliberately left out of scope here — it changes how every config error
  behaves, not just this one.
