# IPC protocol

The wire format between the `neru` CLI and the daemon. Use it to drive Neru
without spawning the `neru` binary. For most scripts, calling `neru` is
simpler, see [Scripting](../guide/scripting.md).

Each call sends one JSON request and reads one JSON response.

## Endpoint

The endpoint is private to your user:

| Platform        | Endpoint                                                                 |
| --------------- | ------------------------------------------------------------------------ |
| macOS and Linux | `$XDG_RUNTIME_DIR/neru/neru.sock`, else `$TMPDIR/neru-<uid>/neru.sock`   |
| Windows         | The named pipe `\\.\pipe\neru-<SID>`                                     |

The daemon prints its endpoint at startup. It queues commands, so concurrent
calls are safe.

## Request

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

## Probing without activating

`hints-probe` returns, in `message`, a count and a sample of what hints mode
would target in the focused window, without drawing or entering a mode. It
accepts only `--role`, `--text`, `--strategy` and `--split-word`, and refuses
anything else with `ERR_INVALID_INPUT`. `neru hints --debug` sends it.

```json
{ "action": "hints-probe", "args": ["--role=button", "--strategy=vision"] }
```

## Response

The reply has optional `data` (the command's payload, such as the status
object) and `version` (the daemon's build version):

```json
{ "success": true, "message": "OK", "code": "OK" }
```

## Response codes

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
