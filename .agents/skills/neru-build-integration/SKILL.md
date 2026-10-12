---
name: neru-build-integration
description: "Help a Neru user build a tool on top of Neru, such as a status bar, a launcher, voice control, a companion UI, automation from another hotkey tool, a Go program, or a client for the IPC socket in another language. Picks the interface, checks the installed version has it, reads the contract from that version's docs, and runs the result against the live daemon. Routes config and hooks work to neru-setup-config. Use when a Neru user wants to script Neru, read its state, follow its events, drive it from another program, or write code against its CLI, socket, or Go client."
---

# Building on Neru

Every command a hotkey runs is also a `neru` command and an IPC request, so a
tool can do anything a binding can, read what Neru sees, and follow what it
does. The work is picking the right interface, confirming the user's Neru has
it, and writing to the documented contract rather than to what one run
happened to print. The interfaces grow between releases, so check before
writing code.

## Pick the interface

The table in `guide/integrations.md` maps what the user wants to the
interface that does it. Fetch it as "Read the contract" below shows, and
choose from that table. Then:

- Hooks, and any other change to the user's config, go to `neru-setup-config`.
- A socket client also needs `reference/ipc.md` for the protocol, and a Go
  program needs `go doc` on the module. Both need `reference/cli.md` for what
  each command's fields mean.

Prefer the CLI for a script. It handles the endpoint, the timeout and the
error codes, and a shell tool needs nothing else. Use the socket or the Go
client for a long-running program, or one that would otherwise start a
process per update.

## Check the installed version has it

Run these before writing anything. When the binary and the docs disagree,
the binary is right. Check the flags the plan uses, such as these:

```bash
neru --version
neru status            # is the daemon running, and does the CLI reach it
neru watch --help      # --event present?
neru query hints --help   # --json and --label-direction present?
neru hints --help      # --targets present?
neru doctor --help     # --json present?
```

When a command or flag the plan needs is missing, so that `--help` does not
list it or the CLI answers `unknown command`, say which interface the
installed version lacks. Say where it is: the nightly channel or a source
build of `main`, or a later release if the
[releases page](https://github.com/y3owk1n/neru/releases) lists one. Then
stop, or offer what the installed version can do and name the difference.
Polling `neru status --json` on a timer can replace `neru watch`, for
example, but it wakes on every tick and not only on a change, so say so.
Never write code against a command or flag the installed binary does not
have, and skip the steps below that use one.

Trust `--help` over a man page. `man neru-watch` and the other pages carry
each command's flags offline on macOS and Linux, but a machine with two
installs can have man pages from the other one. A man page counts only when
the version in its footer matches `neru --version`.

For the Go client, get the module at the same ref as the docs below: the
release tag for a release, `@main` for a nightly or source build. Then read
its API with `go doc`, since the guide shows only part of it:

```bash
go get github.com/y3owk1n/neru/client@v1.2.3   # or @main
go doc -all github.com/y3owk1n/neru/client
```

The client needs a daemon from the release that added the package, or later.
A daemon from before that release does not know the request's `protocol`
field and refuses with `ERR_INVALID_INPUT`. A later daemon that speaks an
older protocol refuses with `ERR_VERSION_MISMATCH`. `Ping` returns the
protocol the daemon speaks, to compare with the package's `ProtocolVersion`.
`ERR_VERSION_MISMATCH` from the CLI means the `neru` binary and the running
daemon are different builds, usually after an upgrade. Restarting the
daemon fixes it: `neru services restart` when it runs as a service, otherwise
quit Neru from its menu bar or tray icon and run `neru launch`.

## Read the contract at the installed version

Fetch the doc that matches the binary. A release prints its tag, such as
`v1.2.3`. Anything else, such as a nightly or a source build like
`v1.2.3-14-gabcdef`, is newer than any tag, so read `main` for it.

```bash
v=$(neru --version | sed -n '1s/^Neru version //p')
case $v in v*.*.*-*) ref=main ;; v*.*.*) ref=$v ;; *) ref=main ;; esac
base="https://raw.githubusercontent.com/y3owk1n/neru/$ref/docs"
curl -fsSL "$base/guide/integrations.md"
curl -fsSL "$base/reference/ipc.md"
curl -fsSL "$base/reference/cli.md"
```

On Windows, in PowerShell:

```powershell
$v = ((neru --version)[0] -replace '^Neru version ', '')
$ref = if ($v -match '^v\d+\.\d+\.\d+$') { $v } else { 'main' }
$base = "https://raw.githubusercontent.com/y3owk1n/neru/$ref/docs"
Invoke-RestMethod "$base/reference/ipc.md"
```

A 404 means that release predates the doc. Read `main` instead, and tell the
user that anything it describes beyond what `--help` lists is newer than
their install.

The IPC reference owns the request and response shape, the endpoint paths,
the response codes and the compatibility rule. Read them there and do not
restate them from memory in code comments or in answers.

## Look at real output first

Run the command the tool will depend on and look at what comes back before
writing a parser:

```bash
neru status --json
neru query hints --json | head -c 2000
neru watch --event mode_enter,mode_exit   # change modes, then Ctrl-C
```

For a socket client, look at the raw reply too. `neru doctor --json` reports
the endpoint under `ipc_endpoint.path`:

```bash
sock=$(neru doctor --json | python3 -c 'import json,sys; print(json.load(sys.stdin)["ipc_endpoint"]["path"])')
python3 -c 'import socket,sys; s=socket.socket(socket.AF_UNIX); s.connect(sys.argv[1]); s.sendall(b"{\"action\":\"status\",\"protocol\":1}"); print(s.makefile().readline())' "$sock"
```

Anything that reads the focused window, such as `query hints`, `query app` or
a one-shot program built on them, reports the terminal when run from one. Run
it as `sleep 3; neru query hints --json` and switch to the target app, and
give a one-shot tool the same delay.

When the user asks for something the output does not carry, such as an app's
display name where Neru reports a bundle id, say so and offer what is there.
Do not guess at a field.

## Follow the documented guarantees

- **Errors.** The CLI exits 1 on every failure and prints the response code
  on stderr. Branch on the code, not the message. Over the socket, a refused
  request carries the code in the reply. The Go client returns `ErrNoDaemon`
  when nothing listens and a `*client.Error` with `Code` when the daemon
  refuses. Anything else is a transport failure, such as a connection that
  broke, and the tool reports it as it is. A CLI script that has to tell no
  daemon apart from a refusal reads `ipc_endpoint.reachable` from
  `neru doctor --json`, which runs without the daemon.
- **No daemon, or paused.** Most commands need the daemon. Handle its absence
  as a state the tool shows, not a crash. `doctor`, `roles` and
  `config validate` run without it. `neru stop` pauses Neru without quitting,
  and the IPC reference lists which commands still answer while it is
  paused.
- **Compatibility.** Ignore fields and events the tool does not know. Neru
  adds them in minor releases and removes them only at a major one.
- **Labels go stale.** A hint label from `query hints` holds only while the
  screen does not change and only for the same flags. Select it right after
  querying, by opening hints with the same flags and typing the label with
  `neru action feed --mode`.
- **Watch.** The stream opens with a snapshot of the whole state, then sends
  one line per event. A reader that falls behind gets a fresh snapshot in
  place of the events it missed, so rebuild the tool's state from every
  snapshot, not only the first. Read the `reason` on `mode_exit` in the
  `neru watch` table before showing a mode, or the display flashes idle on
  every switch between modes.
  The stream ends when the daemon exits. Ask the user whether the tool
  should then exit or reconnect.
- **Output.** A tool another program reads line by line, such as a status
  bar script, flushes after every line.
- **Privacy.** Neru never logs keystrokes, element text or hint search
  terms. A tool built on it should not write them anywhere it keeps either,
  unless that is the tool's purpose and the user asked for it.

## Run it

Run the finished tool against the user's daemon and show it working, such as
the bar updating when a mode opens, or the click landing. A one-shot tool
works when its output matches the CLI's for the same query. macOS ships no
`timeout` command, so bound a test run some other way, such as a time limit
inside the tool or a `kill` after it.

Then check the two states it will meet:

- **Paused.** Read `enabled` from `neru status --json` first. Run
  `neru stop`, confirm the tool shows Neru as paused, then run `neru start`
  only if Neru was enabled before, so the test leaves it as it found it.
- **No daemon.** On macOS and Linux, run the tool with both
  `XDG_RUNTIME_DIR` and `TMPDIR` pointed at an empty, short directory, such
  as one from `mktemp -d /tmp/nr.XXXXXX`. Nothing listens there, so it meets
  the no-daemon state while the real daemon keeps running. A socket client
  with a path option can instead be pointed at a path that does not exist.
  On Windows, ask the user to quit Neru from its tray icon, then relaunch it
  with `neru launch`.

For a Go program, `go vet` and a run against the daemon both pass before
handing it over.

## Routing

- Hooks, hotkeys, macros, declared modes and any other config work go to
  `neru-setup-config`.
- What a command does, or why Neru itself misbehaves, goes to `neru-ask`.
- A missing interface the tool needs is a feature request. Blank issues are
  disabled, so point the user at the issue forms on `y3owk1n/neru`, or
  Discussions for an idea.
