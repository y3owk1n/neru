# Neru

Neru is a keyboard-driven, mouse-free navigation tool: every pointer action
reachable from the keyboard, instantly. This file is the glossary — the words
this repo uses for its own concepts, and the words it deliberately does not.

Mode, Port / Adapter, Bridge and Semantic role are defined in `AGENTS.md`
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
