# Using Neru

A tour of every mode, from moving the cursor to clicking, dragging and
scrolling. It takes about ten minutes. Neru should be running with its
permissions granted, see [Getting started](getting-started.md).

The hotkeys below are the macOS and Windows
[defaults](../reference/configuration.md#global-hotkeys). On Linux, bind them
first as in [Getting started](getting-started.md#bind-your-first-hotkey), or
run the command shown next to each one, such as `neru hints`.

## Every mode works the same way

1. Press a hotkey. An overlay appears, and Neru reads every key until you leave.
2. Type keys to pick a point. The cursor moves there.
3. Act on it. `Shift+L` left-clicks, `Shift+R` right-clicks, `Shift+M`
   middle-clicks.
4. Press `Escape` to leave the mode.

In hints, the grids and scroll, the arrow keys nudge the cursor 10 px, so you
can correct a near miss before you click.

## Hints

`Primary+Shift+Space`, or `neru hints`.

![Animated demo of hints mode](https://github.com/user-attachments/assets/a19ef869-6400-4b5f-b3c6-92af69c2a76b)

1. Press the hotkey. Every clickable element in the focused window gets a
   short label such as `as` or `f`.
2. Type a label. The cursor moves to that element, and fresh labels appear so
   you can pick again.
3. Press `Shift+L` to click it.

If you mistype, press `Backspace`. To find an element by its text instead of
its label, press `/` and type. The
[`[hints]` default hotkeys](../reference/configuration.md#default-hotkeys) list
every key hints mode answers.

Hints read the app's accessibility tree, so they suit native apps, browsers
and Electron. If an app shows few or no hints, use a grid mode, or try another
[hint strategy](../reference/configuration.md#hints).

**Click on select.** Most people want hints to click as soon as the label is
typed, like Vimium. Bind `hints --action left_click`, as in
[Recipes](recipes.md#click-as-soon-as-a-hint-is-typed).

## Recursive grid

`Primary+Shift+C`, or `neru recursive_grid`.

![Animated demo of recursive grid mode](https://github.com/user-attachments/assets/d82e14fa-0ac4-4081-9a0a-f99243d1da5b)

1. Press the hotkey. The screen splits into a 3x3 grid, one key per cell
   (`r t y`, `f g h`, `v b n` by default).
2. Press the key for the cell that holds your target. The cursor jumps to its
   center and that cell splits again.
3. Repeat until the cursor is on the target, usually two or three presses.
4. Press `Shift+L` to click.

`Backspace` goes back one level, and `Space` starts over. Recursive grid needs
nothing from the app, so it works in canvases, games and remote desktops.

## Bisect

`Primary+Shift+B`, or `neru bisect`.

![Animated demo of bisect mode](https://github.com/user-attachments/assets/913e9d15-7f42-470d-b5cf-7a55065789fe)

Bisect halves the screen instead of splitting it into cells, so there are no
labels to read.

1. Press the hotkey. Neru splits the screen into four quadrants.
2. Press `h`, `j`, `k` or `l` to keep the left, bottom, top or right half, or
   `y`, `u`, `b` or `n` to keep a quadrant. The cursor moves to the center of
   what you kept.
3. Repeat until the cursor is on the target, then press `Shift+L`.

`Backspace` undoes the last cut, and `Space` starts over.

## Grid

`Primary+Shift+G`, or `neru grid`.

![Animated demo of grid mode](https://github.com/user-attachments/assets/392fcfa1-9779-464c-8bac-a05a6afc73c3)

1. Press the hotkey. The screen fills with labeled cells.
2. Type a cell's label. A 3x3 subgrid opens inside it.
3. Type a subgrid key. The cursor moves there.
4. Press `Shift+L` to click.

Grid suits coarse jumps across large monitors. `Space` starts over.

## Scroll

`Primary+Shift+S`, or `neru scroll`.

1. Move the cursor over the pane you want to scroll, with any mode.
2. Press the hotkey.
3. Press `j` and `k` to scroll down and up, then `Escape` when you are done.

The [`[scroll]` default hotkeys](../reference/configuration.md#default-hotkeys-4)
list the keys for paging, jumping to the top and scrolling sideways.

## More than a left click

These work in every mode.

**Double and triple click.** Bind a chain, such as
`"Ctrl+Enter" = "action left_click,left_click"`. See
[Recipes](recipes.md#click-with-return).

**Drag.** Move to the start point, press `Shift+I` to hold the left button,
move to the end point with the same mode or the arrow keys, then press
`Shift+U` to release. Leaving the mode releases any held button. Right and
middle drag are in [Recipes](recipes.md#drag-with-any-mouse-button).

**Sticky modifiers.** Tap `Cmd`, `Ctrl`, `Alt` or `Shift` on its own inside a
mode. It stays held, shown next to the cursor, for every following click and
scroll, so `Cmd` then `Shift+L` is a Cmd-click. Tap it again to release it. See
[`[sticky_modifiers]`](../reference/configuration.md#sticky_modifiers).

**Glide.** With `[held_repeat] enabled = true`, holding an arrow key glides
the cursor smoothly instead of stepping, and `accel_enabled = true` speeds it
up the longer you hold. See
[`[held_repeat]`](../reference/configuration.md#held_repeat).

## Several monitors

Hints and grids draw on the monitor under the cursor. To jump to another
monitor, enable monitor select and bind it:

```toml
[monitor_select]
enabled = true

[hotkeys]
"Primary+Shift+M" = "monitor_select"
```

Each display shows a number. Type one and the cursor moves there.

## Next steps

- [Configuring Neru](configuring.md): where the config lives and how to apply
  changes.
- [Recipes](recipes.md): ready-made bindings, such as click on select and
  restoring the cursor after a click.
- [How bindings work](../concepts/bindings.md): what a binding can run, and
  which one wins inside a mode.
