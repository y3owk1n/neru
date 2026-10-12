# IPC protocol

The wire format between the `neru` CLI and the daemon. Use it to drive Neru
without spawning the `neru` binary. For most scripts, calling `neru` is
simpler, see [Scripting](../guide/scripting.md).

Each call sends one JSON request and reads one JSON response, except a
[stream](#streaming), which keeps sending lines after it.

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
{ "action": "hints", "args": ["--action=left_click"], "protocol": 1 }
```

| Field      | Type     | Description                                                                 |
| ---------- | -------- | --------------------------------------------------------------------------- |
| `action`   | string   | Required. The [command](#commands).                                         |
| `args`     | string[] | The command's arguments, written as a user types them after `neru`.         |
| `protocol` | int      | The [protocol](#compatibility) the client needs. The daemon refuses a higher number than its own with `ERR_VERSION_MISMATCH`. |
| `version`  | string   | The `neru` binary's build version. The daemon refuses any other build with `ERR_VERSION_MISMATCH`. Other clients leave it out. |

The daemon refuses a field it does not know with `ERR_INVALID_INPUT`.

## Response

```json
{ "protocol": 1, "success": true, "message": "OK", "code": "OK" }
```

| Field      | Type    | Description                                                     |
| ---------- | ------- | --------------------------------------------------------------- |
| `protocol` | int     | The [protocol](#compatibility) the daemon speaks.               |
| `success`  | bool    | Whether the command did what it was asked.                      |
| `code`     | string  | One of the [response codes](#response-codes).                   |
| `message`  | string  | A sentence for people. Branch on `code`, never on this.         |
| `data`     | any     | The command's payload, when it has one, such as the status object. |
| `version`  | string  | The daemon's build version.                                     |

## Compatibility

Every response carries `protocol`, the version of this contract the daemon
speaks. It is `1`.

Within one protocol, the request and response fields, the response codes, the
command names, and the `data` keys documented here and in the
[CLI reference](cli.md) keep their names, types and meaning. Not covered:

- The `message` text.
- The `version` string.
- The contents of `capabilities`, `profile` and `components`.
- Anything this page and the CLI reference do not document.

A protocol can gain commands, flags, `data` keys, events and response codes,
so ignore keys and events you do not know. Only a change that breaks a client
raises the number, and only a major Neru release makes one. Neru 2 is
protocol 2, which removes the action names `mouse_down` and `mouse_up`.

Writing a client:

- Send `protocol` with the number you wrote against, and check `protocol` in
  the reply. A daemon on a higher one may have changed something you use.
- To tell whether the daemon has a command or flag, send it. A daemon without
  one answers `ERR_UNKNOWN_COMMAND` or `ERR_INVALID_INPUT`.
- A daemon from before the protocol was numbered refuses the `protocol` field
  as unknown, and its replies carry none. Send `{"action":"ping"}` first to
  find out which you have.

A Go program can import `github.com/y3owk1n/neru/client` instead of writing
any of this. It finds the endpoint the way the CLI does, sends `protocol` 1,
and returns typed replies for `status`, the queries and `watch`. Its API
follows the rule above and breaks only at a major Neru release. See the
[package documentation](https://pkg.go.dev/github.com/y3owk1n/neru/client).

## Schemas

Neru publishes the shapes on this page as a JSON Schema, generated from the
types the daemon encodes:

```text
https://raw.githubusercontent.com/y3owk1n/neru/main/schemas/ipc.schema.json
```

Replace `main` with a release tag to read that release's copy. Releases from
before the schema have no copy of it. Each shape is a definition to `$ref`,
such as `#/definitions/status`:

| Definition | Shape |
| ---------- | ----- |
| `request`, `response` | A request and a reply |
| `status` | The `data` of `status`, and the `status` of a watch snapshot |
| `query_displays`, `query_cursor`, `query_window`, `query_app`, `query_hints` | The `data` of each query |
| `toggle_scroll_invert`, `toggle_screen_share`, `toggle_cursor_follow_selection` | The `data` of each toggle |
| `watch_line` | One line of a watch after its reply |
| `config` | The `data` of `config` with no `args`, a reference to `config.schema.json` |

A key the schema marks required is always present, and may be `null` where
the schema allows it. A reply may carry keys the schema does not list, as
[Compatibility](#compatibility) allows, and a request may not.

## Commands

`args` are the words a user types after the command, one string each, with
flags written as `--flag=value` or as two entries.

The daemon parses mode flags exactly as the CLI does. It refuses an unknown
flag, a flag the mode does not accept, an unusable value, or an unmet
dependency such as `--on-exit` without `--action` with `ERR_INVALID_INPUT`. It
ignores a leading repeat of the mode's own name in `args`.

| Command | `args` | `data` |
| ------- | ------ | ------ |
| `hints`, `grid`, `recursive_grid`, `bisect`, `scroll`, `monitor_select`, `idle` | The mode's flags, from the [mode flag reference](cli.md#mode-flag-reference). | none |
| `mode` | The declared mode's name, then optionally `--toggle`. | none |
| `action` | An [action name](cli.md#action-names), then its flags, as after `neru action`. | none |
| `run` | One entry per step, as [`neru run`](cli.md#neru-run) takes them. `--stop-on-error` as an entry of its own sets the policy. | none |
| `macro` | The macro's name, then its arguments. | none |
| `ping` | none | none |
| `start`, `stop` | none | none |
| `status` | none | The object [`neru status --json`](cli.md#neru-status) prints. |
| `health` | none | The checks [`neru doctor`](cli.md#neru-doctor) reports, as `version`, `config`, `mode`, `capabilities`, `profile` and `components`. A failed check makes the reply `ERR_ACTION_FAILED`, and the reply still carries `data`. |
| `config` | None, or `set <key> <value>`, `reset <key>` or `reload`. `set` and `reset` take `--no-reload` last. | With no `args`, the running configuration under the key names `config.toml` uses, which [`neru config dump`](cli.md#neru-config-dump) prints. |
| `config-set` | `<key> <value>`, then optionally `--no-reload`. | none |
| `reload` | none | none |
| `toggle-scroll-invert` | Optionally `--state on\|off\|toggle`, as in [Runtime toggles](cli.md#runtime-toggles). | `{"inverted": bool}` |
| `toggle-screen-share` | Optionally `--state on\|off\|toggle`, as in [Runtime toggles](cli.md#runtime-toggles). | `{"hidden": bool}` |
| `toggle-cursor-follow-selection` | Optionally `--state on\|off\|toggle`, as in [Runtime toggles](cli.md#runtime-toggles). | `{"following": bool}` |
| `query-displays`, `query-cursor`, `query-window`, `query-app`, `query-hints` | See [Querying](#querying). | See [Querying](#querying). |
| `watch` | none | A [stream](#streaming). |

While Neru is stopped, the daemon refuses with `ERR_NOT_RUNNING` everything
except `ping`, `start`, `stop`, `idle`, `status`, `health`, `config`,
`config-set`, `reload`, the toggles, `watch` and every query but
`query-hints`.

## Querying

`query-displays`, `query-cursor`, `query-window`, `query-app` and
`query-hints` return in `data` the objects [`neru query`](cli.md#neru-query)
prints with `--json`. Only `query-hints` takes `args`: `--role`, `--text`,
`--strategy`, `--capture-scope` and `--split-word`, written as a user types
them. It refuses anything else with `ERR_INVALID_INPUT`.

```json
{ "action": "query-hints", "args": ["--role=button", "--strategy=vision"] }
```

## Streaming

`watch` keeps the connection open. The daemon sends one response, then one
JSON object per line until it exits or you close the connection. The lines
are the ones [`neru watch`](cli.md#neru-watch) prints.

```json
{ "action": "watch" }
```

Send nothing after the request. To end a watch, close your end. The daemon
disconnects a reader once a line has waited 5 seconds to send, and a 17th open
watch gets `ERR_BUSY`.

## Response codes

| Code                    | Meaning                                                  |
| ----------------------- | -------------------------------------------------------- |
| `OK`                    | Command succeeded                                        |
| `ERR_UNKNOWN_COMMAND`   | No such command                                          |
| `ERR_INVALID_INPUT`     | Malformed arguments, flag values or request fields       |
| `ERR_NOT_RUNNING`       | Neru is paused via `neru stop`                           |
| `ERR_ALREADY_RUNNING`   | Target is already in the requested state                 |
| `ERR_MODE_DISABLED`     | The requested mode is disabled in the configuration      |
| `ERR_ACTION_FAILED`     | The action was dispatched but did not complete           |
| `ERR_CHAIN_BAIL`        | An action chain aborted, for example `--bail`            |
| `ERR_NOT_SUPPORTED`     | Not implemented on this platform                         |
| `ERR_VERSION_MISMATCH`  | The client needs a newer protocol, or the `neru` binary and the daemon are different builds. Restart the daemon after an upgrade. |
| `ERR_BUSY`              | Too many watches are open                                |

A connection error rather than a response code means no daemon is running.
