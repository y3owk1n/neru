# Glossary

The words these docs use, in alphabetical order. Each entry links to the page
that covers the term in full.

**Action.** One thing Neru does once, without entering a mode, such as
`left_click`, `scroll_down` or `move_mouse`. Written as `action <name>` in a
binding, or `neru action <name>` in a shell. See
[action names](../reference/cli.md#action-names).

**App identity.** How Neru names an app in `app_configs` and `excluded_apps`:
the bundle ID on macOS, `WM_CLASS` or `app_id` on Linux, and the executable
path on Windows. Written as `bundle_id` on every platform. See
[App identity](../reference/configuration.md#app-identity-across-platforms-bundle_id).

**Binding.** A hotkey and the steps it runs, written as one line in a hotkey
table, such as `"Primary+Shift+Space" = "hints"`. See
[How bindings work](bindings.md).

**Capture scope.** The region a mode works in: the focused `window`, or the
whole `screen`. Set with `capture_scope` or `--capture-scope`.

**Daemon.** The background process `neru launch` starts. It registers the
hotkeys and draws the overlays. Every other `neru` command sends it a request, so most commands
fail with a connection error when it is not running.

**Focused app.** The app your keystrokes go to. Neru applies its
`app_configs` entry, if one matches its app identity.

**Hint.** A short label Neru draws on a clickable element in hints mode. Type
it to move the cursor there.

**Hotkey.** A key combination Neru registers, such as `Primary+Shift+Space`.
Global hotkeys work from anywhere, and a mode's hotkeys work while that mode is
open. Also called a chord when it has modifiers. See
[hotkey syntax](../reference/configuration.md#global-hotkeys).

**Idle.** No mode is open. `Escape` returns to idle, and so does the step
`idle`.

**Indicator.** A small label that follows the cursor and shows state, such as
the current mode or which sticky modifiers are held. See
[`[mode_indicator]`](../reference/configuration.md#mode_indicator).

**Macro.** A named sequence you define under `[macros]` and run with
`macro <name>`, optionally with arguments. See
[`[macros]`](../reference/configuration.md#macros).

**Mode.** A state where Neru captures the keyboard and draws an overlay, such as
hints, grid, recursive grid, bisect, scroll or monitor select. You can also
declare your own under [`[modes]`](../reference/configuration.md#modes). See
[Using Neru](../guide/using-neru.md).

**Mode flag.** An option on a mode command, such as `--action` in
`hints --action left_click`. It means the same in a binding, in a shell and
over IPC. See the [mode flag reference](../reference/cli.md#mode-flag-reference).

**Overlay.** The transparent layer Neru draws hints, grids and indicators on.
Clicks pass through it.

**Override file.** The file `neru config set` writes, next to your config and
named after it, such as `config.override.toml`. Its values win over your
config. See [How the layers combine](../guide/configuring.md#how-the-layers-combine).

**Primary.** The main modifier of each platform: `Cmd` on macOS, `Ctrl` on
Linux and Windows. Write `Primary` in a hotkey to share one config across
platforms.

**Selection.** The point a mode has picked, such as a typed hint or a grid
cell. Clicks and other actions aimed at a point run at the selection while a
mode has one, and at the cursor otherwise. See [Targeting](../reference/cli.md#targeting).

**Sequence.** Several steps run in order as one unit. A binding with an array
value, a macro, a mode's `--on-exit` steps and `neru run` are all sequences.
See [Sequences](bindings.md#sequences).

**Step.** One unit of work in a binding: a mode command, an action, a macro, a
config command, or `exec` with a shell command. See
[What a step can be](bindings.md#what-a-step-can-be).

**Sticky modifier.** A modifier you tap on its own inside a mode, which then
stays held for the following clicks and scrolls. See
[Using Neru](../guide/using-neru.md#more-than-a-left-click).

**Strategy.** How hints mode finds elements: `axtree` reads the accessibility
tree, `vision` reads the screen with OCR, and `contour` finds shapes in the
screen image. See [`[hints]`](../reference/configuration.md#hints).
