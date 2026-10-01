# Scripting

Every `neru` command prints its reply, writes errors to stderr, and exits
non-zero on failure. Commands and flags are in the [CLI reference](cli.md),
and the `neru status --json` fields under [`neru status`](cli.md#neru-status).

## Examples

**Toggle the daemon**

```bash
if [ "$(neru status --json | jq -r .enabled)" = true ]; then neru stop; else neru start; fi
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
```

**Run several steps as one unit**

Use [`neru run`](cli.md#neru-run) instead of chaining `neru` calls with `&&`.
It sends the whole sequence in one request, under hotkey binding rules.

## IPC protocol

The CLI and the daemon exchange one JSON object each way over a per-user Unix
socket, or a per-user named pipe on Windows
([Runtime shape](../contributing/architecture.md#runtime-shape)). The daemon
queues commands, so concurrent calls are safe.

**Request**

```json
{ "action": "hints", "args": ["--action=left_click"] }
```

`action` names either a mode command (`hints`, `grid`, `recursive_grid`,
`bisect`, `scroll`, `monitor_select`, `idle`, or `mode` with the declared name
as the first entry of `args`) or a standalone command. `args` carries the
flags a user would type. The optional `version` field carries the client's
build version, and the daemon refuses a mismatch with `ERR_VERSION_MISMATCH`.

The daemon parses mode flags exactly as the CLI does. It refuses an unknown
flag, a flag the mode does not accept, an unusable value, or an unmet
dependency such as `--on-exit` without `--action` with `ERR_INVALID_INPUT`. A
leading repeat of the mode's own name in `args` is ignored.

**Probing without activating.** `hints-probe` returns a count and a sample in
`message` of what hints mode would target in the focused window, without
drawing or entering a mode. It accepts only `--role`, `--text`, `--strategy` and `--split-word`, and refuses
anything else with `ERR_INVALID_INPUT`. `neru hints --debug` sends it.

```json
{ "action": "hints-probe", "args": ["--role=button", "--strategy=vision"] }
```

**Response**, with optional `data` (the command's payload, such as the status
object) and `version` (the daemon's build version):

```json
{ "success": true, "message": "OK", "code": "OK" }
```

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
