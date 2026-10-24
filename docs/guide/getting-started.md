# Getting started

Start Neru, grant its permissions, and bind your first hotkey. Install Neru
first, see [Installation](installation.md).

## Start the daemon

Neru is a background daemon plus the `neru` command. Skip this step if you
installed the login service.

- **macOS:** open `Neru.app`, or run `neru launch`.
- **Windows:** open Neru from the Start Menu, or run `neru launch`.
- **Linux:** run `neru launch`.

Check it with `neru status`.

## Grant permissions

Run `neru doctor`. It lists each permission and backend with its state.
For a bug report, attach the output of `neru doctor --json`.

### macOS

- **Accessibility** is required. Neru asks for it at launch. Grant it in
  **System Settings > Privacy & Security > Accessibility**.
- **Screen Recording** is needed only for the `vision`, `contour` and
  `segment` hint strategies. Neru asks the first time one runs, and macOS applies the grant
  after Neru restarts.

If a grant does not take after an upgrade, see
[Troubleshooting](troubleshooting.md#neru-does-nothing-on-macos).

### Linux

X11 needs only the runtime libraries. Wayland also needs the `input` group and
a udev rule, so follow [Linux setup](linux.md) before going on.

### Windows

No grants are needed. Apps running as administrator are a
[known limit](../reference/platform-support.md#known-gaps).

## Write a config file

```bash
neru config init
```

Neru runs on built-in defaults without a file, but a file is where every
change goes. On first launch, macOS and Windows offer to write it for you.
On Linux, Neru runs on defaults and posts a notification naming
`neru config init`.
[Configuring Neru](configuring.md) covers where it lives and how to apply
edits.

## Bind your first hotkey

### macOS and Windows

Neru ships a launcher for every mode except monitor select, which is disabled
and unbound, and the modes you declare yourself. Hints is
`Primary+Shift+Space`, where `Primary` is `Cmd` on macOS and `Ctrl` on
Windows by default. The [default hotkeys](../reference/configuration.md#global-hotkeys)
list the rest. Go on to [Try it](#try-it).

### Linux

> [!NOTE]
> **Linux ships no global hotkeys**, so none collide with terminal shortcuts
> such as `Ctrl+Shift+C`. Bind your own before you can use Neru.

On Wayland, Neru's own `[hotkeys]` need the `input` group from
[Linux setup](linux.md). Without it, bind `neru hints` and the other modes in
your compositor instead, see
[Global hotkeys on Wayland](linux-desktops.md#global-hotkeys-on-wayland), and
skip the rest of this section. X11 needs nothing more.

The file `neru config init` writes has a commented-out `[hotkeys]` table.
Uncomment it and replace its `Primary+Shift` keys with your own, for example:

```toml
[hotkeys]
"Ctrl+Alt+Space" = "hints"
"Ctrl+Alt+C" = "recursive_grid"
"Ctrl+Alt+S" = "scroll"
```

Keep one `[hotkeys]` table, since a second one makes the file fail to parse.
Then run `neru config reload`. Each value is the command you would type after
`neru` in a shell.

How a `[hotkeys]` table combines with the defaults is in
[Merging behavior](../reference/configuration.md#merging-behavior).

## Try it

Press the hints hotkey. Labels appear on every clickable element. Type one and
the cursor moves there. Press `Shift+L` to click, or `Escape` to leave.

Next, take the ten-minute tour in [Using Neru](using-neru.md).
