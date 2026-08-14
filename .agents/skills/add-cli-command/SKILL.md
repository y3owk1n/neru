---
name: add-cli-command
description: "Add a Neru CLI command or flag: cobra command in internal/cli, IPC handler in internal/app/ipcctrl, service/mode work behind it, man pages, and docs/CLI.md. Use when adding or changing user-facing commands, subcommands, or flags. Not for config.toml options."
---

# Adding a CLI command to Neru

Neru is a daemon plus a thin CLI: almost every command just serializes an IPC
request to the running daemon over its per-user Unix socket or Windows named
pipe. A new command therefore touches three layers plus docs —
skipping the IPC layer and calling app code directly from `internal/cli` is
wrong even when it compiles.

## The chain

1. **Cobra command** in `internal/cli/<name>.go`, registered in an `init()`
   like its siblings. Mode-shaped commands (hints/grid/scroll variants) go
   through `BuildModeCommand(ModeConfig{...})` in `command_builders.go` instead
   of hand-rolling cobra — look at `hints.go` first. Long help text follows the
   house style: prose, then flag explanations, then an `Examples:` block.
2. **IPC transport** is already generic (`internal/adapter/ipc`); you only add
   an action string. Keep the wire shape in one place and reuse existing
   request helpers in `internal/cli/cliutil`.
3. **IPC handler** in `internal/app/ipcctrl`. Handlers register into
   `Controller.Handlers` via the sub-controller for their family
   (`lifecycle`, `modes`, `actions`, `info`, `overlay`, `scroll`, …) — extend
   the matching `RegisterHandlers`, don't bolt a case onto the controller.
4. **The actual work** lives in `internal/app/services/*` or a mode, never in
   the handler. Handlers parse, delegate, and shape the `ipc.Response`.
5. **Man pages**: regenerate with `just genman` (backed by `./cmd/genman`).
   Cobra metadata is the source — if the man page reads wrong, fix the
   command's `Short`/`Long`, not the output.
6. **Docs**: update `docs/CLI.md`. It is the single home for CLI reference
   facts.

## Mode flags are different

A flag on a mode command (`hints --action left_click`) is not registered by
hand. It is one entry in the descriptor table in `internal/domain/modecmd`,
carrying its own parse, its own render, and the modes that accept it — that
entry is what offers the flag on the command line, what the daemon and the
config validator read it with, and what writes its row in the reference.

After adding, removing or re-wording one, run `just genflagref` to rewrite the
generated region of `docs/CLI.md`. An architecture test
(`internal/architecture/mode_flag_contract_test.go`) fails while a descriptor
is unregistered or missing from that region, and while a mode command offers a
flag the table never declared.

## Tests

- Command wiring and flag parsing: follow the table-driven patterns in
  `internal/cli/cli_test.go` / `*_internal_test.go`.
- Handler behavior: `internal/app/ipcctrl/*_test.go` with port mocks from
  `internal/ports/mocks` — assert the response shape and that the right
  service call happened, not internals.
- Anything platform-gated must degrade with `derrors.CodeNotSupported`, and a
  contract test pins that (root `AGENTS.md`, "Hard rules").

## Verify

```bash
just build
./bin/neru <command> --help     # against a running daemon: ./bin/neru launch
just genman
```

Then the standard pre-commit gate from AGENTS.md.
