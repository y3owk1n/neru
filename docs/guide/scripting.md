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

## Check Neru's health

[`neru doctor --json`](../reference/cli.md#neru-doctor) prints every check as
one object, and the reference says what each key means. This notifies when
the check fails:

```bash
neru doctor --json | jq -e .healthy >/dev/null || notify-send "Neru is unhealthy"
```

[`neru services status --json`](../reference/cli.md#neru-services) reports
whether the login service is installed and running, and
[`neru roles --json`](../reference/cli.md#neru-roles) how your roles resolve on
this machine.

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

Hooks also fire on each selection a mode makes, sticky modifiers, cursor
slots, display changes, and the daemon starting and quitting. The reference
lists every event and what each one passes.

## Hand the picked point to a script

A binding's [`--on-exit`](../reference/cli.md#mode-flag-reference) steps see
the selection in `NERU_X` and `NERU_Y`, so a script can use the point you pick
without a hook running for every binding. This copies it to the clipboard on
macOS:

```toml
# ~/.config/neru/config.toml
[hotkeys]
"Primary+Shift+P" = "hints --action move_mouse --on-exit 'exec printf \"%s,%s\" \"$NERU_X\" \"$NERU_Y\" | pbcopy'"
```

## Keep a status bar in sync

A hook starts a process per event. A bar that runs its own long-lived command,
such as Waybar's `exec` or an Eww `deflisten`, can read
[`neru watch`](../reference/cli.md#neru-watch) instead. The script below prints
the current mode, then a new line each time the mode changes. `--event` leaves
out every other event, and a `mode_exit` with reason `switched` prints nothing,
because the next mode's `mode_enter` follows.

```bash
neru watch --event mode_enter,mode_exit | jq --unbuffered -r '
  if .event == "snapshot" then .status.mode
  elif .event == "mode_enter" then .mode
  elif .reason != "switched" then "idle"
  else empty end'
```

Save it as an executable script, for example `~/.local/bin/neru-mode`, and
point the bar at it:

```text
; eww.yuck
(deflisten neru_mode "neru-mode")
```

## Ask what Neru sees

[`neru query`](../reference/cli.md#neru-query) reports the displays and the
cursor in the coordinates `neru action move_mouse` takes, so a script can pass
them straight back. This prints the name of the display under the cursor:

```bash
neru query cursor --json | jq -r .display
```

## Select an element by its label

[`neru query hints`](../reference/cli.md#neru-query) reports the label hints
mode would draw on each element. To select one, open hints mode with the same
flags and feed it the label. This script clicks the first element its flags
select:

```bash
#!/bin/sh
# neru-click: click the first element the given hints flags select
label=$(neru query hints "$@" --json | jq -r '.hints[0].label // empty')
[ -n "$label" ] || { echo "neru-click: no match" >&2; exit 1; }
neru hints "$@" --action left_click
neru action feed --mode $(echo "$label" | fold -w1)
```

Save it as an executable script, for example `~/.local/bin/neru-click`, and
run `neru-click --text Save`. It passes the same flags to both commands,
because Neru assigns labels afresh on every scan and a label holds only for
the same flags. `fold -w1` splits a label such as `AS` into one key per
argument.

## Click what another tool found

When an app hides its controls from every strategy, a tool that knows where
they are can pass them to hints mode. `--targets` takes the JSON `neru query
hints --json` prints, so you can filter a query and feed it back, and any tool
that writes the same shape works too:

```bash
some-tool --emit-targets | neru hints --targets - --action left_click
```

The details are under
[Supplying your own targets](../reference/cli.md#supplying-your-own-targets).

## Talking to the daemon directly

To skip the `neru` binary, send JSON over the socket, or import the Go client
`github.com/y3owk1n/neru/client`. See [Build on Neru](integrations.md) for
programs built this way, and [IPC protocol](../reference/ipc.md) for the
protocol and its schemas.
