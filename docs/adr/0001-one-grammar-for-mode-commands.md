# One grammar for mode commands

**Status**: accepted

A mode command (`hints --action left_click`) reaches the daemon from three
places — the CLI, a hotkey binding, and a direct IPC caller — and each had
grown its own reading of it. `internal/cli/mode_commands.go` validated fully,
the IPC controller's own mode-option parser validated partially and
differently, and `internal/config/validators_hotkeys.go` stopped at the command
word, so a typo'd binding was discoverable only by pressing the key. The three
had already drifted: `--on-exit requires --action` existed CLI-side only, four
error strings differed, and per-mode flag support was a CLI-only concept, so
`grid --search` in a binding activated grid and dropped the flag in silence. We
decided that one package under internal/domain — `internal/domain/modecmd` —
owns the whole grammar — the flag vocabulary, which modes accept which flag,
parsing, validation, and rendering back to arguments — and that the CLI, the
IPC controller and the config validator all call it rather than each reading
the command their own way.

## Considered options

**A typed payload on the wire instead of an argument list.** This would delete
the CLI's render-then-reparse round trip outright. Rejected: `docs/CLI.md`
documents `{"action":"hints","args":[]}` and invites scripts to use it, so the
argument-list shape is public API. The hotkey path also has to parse text
regardless — a binding is a string in `configs/default-config.toml` — so the
parser would survive the change anyway and only the CLI would benefit.

**Keep both validators and pin them with a test.** Cheaper, and it would catch
future drift. Rejected because the drift has already happened and a test would
have recorded rather than prevented it; the rules are cross-cutting enough
(action vocabulary, flag co-dependencies, per-mode acceptance) that two
implementations agreeing is a coincidence to be maintained, not a property.

**A data-only flag descriptor with parse and render as switches**, exhaustiveness
pinned by an `internal/architecture` test. This is the house style, and the
guardrail suite is where this repo usually puts contracts. Rejected for this
one case: a switch with a `default:` clause is precisely the shape that let the
per-mode divergence go unnoticed, and here the compiler can do the job instead.
Each flag descriptor therefore carries its own `Set` and `Render` closures, so
adding a flag is one table entry and there is no branch to forget.

## Consequences

- A `domain` package knows CLI flag spellings. That reads backwards for a
  hexagonal core, and it is deliberate: the flag names are the vocabulary the
  user writes in `configs/default-config.toml`, not a delivery-mechanism detail.
- The grammar cannot import `internal/config`, because config validates
  bindings and so must import the grammar. The strategy, label-direction and
  cursor-selection-mode constants move down out of `internal/config` and
  `internal/app/modes` to make that possible.
- Error messages are context-free, since one string now serves a shell user, an
  IPC response and a config error. Advice that named `neru action X` on the CLI
  and `action X` on the wire is reworded to be true in both.
- `--debug` becomes its own IPC action rather than a mode flag. It never
  activated anything — it short-circuited to a probe and returned a summary
  string — and leaving it on the Activation would reintroduce a field that
  means nothing to the handler.
- The mode handler takes an Activation directly, so its three activation entry
  points collapse to one and the field-by-field copying between two option
  structs is deleted. `OnExit`'s nil-versus-empty contract (nil preserves prior
  steps, empty clears them) survives that collapse and is pinned by the
  round-trip test.
