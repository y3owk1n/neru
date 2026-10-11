# The wire protocol breaks only at a major

**Status**: accepted

The daemon's only compatibility check was the build string. The `neru` binary
sends it and the daemon refuses any other build, which is right for the two
halves of one program. A third-party client sends no build string, so every
check passed, and it found out about a renamed key or a removed command when
its feature stopped working. `docs/reference/ipc.md` listed the modes, the
queries and `watch`, and left out fourteen commands, so part of the contract a
client codes against existed only in the source.

So we gave the wire a number, `protocol`, and a written contract in
`docs/reference/ipc.md`. **The number rises only when a change breaks a client,
and only a major release makes one.** Adding a command, a flag, a `data` key,
an event or a response code leaves it alone. A request may carry the number it
needs, and a daemon on a lower one refuses it. Every reply carries the
daemon's. **Unknown request fields stay refused**, as they always were.

## Considered options

- **A level that rises with every addition**, as Neovim's API level does. A
  client could then ask for "the level that added `query-hints`" and get a
  clean refusal from an older daemon. The cost is the bump itself: forgetting
  one is silent to CI, so by ADR 0011 it needs a guardrail, and that guardrail
  has to snapshot every handler name, mode flag, action name, event name and
  wire struct's JSON tags. A client already learns whether a daemon has a
  command or flag by sending it, since `ERR_UNKNOWN_COMMAND` and
  `ERR_INVALID_INPUT` answer per request.
- **Accept unknown request fields.** It would let a newer client send an
  optional field to an older daemon and have it ignored. Ignoring a field the
  client relies on is the silent failure this record exists to remove, and the
  refusal also catches a misspelled field on the client's first run.
- **Keep the build string as the contract.** Every Neru release would break
  every third-party client, including releases that change nothing on the wire.

## Consequences

- Neru 2 ships protocol 2. It removes the action names `mouse_down` and
  `mouse_up`, which a client can send.
- A daemon from before the number refuses the `protocol` field as unknown. The
  reference tells a client to send `ping` first and read the reply.
- We removed the `params` request field, which nothing documented and the
  daemon ignored, so the documented envelope is the whole envelope.
- `internal/architecture/ipc_reference_test.go` holds the reference's command
  table to the commands the daemon registers, in both directions. A new command
  without a row is silent to CI otherwise. No test pins the `data` keys, so only
  review against the reference catches a change to one.
