# A vocabulary has one home

**Status**: accepted

Two of the vocabularies a person writes into `config.toml` turned out to be
declared more than once, with more than one file claiming to be canonical.

The named keys exist in `internal/domain/keyvocab` — nine names and a doc
comment claiming every producer and consumer must go through it — and in
`internal/config`'s `validNamedKeys` — the full set, Space through F24, and a
comment claiming every validator, normalizer and key parser should reference
it. They exist twice more in Objective-C: `keymap_darwin.m`'s name-to-keycode
dictionary and `eventtap_darwin.m`'s `specialKeyName`, which drops the
`Enter`/`Backspace` aliases the first copy carries. Nothing holds the four
together — `internal/architecture/keyvocab_wire_test.go` pins only the
`__keyup_`/`__modifier_` wire prefixes — and they have diverged where nobody
was looking. X11 and Windows never emit `PageUp`, `PageDown`, `Home` or `End`
although the validator accepts them. macOS numpad Enter reaches comparison as
the raw character `"\x03"` and matches nothing, while numpad Clear arrives as
`"\x7f"` and fires a `delete` binding. Linux emits `Insert`, which the
validator rejects. Every one of these fails silently, which is what a
vocabulary written four times does.

Those four are the state this ADR was written against, and three are now
fixed: the navigation keys reach X11 and Windows (#1382), the macOS numpad
folds to named keys (#1377), and `Insert` validates everywhere (#1390). The
paragraph above is kept as written because the decision below was made against
it, and because the shape it describes outlived the instances — `Insert`
became the accepted spelling that X11, Windows and macOS all dropped, the same
failure wearing the opposite sign. X11 and Windows emit it now (#1392), and
macOS is the one backend left. That one is not the same failure: Carbon
declares no Insert virtual key code, so it is the documented F21–F24 gap under
another name.

The element roles have the opposite shape: one real home —
`internal/domain/element`'s `RoleVocabulary` table, with per-platform coverage
tests on the AT-SPI and UIA columns — and a spelling that looks like a
mistake, because the platform-neutral `Role` constants are literally AX
strings and the Windows backend emits `"AXUnknown"` and `"AXWindow"`.

We decided both questions the same way: **a vocabulary — a closed set of
names Neru promises to recognise — is declared exactly once, in Go, and every
other appearance is a projection of that declaration or a pin against it.**
For the named keys the home is `internal/domain/keyvocab`, which absorbs
`validNamedKeys`; `internal/config`'s validators and normalizers and all
three taps read it, and the Objective-C copies get pins in ADR 0007's
inventory. For the element roles the home stays where it is, and the AX
spelling stays the interchange spelling. *Vocabulary* is in `CONTEXT.md`.

## Considered options

- **Generate the native tables from the Go declaration.** What the
  architecture review proposed. Rejected: it would be this repo's first
  `go:generate`, and ADR 0007 already chose the other answer three times —
  where the second implementation is in another language, pin it, don't
  delete or overwrite it. The name tables are a small fraction of the `.m`
  files they live in; a generator would own the file to write the fraction.
- **Let `internal/config` keep the key names.** Defensible — the spellings
  are user-facing config surface, and `validNamedKeys` is the copy that
  actually runs today. Rejected because the domain already owns the words for
  the other vocabulary (roles live in `internal/domain/element`, which the
  config validators import), the Linux and Windows taps already import
  `keyvocab`, and config→domain is an import that exists while domain→config
  is one that must not.
- **Invent a neutral role spelling.** Renaming `RoleButton = "AXButton"` to
  something platform-free touches ~30 write-down sites for zero user-visible
  change, and every mapping table, docs row and diagnostic message moves with
  it. The AX spelling earned interchange status by being first; a UIA backend
  emitting `"AXUnknown"` is surprising exactly once, and this ADR is where
  the surprise is recorded.

## Consequences

- `keyvocab` absorbs `validNamedKeys` and the competing canonicality comment
  goes; a named key is added in one place. `Insert` joins the set — evdev and
  Wayland emit it today and the validator rejects it, the same shape as the
  documented F21–F24 gap. `CapsLock` is declined: a modifier-shaped key with
  per-platform semantics nobody has asked for.
- The silent parity failures above are bugs, not vocabulary decisions, and
  are fixed as bugs: X11 and Windows emit the navigation keys; macOS folds
  numpad keys to named keys the way Wayland already folds `KP_Enter` to
  `Return`.
- The macOS name tables become the first string-list pins in ADR 0007's
  inventory, anchored to the one Go declaration — and pinned to each other,
  since one Objective-C copy carries aliases the other drops.
- `RoleDiagnosticNoNativeEquivalent` reaches the ADR 0002 warning tier, so
  `neru config validate` stops printing "Configuration is valid" for an entry
  that resolves to nothing on the running platform and shows up only in
  `neru roles --explain`.
- The stale role examples in CLI help (`AXButton,AXLink`, a spelling that is
  now a fatal diagnostic) are fixed, and the docs guard extends over CLI help
  strings so the vocabulary's own documentation cannot direct a person to a
  fatal spelling again.
- Deliberately unpinned: `shouldPrefetchActions` and `interactiveLeafRoles`.
  Both are heuristics whose disagreement with config is bounded by an
  equivalent fallback — divergence there costs a round-trip, not a hint — and
  a pin would freeze a heuristic as if it were a contract.
- One fact this ADR's grilling settled halfway: the macOS SDK declares
  `AXSearchField`, `AXToolbarButton`, `AXSwitch` and `AXTabButton` as
  *subroles*, and Neru's matching compares role only. A live AX probe
  (2026-08-09, Safari and System Settings) confirmed two of the four in
  practice — search fields report `AXTextField / AXSearchField` and SwiftUI
  toggles report `AXCheckBox / AXSwitch`, so the `search_field` and `switch`
  vocabulary entries match nothing on macOS and are carried today only by
  `text_field` and `checkbox` also being defaults. `AXToolbarButton` and
  `AXTabButton` are unprobed (no app with those controls was running). The
  fix is a mapping-table change — the AX column learning about subroles —
  not a change to this rule.
