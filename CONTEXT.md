# Neru

Neru is a keyboard-driven, mouse-free navigation tool: every pointer action
reachable from the keyboard, instantly. This file is the glossary — the words
this repo uses for its own concepts, and the words it deliberately does not.

Mode, Port / Adapter and Semantic role are defined in `AGENTS.md`
under Domain Concepts and are not repeated here; each fact has one home.

## Language

### Triggering work from the keyboard

**Hotkey**:
A key combination the daemon registers globally, so pressing it reaches Neru
before the focused application sees it.
_Avoid_: shortcut, accelerator

**Binding**:
The mapping from one hotkey to the ordered steps it runs.
_Avoid_: keybinding, mapping

**Step**:
One unit of work written in a binding — a mode command, an action, a shell
command, or a nested sequence.
_Avoid_: action string, command, entry

**Sequence**:
An ordered list of steps run as one unit. A binding, a macro body, and a
mode's `--on-exit` are all sequences, and all behave identically.
_Avoid_: chain, pipeline, script

**Macro**:
A named, parameterised sequence that steps can invoke by name.
_Avoid_: alias, function, snippet

**Focused app**:
The application the operating system currently routes keystrokes to. Neru
names it by the platform's application identifier, and uses it to decide which
per-app overrides apply.
_Avoid_: frontmost app, active app, current app, bundle ID as a bare noun

**Keymap**:
The [[Binding]]s in force right now — the ones the current mode answers to, or
the global ones when no mode is active, with the [[Focused app]]'s overrides
already applied. A keymap is settled when the mode, the focused app or the
configuration changes, not when a key arrives; a keystroke consults one, it
never builds one. That is the rule the word carries, and the decision behind
it is ADR 0005.
_Avoid_: dispatch table, hotkey map, resolved config, keymap table

### Configuring Neru

**Option**:
One setting a person can write in `config.toml`: one key path, one default,
one meaning. An Option is declared once; its default, its line in the example
configuration and its row in the reference are projections of that
declaration, never independent facts. When they disagree, the declaration is
right and the others are stale.
_Avoid_: setting, key, field, knob, param, config value

### Entering a mode

**Mode command**:
A step that names a navigation mode, written with flags — `hints --action
left_click`. This is the text form, the thing a person writes in a binding or
types after `neru`.
_Avoid_: mode invocation, mode action string, launcher

**Mode flag**:
One named option a mode command accepts. Which modes accept a given flag is
part of the flag's definition, not of the mode's.
_Avoid_: option, param, arg, switch

**Activation**:
The typed intent to enter a mode: which mode, plus the value of every mode
flag that was given. A mode command parses into an Activation; the CLI builds
one directly from its flags; both reach the mode handler as the same value.
_Avoid_: mode activation options, request, params, opts, config

**Probe**:
A read-only query reporting what a mode would target, without entering it.
_Avoid_: debug mode, dry run, preview

### Drawing on screen

**Overlay**:
The transparent, always-on-top surface Neru draws on.
_Avoid_: window, HUD, canvas

**Frame**:
The complete description of what should be on screen for one mode. A mode hands
over a Frame; realising it — showing the overlay, switching to the mode,
drawing — belongs to the adapter, not to the mode.
_Avoid_: draw call, render pass, overlay state

**Refresh**:
A mode putting its [[Frame]] back on screen after the world changed underneath
it. There are three causes and no others: a screen change, a theme change, and
a monitor move. Anything that is not a mode restoring its own drawing is not a
refresh, whatever the function is called today.
_Avoid_: overlay resize, style re-resolution, hotkey re-registration, xkb state
reload, virtual-pointer repositioning

**Indicator**:
A small overlay that tracks the cursor and reports state, independent of the
active mode's own drawing: the mode indicator, the sticky-modifiers indicator,
the virtual pointer.
_Avoid_: badge, HUD, widget

**Badge**:
The chip drawn behind a label — a hint's, a recursive-grid cell's, a
monitor-select target's. A rendering primitive, not an [[Indicator]].
_Avoid_: pill, chip, tag

**Style**:
An overlay's resolved appearance: configuration combined with the current
light/dark theme. Resolved once, at the adapter boundary.
_Avoid_: theme, palette, config

### Reaching the operating system

**Bridge**:
A package that compiles native source — Objective-C on macOS, C on Linux — and
publishes it as headers. A Bridge's interface is those headers, not its Go API:
other packages call it by including a header, and the Go import is what links
the compiled objects into the binary. So a package that includes a Bridge's
header states a direct import of it even when it calls no Go symbol there, and
that import is deliberate rather than dead. Untagged code cannot import a
Bridge at all — it crosses through a build-tagged pair. The decision is ADR
0009.
_Avoid_: cgo package, native layer, FFI layer, glue

### Computing the same thing twice

**Shared derivation**:
A value more than one layer computes from the same inputs and must compute
identically: where a grid cell is drawn and where the cursor lands inside it,
what a written font name means. A shared derivation has exactly one
implementation, and it lives at the lowest layer that has more than one
caller — not in the domain by default. Where the second implementation is in
Objective-C, Go cannot be the one implementation, so the copies are pinned by
a test instead. ADR 0007 has the reasoning.
_Avoid_: helper, util, common code, shared logic

### Naming what a person can write

**Vocabulary**:
A closed set of names a person can write and Neru promises to recognise — the
semantic roles, the named keys, the hint placements. A vocabulary is declared
exactly once, in Go; a validator, a platform table or a docs row is a
projection of that declaration, and a copy in another language is pinned to it
the way a [[Shared derivation]]'s language-boundary copy is, never generated.
Which declaration a contested vocabulary belongs to is ADR 0008.
_Avoid_: role list, key table, lookup table, enum
