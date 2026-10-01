# Scripting

Every `neru` command is an ordinary process. It prints its reply, writes errors
to stderr, and exits non-zero on failure. That makes Neru easy to drive from
shell scripts and external hotkey daemons. This page shows the common patterns
and documents the protocol the CLI speaks to the daemon, for tools that want to
skip the CLI. What each command and flag does is in the
[CLI reference](cli.md). `neru status --json` and its fields are described
under [`neru status`](cli.md#neru-status).

## Examples

**Toggle the daemon**

```bash
if [ "$(neru status --json | jq -r .enabled)" = "true" ]; then
    neru stop
else
    neru start
fi
```

**Check whether the daemon is reachable**

```bash
neru status &>/dev/null && echo "Running" || echo "Not running"
```

**Drive Neru from an external hotkey manager**

```text
# ~/.config/skhd/skhdrc
ctrl - f : neru hints
ctrl - g : neru grid
ctrl - r : neru hints --action right_click
ctrl - t : neru hints --action left_click --repeat
```

**Run several steps as one unit**

Chaining `neru` invocations with `&&` spawns a process and opens a connection
per step. [`neru run`](cli.md#neru-run) sends the whole sequence once and the
daemon executes it in order, under the same rules a hotkey binding gets:

```bash
neru run "action save_cursor_pos" "hints --action left_click" \
         "action wait_for_mode_exit --bail" "action restore_cursor_pos"
```

## IPC protocol

The CLI and the daemon exchange one JSON object each way over a per-user Unix
domain socket, or a per-user named pipe on Windows. Where the endpoint lives is
described in [Runtime shape](../contributing/architecture.md#runtime-shape).
The daemon queues incoming commands, so concurrent calls from scripts are safe.

**Request**

```json
{ "action": "hints", "args": ["--action=left_click"] }
```

`action` names either a mode command (`hints`, `grid`, `recursive_grid`,
`bisect`, `scroll`, `monitor_select`, `idle`, or `mode` with the declared name
as the first entry of `args`) or one of the standalone commands. `args` carries
the same flags a user would type. An optional `version` field carries the
client's build version. When it differs from the daemon's, the daemon refuses
the request with `ERR_VERSION_MISMATCH`.

A mode command's flags are read exactly as the CLI reads them, and answered
with the same message. An unknown flag, a flag the named mode does not accept,
an unusable value, and an unmet dependency such as `--on-exit` without
`--action` are all refused with `ERR_INVALID_INPUT` rather than accepted and
dropped. Repeating the mode's own name as the first entry of `args` is accepted
and ignored.

**Probing without activating**

`hints-probe` reports what hints mode would target for the focused window and
answers with a count and a sample in `message`. It draws nothing and enters no
mode, so it takes only the flags that decide which elements are collected:
`--role`, `--text`, `--strategy`, `--split-word`. Anything else is refused with
`ERR_INVALID_INPUT`. This is what `neru hints --debug` sends.

```json
{ "action": "hints-probe", "args": ["--role=button", "--strategy=vision"] }
```

**Response**

```json
{ "success": true, "message": "OK", "code": "OK" }
```

A response may also carry `data`, the command's payload (the status object for
`status`), and `version`, the daemon's build version.

**Response codes**

| Code                    | Meaning                                                  |
| ----------------------- | -------------------------------------------------------- |
| `OK`                    | Command succeeded                                        |
| `ERR_UNKNOWN_COMMAND`   | No such command                                          |
| `ERR_INVALID_INPUT`     | Malformed arguments or flag values                       |
| `ERR_NOT_RUNNING`       | Neru is paused via `neru stop`                           |
| `ERR_ALREADY_RUNNING`   | Target is already in the requested state                 |
| `ERR_MODE_DISABLED`     | The requested mode is disabled in the configuration      |
| `ERR_ACTION_FAILED`     | The action was dispatched but did not complete           |
| `ERR_CHAIN_BAIL`        | An action chain aborted, for example `--bail`            |
| `ERR_NOT_SUPPORTED`     | Not implemented on this platform                         |
| `ERR_VERSION_MISMATCH`  | Client and daemon builds differ. Restart the daemon.     |

A connection error rather than a response code means no daemon is running.
