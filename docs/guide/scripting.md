# Scripting

Every hotkey step is also a shell command, so anything a binding does, a
script, a launcher or another hotkey daemon can do too. Each `neru` command
prints its reply, writes errors to stderr, and exits non-zero on failure.
Commands are in the [CLI reference](../reference/cli.md), and the
`neru status --json` fields under
[`neru status`](../reference/cli.md#neru-status).

## Pause and resume Neru

```bash
if [ "$(neru status --json | jq -r .enabled)" = true ]; then neru stop; else neru start; fi
```

## Check whether the daemon is running

```bash
neru status &>/dev/null && echo "Running" || echo "Not running"
```

## Drive Neru from another hotkey daemon

```text
# ~/.config/skhd/skhdrc
ctrl - f : neru hints
ctrl - g : neru grid
ctrl - r : neru hints --action right_click
```

## Run several steps as one unit

Use [`neru run`](../reference/cli.md#neru-run) instead of chaining `neru`
calls with `&&`. It sends the whole sequence in one request, under hotkey
binding rules.

To leave every global hotkey to the other daemon, see
[Recipes](recipes.md#leave-global-hotkeys-to-another-tool).

## React to what Neru does

Steps under [`[hooks]`](../reference/configuration.md#hooks) run when
something happens, such as a mode opening or the focused app changing. Their
`exec` steps see the event as environment variables.

```toml
# ~/.config/neru/config.toml
[hooks]
on_mode_enter = "exec sketchybar --trigger neru_mode MODE=\"$NERU_MODE\""
on_mode_exit  = "exec sketchybar --trigger neru_mode MODE=idle"
```

## Talking to the daemon directly

To skip the `neru` binary, send JSON over the socket. The format is in
[IPC protocol](../reference/ipc.md).
