# Using Neru

A tour of every mode, from moving the cursor to clicking, dragging and
scrolling. It takes about ten minutes. Neru should be running with its
permissions granted. See [Getting started](getting-started.md).

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

In hints, the grids and scroll, the arrow keys nudge the cursor a few pixels,
so you can correct a near miss before you click.

## Hints

`Primary+Shift+Space`, or `neru hints`.

![Animated demo of hints mode](https://github.com/user-attachments/assets/4327c027-d3c8-4915-a740-a0c9c46b386d)

1. Press the hotkey. Every clickable element in the focused window gets a
   short label such as `as` or `f`. A disabled element, such as a greyed-out
   button, gets none, since clicking it does nothing.
2. Type a label. The cursor moves to that element, and fresh labels appear so
   you can pick again.
3. Press `Shift+L` to click it.

If you mistype, press `Backspace`. To find an element by its text instead of
its label, press `/` and type. The
[`[hints]` default hotkeys](../reference/configuration.md#hints-default-hotkeys) list
every key hints mode answers.

Hints read the app's accessibility tree, so they suit native apps, browsers
and Electron. If an app shows few or no hints, use a grid mode, or try another
[hint strategy](../reference/configuration.md#hints). `neru roles --explain`
shows which native roles your `clickable_roles` select here, and
[Troubleshooting](troubleshooting.md#hints-and-grids)
covers the common causes. A tool that knows where the controls are can also
[supply the targets](../reference/cli.md#supplying-your-own-targets).

**Click on select.** Most people want hints to click as soon as the label is
typed, like Vimium. Bind `hints --action left_click`, as in
[Recipes](recipes.md#click-as-soon-as-a-hint-is-typed). To hand the picked
point to a script instead, add `--on-exit` steps, which read it from `NERU_X`
and `NERU_Y`. See [Scripting](scripting.md#hand-the-picked-point-to-a-script).

## Recursive grid

`Primary+Shift+C`, or `neru recursive_grid`.

![Animated demo of recursive grid mode](https://github.com/user-attachments/assets/f72b43fb-9745-4c17-a80d-15ca4077a5a2)

1. Press the hotkey. The screen splits into a grid with one key per cell, by
   default 3x3 with `r t y`, `f g h`, `v b n`. Size and keys are under
   [`[recursive_grid]`](../reference/configuration.md#recursive_grid).
2. Press the key for the cell that holds your target. The cursor jumps to its
   center and that cell splits again.
3. Repeat until the cursor is on the target, usually two or three presses.
4. Press `Shift+L` to click.

`Backspace` goes back one level, and `Space` starts over. Recursive grid needs
nothing from the app, so it works in canvases, games and remote desktops.

## Bisect

`Primary+Shift+B`, or `neru bisect`.

![Animated demo of bisect mode](https://github.com/user-attachments/assets/6b18396a-9bb8-4d1b-a8f8-270c19c0f727)

Bisect halves the screen instead of splitting it into cells, so there are no
labels to read.

1. Press the hotkey. Neru splits the screen into four quadrants.
2. Press `h`, `j`, `k` or `l` to keep the left, bottom, top or right half, or
   `y`, `u`, `b` or `n` to keep a quadrant. The cursor moves to the center of
   what you kept. These are the
   [`[bisect]` default hotkeys](../reference/configuration.md#bisect-default-hotkeys).
3. Repeat until the cursor is on the target, then press `Shift+L`.

`Backspace` undoes the last cut, and `Space` starts over.

## Grid

`Primary+Shift+G`, or `neru grid`.

![Animated demo of grid mode](https://github.com/user-attachments/assets/b0759650-a3fa-43f3-9123-237124afe870)

1. Press the hotkey. The screen fills with labeled cells.
2. Type a cell's label. A 3x3 subgrid opens inside it.
3. Type a subgrid key. The cursor moves there.
4. Press `Shift+L` to click.

Grid suits coarse jumps across large monitors. `Space` starts over.

`neru grid --no-subgrid` skips step 3. The label alone selects the cell's
center, so `neru grid --no-subgrid --action move_mouse` saves a key on every
jump. Bind it to a second hotkey to keep the subgrid on the first.

## Scroll

`Primary+Shift+S`, or `neru scroll`.

1. Move the cursor over the pane you want to scroll, with any mode.
2. Press the hotkey.
3. Press `j` and `k` to scroll down and up, then `Escape` when you are done.

The [`[scroll]` default hotkeys](../reference/configuration.md#scroll-default-hotkeys)
list the keys for paging, jumping to the top and scrolling sideways.

## Vim

`neru vim`.

![Animated demo of vim mode](https://github.com/user-attachments/assets/3b8a4ccb-f102-4971-902e-c4097d14afe5)

Vim mode has no default hotkey, so bind one:

```toml
[hotkeys]
"Primary+Shift+V" = "vim"
```

1. Press the hotkey. Neru reads the text in the focused window and puts a
   cursor on the character nearest the mouse pointer. A status bar shows the
   mode and the command being typed.
2. Move with vim's keys, such as `w`, `b`, `j`, `k`, `f`, `/` and counts. To
   jump anywhere, press `s`, type a few characters of the target, then the
   label that appears on it.
3. Act on it. `Return` clicks the character under the cursor, `y` and a
   motion copy the text as read, and `i`, `c` or `d` edit the app's text in
   place.
4. Press `Escape` to leave. With a command half typed, the first `Escape`
   cancels it.

Vim mode keeps every key for vim, so it does not bind the shared keys in
the next section, such as `Shift+L`. `gc`, `gr` and `gm` click, and
`:leftdown` and `:leftup` around a motion drag. On macOS it needs the Screen
Recording permission, see [Getting started](getting-started.md#macos).
[`neru vim`](../reference/cli.md#neru-vim) lists every key and what an edit
does to the app, and [`[vim.keymaps]`](../reference/configuration.md#vim-keymaps)
remaps them.

## More than a left click

These work in every mode except monitor select, which only picks a display,
and vim, which has its own keys for them.

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
the cursor instead of stepping, and `accel_enabled = true` speeds it
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
