# Neru agent guide

This is Neru {{version}}, speaking IPC protocol {{protocol}}. Everything below
is true of this build. When the docs, a skill or your memory disagree with
it, this build is right.

Neru is a keyboard-driven mouse replacement. A daemon, started with
`neru launch`, watches hotkeys, draws overlays and moves the cursor. Every
other `neru` command sends it a request over a per-user socket. Every
command a hotkey runs is also a `neru` command and an IPC request, so a
program can do anything a binding can.

## Learn what this build accepts

These run without the daemon and without a config:

- `neru describe --json` lists every mode, mode flag, action, config option,
  event and hook variable this build reads. `platforms` says where each one
  does something, and `here` says whether it does on this machine. Use it
  instead of guessing a name.
- `neru schema config` prints the config.toml schema, with each option's
  type and default.
- `neru schema ipc` prints the IPC schema, one definition per request, reply
  payload and watch line.
- `neru <command> --help` gives a command's flags and what each does. On
  macOS and Linux, `man neru-<command>` has the same text, such as
  `man neru-query-hints`.
- `neru doctor --json` reports config validity, permissions, the socket path
  under `ipc_endpoint.path`, and whether a daemon answers there.

The docs for this build start at {{docs}}.

## Read what Neru sees

These need the running daemon:

- `neru status --json` gives the mode, whether Neru is paused, and the
  runtime toggles.
- `neru query displays|cursor|window|app --json` gives positions and sizes in
  the coordinates `neru action move_mouse` takes.
- `neru query hints --json` lists what hints mode would label in the focused
  window, with each element's role, position and text. Run from a terminal
  it reports the terminal, so wait a few seconds or run it from a hotkey.
- `neru watch --event <name>,...` streams events as JSON lines, after a
  snapshot of the whole state. Rebuild from every snapshot, since a reader
  that falls behind gets one in place of the events it missed.

## Drive it

- A mode: `neru hints --action left_click`, `neru grid`, and the others
  `neru describe` lists under `modes`.
- An action without a mode: `neru action <name>`, such as
  `neru action snap --text Save` to move the cursor onto an element.
- Several steps as one unit: `neru run "<step>" "<step>"`.
- Targets your program found: `neru hints --targets -` reads them as JSON
  on stdin, in the shape `neru query hints --json` prints.
- The socket directly: the request and reply shapes are in `neru schema ipc`.
  A Go program can use the client package,
  `go get github.com/y3owk1n/neru/client@{{ref}}`.

## Change the config

- `neru config validate` checks a file before the daemon loads it.
- `neru config set <key> <value>` applies one value at once and saves it to
  `config.override.toml` beside the config, which overrides `config.toml`.
  Tell the user, since a later edit to `config.toml` will not undo it.
- `neru config dump` prints the configuration in force.
- An option whose `here` is false loads with a warning and does nothing on
  this machine.

## Rules

- **Errors.** Every failing command exits 1. When the daemon refused the
  request, stderr ends with its response code, such as
  `(code: ERR_NOT_RUNNING)`. Branch on the code, not the message.
- **Paused.** `neru stop` pauses without quitting. Modes, actions and
  `query hints` answer `ERR_NOT_RUNNING` until `neru start`. Status, describe,
  config and the other queries still answer.
- **Compatibility.** Ignore keys and events you do not know. Neru adds them
  in minor releases and removes them only at a major one, which raises the
  protocol.
- **Labels go stale.** A hint label holds only while the screen does not
  change and only for the same flags. Use it straight after the query.
- **Privacy.** `neru query hints` returns the text on the user's screen.
  Do not log it, store it, or send it anywhere the user did not ask for.
